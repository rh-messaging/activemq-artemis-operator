package v2alpha1activemqartemisscaledown

import (
	"context"

	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"
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

	"io/ioutil"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/draincontroller"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var log = logf.Log.WithName("controller_v2alpha1activemqartemisscaledown")

var (
	masterURL  string
	kubeconfig string
	namespace  string
	localOnly  bool
)

var StopCh chan struct{}

var controllers map[string]*draincontroller.Controller = make(map[string]*draincontroller.Controller)

var kubeClient *kubernetes.Clientset

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
	c, err := controller.New("v2alpha1activemqartemisscaledown-controller", mgr, controller.Options{Reconciler: r})
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

	if !nsoptions.Match(request.Namespace) {
		reqLogger.Info("Request not in watch list, ignore", "request", request)
		return reconcile.Result{}, nil
	}

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
	//masterURL = instance.Spec.MasterURL
	//kubeconfig = instance.Spec.Kubeconfig
	//namespace = instance.Spec.Namespace
	namespace = request.Namespace
	localOnly = instance.Spec.LocalOnly

	//reqLogger.Info("====", "masterUrl:", masterURL)
	//reqLogger.Info("====", "kubeconfig:", kubeconfig)
	reqLogger.Info("====", "namespace:", namespace)
	reqLogger.Info("====", "localOnly:", localOnly)

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		reqLogger.Error(err, "Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		reqLogger.Error(err, "Error building kubernetes clientset: %s", err.Error())
	}

	kubeInformerFactory, drainControllerInstance, isNewController := r.getDrainController(localOnly, namespace, kubeClient, instance)

	if isNewController {
		reqLogger.Info("==== Starting async factory...")
		go kubeInformerFactory.Start(*drainControllerInstance.GetStopCh())

		reqLogger.Info("==== Running drain controller async so multiple controllers can run...")
		go runDrainController(drainControllerInstance)
	}

	reqLogger.Info("==== OK, return result")
	return reconcile.Result{}, nil
}

func (r *ReconcileActiveMQArtemisScaledown) getDrainController(localOnly bool, namespace string, kubeClient *kubernetes.Clientset, instance *brokerv2alpha1.ActiveMQArtemisScaledown) (kubeinformers.SharedInformerFactory, *draincontroller.Controller, bool) {
	var kubeInformerFactory kubeinformers.SharedInformerFactory
	var controllerInstance *draincontroller.Controller
	controllerKey := "*"
	if localOnly {
		if namespace == "" {
			bytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err != nil {
				log.Error(err, "Using --localOnly without --namespace, but unable to determine namespace: %s", err.Error())
			}
			namespace = string(bytes)
			log.Info("==== reading ns from file", "namespace", namespace)
		}
		controllerKey = namespace
	}
	if inst, ok := controllers[controllerKey]; ok {
		log.Info("Drain controller already exists", "namespace", namespace)
		inst.AddInstance(instance)
		return nil, nil, false
	}

	if localOnly {
		// localOnly means there is only one target namespace and it is the same as operator's
		log.Info("==== getting localOnly informer factory", "namespace", controllerKey)
		log.Info("==== creating namespace wide factory")
		log.Info("Configured to only operate on StatefulSets in namespace " + namespace)
		kubeInformerFactory = kubeinformers.NewFilteredSharedInformerFactory(kubeClient, time.Second*30, namespace, nil)
	} else {
		log.Info("==== getting global informer factory")
		log.Info("Creating informer factory to operate on StatefulSets across all namespaces")
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	}

	log.Info("==== new drain controller...")
	controllerInstance = draincontroller.NewController(controllerKey, kubeClient, kubeInformerFactory, namespace, localOnly, r.client)
	controllers[controllerKey] = controllerInstance

	log.Info("Adding scaledown instance to controller", "controller", controllerInstance, "scaledown", instance)
	controllerInstance.AddInstance(instance)

	return kubeInformerFactory, controllerInstance, true
}

func runDrainController(controller *draincontroller.Controller) {
	if err := controller.Run(1); err != nil {
		log.Error(err, "Error running controller: %s", err.Error())
	}
}

func ReleaseController(brokerCRName string) {
}
