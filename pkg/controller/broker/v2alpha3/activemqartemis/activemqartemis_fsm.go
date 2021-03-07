package v2alpha3activemqartemis

import (
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Names of states
const (
	CreatingK8sResources   = "creating_k8s_resources"
	ConfiguringEnvironment = "configuring_broker_environment"
	CreatingContainer      = "creating_container"
	ContainerRunning       = "running"
	Scaling                = "scaling"
)

// IDs of states
const (
	NotCreatedID           = -1
	CreatingK8sResourcesID = 0
	//ConfiguringEnvironment = "configuring_broker_environment"
	//CreatingContainer      = "creating_container"
	ContainerRunningID          = 1
	ScalingID                   = 2
	NumActiveMQArtemisFSMStates = 3
)

// Completion of CreatingK8sResources state
const (
	None                   = 0
	CreatedHeadlessService = 1 << 0
	//CreatedPersistentVolumeClaim = 1 << 1
	CreatedStatefulSet           = 1 << 1
	CreatedConsoleJolokiaService = 1 << 2
	CreatedMuxProtocolService    = 1 << 3
	CreatedPingService           = 1 << 4
	CreatedRouteOrIngress        = 1 << 5
	CreatedCredentialsSecret     = 1 << 6
	CreatedNettySecret           = 1 << 7

	Complete = CreatedHeadlessService |
		//CreatedPersistentVolumeClaim |
		CreatedConsoleJolokiaService |
		CreatedMuxProtocolService |
		CreatedStatefulSet |
		CreatedPingService |
		CreatedRouteOrIngress |
		CreatedCredentialsSecret |
		CreatedNettySecret
)

// Machine id
const (
	ActiveMQArtemisFSMID = 0
)

type ActiveMQArtemisFSM struct {
	m                  fsm.IMachine
	namespacedName     types.NamespacedName
	customResource     *brokerv2alpha3.ActiveMQArtemis
	prevCustomResource *brokerv2alpha3.ActiveMQArtemis
	r                  *ReconcileActiveMQArtemis
}

// Need to deep-copy the instance?
func MakeActiveMQArtemisFSM(instance *brokerv2alpha3.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) ActiveMQArtemisFSM {

	var creatingK8sResourceIState fsm.IState
	var containerRunningIState fsm.IState
	var scalingIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.NewMachine(),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.prevCustomResource = &brokerv2alpha3.ActiveMQArtemis{}
	amqbfsm.r = r

	// TODO: Fix disconnect here between passing the parent and being added later as adding implies parenthood
	creatingK8sResourceState := MakeCreatingK8sResourcesState(&amqbfsm, _namespacedName)
	creatingK8sResourceIState = &creatingK8sResourceState
	amqbfsm.Add(&creatingK8sResourceIState)

	containerRunningState := MakeContainerRunningState(&amqbfsm, _namespacedName)
	containerRunningIState = &containerRunningState
	amqbfsm.Add(&containerRunningIState)

	scalingState := MakeScalingState(&amqbfsm, _namespacedName)
	scalingIState = &scalingState
	amqbfsm.Add(&scalingIState)

	return amqbfsm
}

func NewActiveMQArtemisFSM(instance *brokerv2alpha3.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) *ActiveMQArtemisFSM {

	amqbfsm := MakeActiveMQArtemisFSM(instance, _namespacedName, r)

	return &amqbfsm
}
func (amqbfsm *ActiveMQArtemisFSM) Add(s *fsm.IState) {

	amqbfsm.m.Add(s)
}

func (amqbfsm *ActiveMQArtemisFSM) Remove(s *fsm.IState) {

	amqbfsm.m.Remove(s)
}

func ID() int {

	return ActiveMQArtemisFSMID
}

func (amqbfsm *ActiveMQArtemisFSM) Enter(startStateID int) error {

	var err error = nil

	// For the moment sequentially set stuff up
	// k8s resource creation and broker environment configuration can probably be done concurrently later
	amqbfsm.r.result = reconcile.Result{}
	if err = amqbfsm.m.Enter(CreatingK8sResourcesID); nil != err {
		err, _ = amqbfsm.m.Update()
	}

	return err
}

func (amqbfsm *ActiveMQArtemisFSM) UpdateCustomResource(newRc *brokerv2alpha3.ActiveMQArtemis) {
	*amqbfsm.prevCustomResource = *amqbfsm.customResource
	*amqbfsm.customResource = *newRc
}

func (amqbfsm *ActiveMQArtemisFSM) Update() (error, int) {

	// Was the current state complete?
	amqbfsm.r.result = reconcile.Result{}
	err, nextStateID := amqbfsm.m.Update()
	ssNamespacedName := types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: amqbfsm.customResource.Namespace}
	UpdatePodStatus(amqbfsm.customResource, amqbfsm.r.client, ssNamespacedName)

	return err, nextStateID
}

func (amqbfsm *ActiveMQArtemisFSM) Exit() error {

	amqbfsm.r.result = reconcile.Result{}
	err := amqbfsm.m.Exit()

	return err
}
