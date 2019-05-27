package activemqartemis

import (
	brokerv1alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	"k8s.io/apimachinery/pkg/types"
)

// Names of states
const (
	CreatingK8sResources   = "creating_k8s_resources"
	ConfiguringEnvironment = "configuring_broker_environment"
	CreatingContainer      = "creating_container"
	ContainerRunning       = "running"
)

// IDs of states
const (
	InvalidState           = -1
	CreatingK8sResourcesID = 0
	//ConfiguringEnvironment = "configuring_broker_environment"
	//CreatingContainer      = "creating_container"
	ContainerRunningID          = 1
	NumActiveMQArtemisFSMStates = 2
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

	//Complete = CreatedHeadlessService | CreatedConsoleJolokiaService | CreatedMuxProtocolService
	Complete = CreatedHeadlessService |
		//CreatedPersistentVolumeClaim |
		CreatedConsoleJolokiaService |
		CreatedMuxProtocolService |
		CreatedStatefulSet |
		CreatedPingService |
		CreatedRouteOrIngress
)

// Machine id
const (
	ActiveMQArtemisFSMID = 0
)

type ActiveMQArtemisFSM struct {
	m              fsm.IMachine
	namespacedName types.NamespacedName
	customResource *brokerv1alpha1.ActiveMQArtemis
	r              *ReconcileActiveMQArtemis
}

// Need to deep-copy the instance?
func MakeActiveMQArtemisFSM(instance *brokerv1alpha1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) ActiveMQArtemisFSM {

	var creatingK8sResourceIState fsm.IState
	var containerRunningIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.NewMachine(),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.r = r

	// TODO: Fix disconnect here between passing the parent and being added later as adding implies parenthood
	creatingK8sResourceState := MakeCreatingK8sResourcesState(&amqbfsm, _namespacedName)
	creatingK8sResourceIState = &creatingK8sResourceState
	amqbfsm.Add(&creatingK8sResourceIState)

	containerRunningState := MakeContainerRunningState(&amqbfsm, _namespacedName)
	containerRunningIState = &containerRunningState
	amqbfsm.Add(&containerRunningIState)

	return amqbfsm
}

func NewActiveMQArtemisFSM(instance *brokerv1alpha1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) *ActiveMQArtemisFSM {
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

	// For the moment sequentially set stuff up
	// k8s resource creation and broker environment configuration can probably be done concurrently later

	// Enter == Setup
	err := amqbfsm.m.Enter(CreatingK8sResourcesID)
	return err
}

func (amqbfsm *ActiveMQArtemisFSM) Update() (error, int) {

	// Was the current state complete?

	// Update == Reconcile
	err, nextStateID := amqbfsm.m.Update()
	return err, nextStateID
}

func (amqbfsm *ActiveMQArtemisFSM) Exit() error {

	// Exit == Teardown
	err := amqbfsm.m.Exit()
	return err
}
