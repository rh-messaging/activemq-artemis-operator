package controllers

import (
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var fsmLog = ctrl.Log.WithName("activemqartemis_fsm")

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
	LabelBuilder                  selectors.LabelerData
	GLOBAL_DATA_PATH              string
}

type ActiveMQArtemisFSM struct {
	m                  fsm.IMachine
	namespacedName     types.NamespacedName
	customResource     *brokerv1beta1.ActiveMQArtemis
	prevCustomResource *brokerv1beta1.ActiveMQArtemis
	r                  *ActiveMQArtemisReconciler
	namers             *Namers
	podInvalid         bool
}

//used for persistence of fsm
type ActiveMQArtemisFSMData struct {
	MCurrentStateID                        int   `json:"mcurrentstateid,omitempty"`
	MNextStateID                           int   `json:"mnextstateid,omitempty"`
	MPreviousStateID                       int   `json:"mpreviousstateid,omitempty"`
	MNumStates                             int   `json:"mnumstates,omitempty"`
	MActive                                bool  `json:"mactive,omitempty"`
	MIDCurrentState                        int   `json:"midcurrentstate"`
	StateCreateK8sStepsComplete            uint8 `json:"statecreatek8sstepscomplete,omitempty"`
	StateScalingEnteringObservedGeneration int64 `json:"statescalingenteringobservedgeneration,omitempty"`
}

func MakeActiveMQArtemisFSMFromData(fsmData *ActiveMQArtemisFSMData, instance *brokerv1beta1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ActiveMQArtemisReconciler) *ActiveMQArtemisFSM {

	var creatingK8sResourceIState fsm.IState
	var containerRunningIState fsm.IState
	var scalingIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.CreateMachine(fsmData.MCurrentStateID,
			fsmData.MNextStateID,
			fsmData.MPreviousStateID,
			fsmData.MNumStates,
			fsmData.MActive),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.prevCustomResource = &brokerv1beta1.ActiveMQArtemis{}
	amqbfsm.r = r
	amqbfsm.namers = amqbfsm.MakeNamers()
	amqbfsm.podInvalid = false

	creatingK8sResourceState := CreatingK8sResourcesState{
		s:              fsm.MakeState(CreatingK8sResources, CreatingK8sResourcesID),
		namespacedName: _namespacedName,
		parentFSM:      &amqbfsm,
		stepsComplete:  fsmData.StateCreateK8sStepsComplete,
	}
	creatingK8sResourceIState = &creatingK8sResourceState
	amqbfsm.Add(&creatingK8sResourceIState)

	containerRunningState := MakeContainerRunningState(&amqbfsm, _namespacedName)
	containerRunningIState = &containerRunningState
	amqbfsm.Add(&containerRunningIState)

	scalingState := ScalingState{
		s:                          fsm.MakeState(Scaling, ScalingID),
		namespacedName:             _namespacedName,
		parentFSM:                  &amqbfsm,
		enteringObservedGeneration: fsmData.StateScalingEnteringObservedGeneration,
	}
	scalingIState = &scalingState
	amqbfsm.Add(&scalingIState)

	switch fsmData.MCurrentStateID {
	case CreatingK8sResourcesID:
		fsmLog.Info("restoring curent state to be creatk8s")
		amqbfsm.m.SetCurrentState(creatingK8sResourceIState)
	case ContainerRunningID:
		fsmLog.Info("restoring urrent state to running")
		amqbfsm.m.SetCurrentState(containerRunningIState)
	case ScalingID:
		fsmLog.Info("restoring current to scaling")
		amqbfsm.m.SetCurrentState(scalingIState)
	}

	return &amqbfsm
}

func (amqbfsm *ActiveMQArtemisFSM) GetFSMData() *ActiveMQArtemisFSMData {

	var stepsComplete uint8 = 0
	var enteringObservedGeneration int64 = 0

	machine, ok := amqbfsm.m.(*fsm.Machine)
	if !ok {
		fsmLog.Error(nil, "cannot get fsm machine")
		newMachine := fsm.MakeMachine()
		machine = &newMachine
	}

	istate := machine.GetState(CreatingK8sResourcesID)
	creatingK8sState, ok := (*istate).(*CreatingK8sResourcesState)
	if !ok {
		fsmLog.Error(nil, "fsm doesn't have creatingk8sresource state")
	} else {
		stepsComplete = creatingK8sState.stepsComplete
	}
	istate = machine.GetState(ScalingID)
	scalingState, ok := (*istate).(*ScalingState)
	if !ok {
		fsmLog.Error(nil, "fsm doesn't have Scaling state")
	} else {
		enteringObservedGeneration = scalingState.enteringObservedGeneration
	}

	data := ActiveMQArtemisFSMData{
		MCurrentStateID:                        machine.GetCurrentStateID(),
		MNextStateID:                           machine.GetNextStateID(),
		MPreviousStateID:                       machine.GetPreviousStateID(),
		MNumStates:                             machine.GetNumStates(),
		MActive:                                machine.GetActive(),
		MIDCurrentState:                        machine.GetIDCurrentState(),
		StateCreateK8sStepsComplete:            stepsComplete,
		StateScalingEnteringObservedGeneration: enteringObservedGeneration,
	}
	return &data
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
		LabelBuilder:                  selectors.LabelerData{},
		GLOBAL_DATA_PATH:              "/opt/" + amqbfsm.customResource.Name + "/data",
	}
	newNamers.SsNameBuilder.Base(amqbfsm.customResource.Name).Suffix("ss").Generate()
	newNamers.SsGlobalName = amqbfsm.customResource.Name
	newNamers.SvcHeadlessNameBuilder.Prefix(amqbfsm.customResource.Name).Base("hdls").Suffix("svc").Generate()
	newNamers.SvcPingNameBuilder.Prefix(amqbfsm.customResource.Name).Base("ping").Suffix("svc").Generate()
	newNamers.PodsNameBuilder.Base(amqbfsm.customResource.Name).Suffix("container").Generate()
	newNamers.SecretsCredentialsNameBuilder.Prefix(amqbfsm.customResource.Name).Base("credentials").Suffix("secret").Generate()
	newNamers.SecretsConsoleNameBuilder.Prefix(amqbfsm.customResource.Name).Base("console").Suffix("secret").Generate()
	newNamers.SecretsNettyNameBuilder.Prefix(amqbfsm.customResource.Name).Base("netty").Suffix("secret").Generate()
	newNamers.LabelBuilder.Base(amqbfsm.customResource.Name).Suffix("app").Generate()

	return &newNamers
}

// Need to deep-copy the instance?
func MakeActiveMQArtemisFSM(instance *brokerv1beta1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ActiveMQArtemisReconciler) *ActiveMQArtemisFSM {

	var creatingK8sResourceIState fsm.IState
	var containerRunningIState fsm.IState
	var scalingIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.NewMachine(),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.prevCustomResource = &brokerv1beta1.ActiveMQArtemis{}
	amqbfsm.r = r
	amqbfsm.namers = amqbfsm.MakeNamers()
	amqbfsm.podInvalid = false

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

	return &amqbfsm
}

func NewActiveMQArtemisFSM(instance *brokerv1beta1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ActiveMQArtemisReconciler) *ActiveMQArtemisFSM {

	amqbfsm := MakeActiveMQArtemisFSM(instance, _namespacedName, r)

	return amqbfsm
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
		fsmLog.Error(nil, "Panic happened with error!", "details", err)
	}
}

func (amqbfsm *ActiveMQArtemisFSM) Enter(startStateID int) error {

	defer amqbfsm.panicOccurred()

	var err error = nil

	// For the moment sequentially set stuff up
	// k8s resource creation and broker environment configuration can probably be done concurrently later
	amqbfsm.r.Result = ctrl.Result{}
	if err = amqbfsm.m.Enter(CreatingK8sResourcesID); nil != err {
		err, _ = amqbfsm.m.Update()
	}

	return err
}

func (amqbfsm *ActiveMQArtemisFSM) UpdateCustomResource(newRc *brokerv1beta1.ActiveMQArtemis) {
	*amqbfsm.prevCustomResource = *amqbfsm.customResource
	*amqbfsm.customResource = *newRc
}

func (amqbfsm *ActiveMQArtemisFSM) Update() (error, int) {

	defer amqbfsm.panicOccurred()

	// Was the current state complete?
	amqbfsm.r.Result = ctrl.Result{}
	err, nextStateID := amqbfsm.m.Update()
	ssNamespacedName := types.NamespacedName{Name: amqbfsm.namers.SsNameBuilder.Name(), Namespace: amqbfsm.customResource.Namespace}
	UpdatePodStatus(amqbfsm.customResource, amqbfsm.r.Client, ssNamespacedName)

	return err, nextStateID
}

func (amqbfsm *ActiveMQArtemisFSM) Exit() error {

	amqbfsm.r.Result = ctrl.Result{}
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

func (amqbfsm *ActiveMQArtemisFSM) SetPodInvalid(isInvalid bool) {
	amqbfsm.podInvalid = isInvalid
}

func (amqbfsm *ActiveMQArtemisFSM) GetPodInvalid() bool {
	return amqbfsm.podInvalid
}
