package v2alpha3activemqartemis

import (
	"context"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_v2alpha3activemqartemis")

var namespacedNameToFSM = make(map[types.NamespacedName]*ActiveMQArtemisFSM)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ActiveMQArtemis Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileActiveMQArtemis{client: mgr.GetClient(), scheme: mgr.GetScheme(), result: reconcile.Result{Requeue: false}}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("v2alpha3activemqartemis-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ActiveMQArtemis
	err = c.Watch(&source.Kind{Type: &brokerv2alpha3.ActiveMQArtemis{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ActiveMQArtemis
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &brokerv2alpha3.ActiveMQArtemis{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner ActiveMQArtemis
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &brokerv2alpha3.ActiveMQArtemis{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileActiveMQArtemis{}

// ReconcileActiveMQArtemis reconciles a ActiveMQArtemis object
type ReconcileActiveMQArtemis struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	result reconcile.Result
}

// Reconcile reads that state of the cluster for a ActiveMQArtemis object and makes changes based on the state read
// and what is in the ActiveMQArtemis.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileActiveMQArtemis) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemis")

	var err error = nil
	var namespacedNameFSM *ActiveMQArtemisFSM = nil
	var amqbfsm *ActiveMQArtemisFSM = nil

	customResource := &brokerv2alpha3.ActiveMQArtemis{}
	namespacedName := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	// Fetch the ActiveMQArtemis instance
	// When first creating this will have err == nil
	// When deleting after creation this will have err NotFound
	// When deleting before creation reconcile won't be called
	if err = r.client.Get(context.TODO(), request.NamespacedName, customResource); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ActiveMQArtemis Controller Reconcile encountered a IsNotFound, checking to see if we should delete namespacedName tracking for request NamespacedName " + request.NamespacedName.String())

			// See if we have been tracking this NamespacedName
			if namespacedNameFSM = namespacedNameToFSM[namespacedName]; namespacedNameFSM != nil {
				reqLogger.Info("Removing namespacedName tracking for " + namespacedName.String())
				// If so we should no longer track it
				amqbfsm = namespacedNameToFSM[namespacedName]
				amqbfsm.Exit()
				delete(namespacedNameToFSM, namespacedName)
				amqbfsm = nil
			}

			// Setting err to nil to prevent requeue
			err = nil
		} else {
			reqLogger.Error(err, "ActiveMQArtemis Controller Reconcile errored thats not IsNotFound, requeuing request", "Request Namespace", request.Namespace, "Request Name", request.Name)
			// Leaving err as !nil causes requeue
		}

		// Add error detail for use later
		return r.result, err
	}

	// Do lookup to see if we have a fsm for the incoming name in the incoming namespace
	// if not, create it
	// for the given fsm, do an update
	// - update first level sets? what if the operator has gone away and come back? stateless?
	if namespacedNameFSM = namespacedNameToFSM[namespacedName]; namespacedNameFSM == nil {
		// TODO: Fix multiple fsm's post ENTMQBR-2875
		if len(namespacedNameToFSM) > 0 {
			reqLogger.Info("ActiveMQArtemis Controller Reconcile does not yet support more than one custom resource instance per namespace!")
			return r.result, nil
		}
		amqbfsm = NewActiveMQArtemisFSM(customResource, namespacedName, r)
		namespacedNameToFSM[namespacedName] = amqbfsm

		// Enter the first state; atm CreatingK8sResourcesState
		amqbfsm.Enter(CreatingK8sResourcesID)
	} else {
		amqbfsm = namespacedNameFSM
		//remember current customeResource so that we can compare for update
		amqbfsm.UpdateCustomResource(customResource)
		err, _ = amqbfsm.Update()
	}

	// Single exit, return the result and error condition
	return r.result, err
}
