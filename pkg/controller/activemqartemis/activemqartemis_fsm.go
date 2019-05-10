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

type ActiveMQArtemisFSM struct {
	m              fsm.IMachine
	namespacedName types.NamespacedName
	customResource *brokerv1alpha1.ActiveMQArtemis
	r              *ReconcileActiveMQArtemis
}

// Need to deep-copy the instance?
func MakeActiveMQArtemisFSM(instance *brokerv1alpha1.ActiveMQArtemis, _namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) ActiveMQArtemisFSM {

	var someIState fsm.IState

	amqbfsm := ActiveMQArtemisFSM{
		m: fsm.NewMachine(),
	}

	amqbfsm.namespacedName = _namespacedName
	amqbfsm.customResource = instance
	amqbfsm.r = r

	// TODO: Fix disconnect here between passing the parent and being added later as adding implies parenthood
	someRS := MakeCreatingK8sResourcesState(&amqbfsm, _namespacedName)
	someIState = &someRS
	amqbfsm.Add(&someIState)

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

func (amqbfsm *ActiveMQArtemisFSM) Enter(stateFrom *fsm.IState) {

	// For the moment sequentially set stuff up
	// k8s resource creation and broker environment configuration can probably be done concurrently later

	// Enter == Setup
	amqbfsm.m.Enter(stateFrom)
}

func (amqbfsm *ActiveMQArtemisFSM) Update() {

	// Update == Reconcile
}

func (amqbfsm *ActiveMQArtemisFSM) Exit(stateFrom *fsm.IState) {

	// Exit == Teardown
	amqbfsm.m.Exit(stateFrom)
}

//func (amqbfsm *ActiveMQArtemisFSM) Update(request reconcile.Request, r *ReconcileActiveMQArtemis) (reconcile.Result, error) {
//
//	// Log where we are and what we're doing
//	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
//	reqLogger.Info("Executing ActiveMQArtemisFSM Update")
//
//	// Set it up
//	var err error = nil
//	var reconcileResult reconcile.Result
//	instance := &brokerv1alpha1.ActiveMQArtemis{}
//	found := &corev1.Pod{}
//
//	// Do what's needed
//	for {
//		// Fetch the ActiveMQArtemis instance
//		if err = r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
//			// Add error detail for use later
//			break
//		}
//
//		// Check to see if the k8s resources exist, if not that's our state
//		// Lets assume we are installing for the first time for the moment...
//
//
//		// Check if this Pod already exists
//		if err = r.client.Get(context.TODO(), amqbfsm.namespacedName, found); err == nil {
//			// Don't do anything as the pod exists
//			break
//		}
//
//		break
//	}
//
//	// Handle error, if any
//	if err != nil {
//		if errors.IsNotFound(err) {
//			reconcileResult = reconcile.Result{}
//			reqLogger.Error(err, "ActiveMQArtemis Controller Reconcile encountered a IsNotFound, preventing request requeue", "Pod.Namespace", request.Namespace, "Pod.Name", request.Name)
//			// Setting err to nil to prevent requeue
//			err = nil
//		} else {
//			//log.Error(err, "ActiveMQArtemis Controller Reconcile errored")
//			reqLogger.Error(err, "ActiveMQArtemis Controller Reconcile errored, requeuing request", "Pod.Namespace", request.Namespace, "Pod.Name", request.Name)
//		}
//	}
//
//	// Single exit, return the result and error condition
//	return reconcileResult, err
//
//}
