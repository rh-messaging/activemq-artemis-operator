package activemqartemisscaledown

import (
	"context"

	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"time"

	"github.com/rh-messaging/activemq-artemis-operator/pkg/draincontroller"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/signals"
	"io/ioutil"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var log = logf.Log.WithName("controller_activemqartemisscaledown")

var (
	masterURL  string
	kubeconfig string
	namespace  string
	localOnly  bool
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ActiveMQArtemisScaledown Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileActiveMQArtemisScaledown{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("activemqartemisscaledown-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ActiveMQArtemisScaledown
	err = c.Watch(&source.Kind{Type: &brokerv2alpha1.ActiveMQArtemisScaledown{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ActiveMQArtemisScaledown
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &brokerv2alpha1.ActiveMQArtemisScaledown{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileActiveMQArtemisScaledown{}

// ReconcileActiveMQArtemisScaledown reconciles a ActiveMQArtemisScaledown object
type ReconcileActiveMQArtemisScaledown struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ActiveMQArtemisScaledown object and makes changes based on the state read
// and what is in the ActiveMQArtemisScaledown.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileActiveMQArtemisScaledown) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemisScaledown")

	// Fetch the ActiveMQArtemisScaledown instance
	instance := &brokerv2alpha1.ActiveMQArtemisScaledown{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	//the drain controller code
	masterURL = instance.Spec.MasterURL
	kubeconfig = instance.Spec.Kubeconfig
	namespace = instance.Spec.Namespace
	localOnly = instance.Spec.LocalOnly

	reqLogger.Info("====", "masterUrl:", masterURL)
	reqLogger.Info("====", "kubeconfig:", kubeconfig)
	reqLogger.Info("====", "namespace:", namespace)
	reqLogger.Info("====", "localOnly:", localOnly)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		reqLogger.Error(err, "Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		reqLogger.Error(err, "Error building kubernetes clientset: %s", err.Error())
	}

	var kubeInformerFactory kubeinformers.SharedInformerFactory
	if localOnly {
		reqLogger.Info("==== in localOnly mode")
		if namespace == "" {
			bytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err != nil {
				reqLogger.Error(err, "Using --localOnly without --namespace, but unable to determine namespace: %s", err.Error())
			}
			namespace = string(bytes)
			reqLogger.Info("==== reading ns from file", "namespace", namespace)
		}
		reqLogger.Info("==== creating namespace wide factory")
		reqLogger.Info("Configured to only operate on StatefulSets in namespace " + namespace)
		kubeInformerFactory = kubeinformers.NewFilteredSharedInformerFactory(kubeClient, time.Second*30, namespace, nil)
	} else {
		reqLogger.Info("Configured to operate on StatefulSets across all namespaces")
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	}

	reqLogger.Info("==== new drain controller...")
	drainControllerInstance := draincontroller.NewController(kubeClient, kubeInformerFactory, namespace, localOnly)

	reqLogger.Info("==== Starting async factory...")
	go kubeInformerFactory.Start(stopCh)

	reqLogger.Info("==== Running drain controller...")
	if err = drainControllerInstance.Run(1, stopCh); err != nil {
		reqLogger.Info("===== failed to run drainer", "error", err.Error())
		reqLogger.Error(err, "Error running controller: %s", err.Error())
		return reconcile.Result{}, err
	}

	reqLogger.Info("==== OK, return result")
	return reconcile.Result{}, nil
}
