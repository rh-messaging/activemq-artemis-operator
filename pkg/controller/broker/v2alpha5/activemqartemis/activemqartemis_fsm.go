package v2alpha5activemqartemis

import (
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha5"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
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

type Namers struct {
	SsGlobalName                  string
	SsNameBuilder                 namer.NamerData
	SvcHeadlessNameBuilder        namer.NamerData
	SvcPingNameBuilder            namer.NamerData
	PodsNameBuilder               namer.NamerData
	SecretsCredentialsNameBuilder namer.NamerData
	SecretsConsoleNameBuilder     namer.NamerData
	SecretsNettyNameBuilder       namer.NamerData
}

type ActiveMQArtemisFSM struct {
	m                  fsm.IMachine
	namespacedName     types.NamespacedName
	customResource     *brokerv2alpha5.ActiveMQArtemis
	prevCustomResource *brokerv2alpha5.ActiveMQArtemis
	r                  *ReconcileActiveMQArtemis
	namers             *Namers
}

func (amqbfsm *ActiveMQArtemisFSM) MakeNamers() *Namers {
	newNamers := Namers{
		SsGlobalName:                  "",
		SsNameBuilder:                 namer.NamerData{},
		SvcHeadlessNameBuilder:        namer.NamerData{},
		SvcPingNameBuilder:            namer.NamerData{},
		PodsNameBuilder:               namer.NamerData{},
		SecretsCredentialsNameBuilder: namer.NamerData{},
		SecretsConsoleNameBuilder:     namer.NamerData{},
		SecretsNettyNameBuilder:       namer.NamerData{},
	}
	newNamers.SsNameBuilder.Base(amqbfsm.customResource.Name).Suffix("ss").Generate()
	newNamers.SsGlobalName = amqbfsm.customResource.Name
	newNamers.SvcHeadlessNameBuilder.Prefix(amqbfsm.customResource.Name).Base("hdls").Suffix("svc").Generate()
	newNamers.SvcPingNameBuilder.Prefix(amqbfsm.customResource.Name).Base("ping").Suffix("svc").Generate()
	newNamers.PodsNameBuilder.Base(amqbfsm.customResource.Name).Suffix("container").Generate()
	newNamers.SecretsCredentialsNameBuilder.Prefix(amqbfsm.customResource.Name).Base("credentials").Suffix("secret").Generate()
	newNamers.SecretsConsoleNameBuilder.Prefix(amqbfsm.customResource.Name).Base("console").Suffix("secret").Generate()
	newNamers.SecretsNettyNameBuilder.Prefix(amqbfsm.customResource.Name).Base("netty").Suffix("secret").Generate()

	log.Info("mmmm all namers in fsm generated!", "ssbuild", newNamers.SsNameBuilder.Name())
	return &newNamers
}

// Need to deep-copy the instance?
func MakeActiveMQArtemisFSM(instance *brokerv2alpha5.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) ActiveMQArtemisFSM {

	var creatingK8sResourceIState fsm.IState
	var containerRunningIState fsm.IState
	var scalingIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.NewMachine(),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.prevCustomResource = &brokerv2alpha5.ActiveMQArtemis{}
	amqbfsm.r = r
	amqbfsm.namers = amqbfsm.MakeNamers()

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

func NewActiveMQArtemisFSM(instance *brokerv2alpha5.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) *ActiveMQArtemisFSM {

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

func (amqbfsm *ActiveMQArtemisFSM) panicOccurred() {
	if err := recover(); err != nil {
		log.Error(nil, "Panic happened with error!", "details", err)
	}
}

func (amqbfsm *ActiveMQArtemisFSM) Enter(startStateID int) error {

	defer amqbfsm.panicOccurred()

	var err error = nil

	// For the moment sequentially set stuff up
	// k8s resource creation and broker environment configuration can probably be done concurrently later
	amqbfsm.r.result = reconcile.Result{}
	if err = amqbfsm.m.Enter(CreatingK8sResourcesID); nil != err {
		err, _ = amqbfsm.m.Update()
	}

	return err
}

func (amqbfsm *ActiveMQArtemisFSM) UpdateCustomResource(newRc *brokerv2alpha5.ActiveMQArtemis) {
	*amqbfsm.prevCustomResource = *amqbfsm.customResource
	*amqbfsm.customResource = *newRc
}

func (amqbfsm *ActiveMQArtemisFSM) Update() (error, int) {

	defer amqbfsm.panicOccurred()

	// Was the current state complete?
	amqbfsm.r.result = reconcile.Result{}
	err, nextStateID := amqbfsm.m.Update()
	ssNamespacedName := types.NamespacedName{Name: amqbfsm.namers.SsNameBuilder.Name(), Namespace: amqbfsm.customResource.Namespace}
	UpdatePodStatus(amqbfsm.customResource, amqbfsm.r.client, ssNamespacedName)

	return err, nextStateID
}

func (amqbfsm *ActiveMQArtemisFSM) Exit() error {

	amqbfsm.r.result = reconcile.Result{}
	err := amqbfsm.m.Exit()

	return err
}

func (amqbfsm *ActiveMQArtemisFSM) GetStatefulSetNamespacedName() types.NamespacedName {
	var result types.NamespacedName = types.NamespacedName{
		Namespace: amqbfsm.namespacedName.Namespace,
		Name:      amqbfsm.namers.SsNameBuilder.Name(),
	}
	return result
}

func (amqbfsm *ActiveMQArtemisFSM) GetHeadlessServiceName() string {
	return amqbfsm.namers.SvcHeadlessNameBuilder.Name()
}

func (amqbfsm *ActiveMQArtemisFSM) GetPingServiceName() string {
	return amqbfsm.namers.SvcPingNameBuilder.Name()
}

func (amqbfsm *ActiveMQArtemisFSM) GetCredentialsSecretName() string {
	return amqbfsm.namers.SecretsCredentialsNameBuilder.Name()
}

func (amqbfsm *ActiveMQArtemisFSM) GetNettySecretName() string {
	return amqbfsm.namers.SecretsNettyNameBuilder.Name()
}

func (amqbfsm *ActiveMQArtemisFSM) GetConsoleSecretName() string {
	return amqbfsm.namers.SecretsConsoleNameBuilder.Name()
}

func (amqbfsm *ActiveMQArtemisFSM) GetStatefulSetName() string {
	return amqbfsm.namers.SsNameBuilder.Name()
}
