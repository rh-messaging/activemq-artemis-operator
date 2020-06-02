package v2alpha1activemqartemisaddress

import (
	"context"
	mgmt "github.com/artemiscloud/activemq-artemis-management"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/secrets"
	ss "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strconv"
)

var log = logf.Log.WithName("controller_v2alpha1activemqartemisaddress")
var namespacedNameToAddressName = make(map[types.NamespacedName]brokerv2alpha1.ActiveMQArtemisAddress)

//This channel is used to receive new ready pods
var C = make(chan types.NamespacedName)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ActiveMQArtemisAddress Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	go setupAddressObserver(mgr, C)
	return &ReconcileActiveMQArtemisAddress{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func setupAddressObserver(mgr manager.Manager, c chan types.NamespacedName) {
	log.Info("Setting up address observer")
	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Error(err, "Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "Error building kubernetes clientset: %s", err.Error())
	}

	namespace, err := k8sutil.GetWatchNamespace()

	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		return
	}

	observer := NewAddressObserver(kubeClient, namespace, mgr.GetClient())

	if err = observer.Run(C); err != nil {
		log.Error(err, "Error running controller: %s", err.Error())
	}

	log.Info("Finish setup address observer")
	return
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("v2alpha1activemqartemisaddress-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ActiveMQArtemisAddress
	err = c.Watch(&source.Kind{Type: &brokerv2alpha1.ActiveMQArtemisAddress{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ActiveMQArtemisAddress
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &brokerv2alpha1.ActiveMQArtemisAddress{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileActiveMQArtemisAddress{}

// ReconcileActiveMQArtemisAddress reconciles a ActiveMQArtemisAddress object
type ReconcileActiveMQArtemisAddress struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ActiveMQArtemisAddress object and makes changes based on the state read
// and what is in the ActiveMQArtemisAddress.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileActiveMQArtemisAddress) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemisAddress")

	// Fetch the ActiveMQArtemisAddress instance
	instance := &brokerv2alpha1.ActiveMQArtemisAddress{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		// Delete action
		addressInstance, lookupSucceeded := namespacedNameToAddressName[request.NamespacedName]
		if lookupSucceeded {
			err = deleteQueue(&addressInstance, request, r.client)
			delete(namespacedNameToAddressName, request.NamespacedName)
		}
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	} else {
		err = createQueue(instance, request, r.client)
		if nil == err {
			namespacedNameToAddressName[request.NamespacedName] = *instance //.Spec.QueueName
		}
	}

	return reconcile.Result{}, nil
}

func createQueue(instance *brokerv2alpha1.ActiveMQArtemisAddress, request reconcile.Request, client client.Client) error {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Creating ActiveMQArtemisAddress")

	var err error = nil
	artemisArray := getPodBrokers(instance, request, client)
	if nil != artemisArray {
		for _, a := range artemisArray {
			if nil == a {
				reqLogger.Info("Creating ActiveMQArtemisAddress artemisArray had a nil!")
				continue
			}
			_, err := a.CreateQueue(instance.Spec.AddressName, instance.Spec.QueueName, instance.Spec.RoutingType)
			if nil != err {
				reqLogger.Info("Creating ActiveMQArtemisAddress error for " + instance.Spec.QueueName)
				break
			} else {
				reqLogger.Info("Created ActiveMQArtemisAddress for " + instance.Spec.QueueName)
			}
		}
	}

	return err
}

func deleteQueue(instance *brokerv2alpha1.ActiveMQArtemisAddress, request reconcile.Request, client client.Client) error {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Deleting ActiveMQArtemisAddress")

	var err error = nil
	artemisArray := getPodBrokers(instance, request, client)
	if nil != artemisArray {
		for _, a := range artemisArray {
			_, err := a.DeleteQueue(instance.Spec.QueueName)
			if nil != err {
				reqLogger.Info("Deleting ActiveMQArtemisAddress error for " + instance.Spec.QueueName)
				break
			} else {
				reqLogger.Info("Deleted ActiveMQArtemisAddress for " + instance.Spec.QueueName)
				reqLogger.Info("Checking parent address for bindings " + instance.Spec.AddressName)
				bindingsData, err := a.ListBindingsForAddress(instance.Spec.AddressName)
				if nil == err {
					if "" == bindingsData.Value {
						reqLogger.Info("No bindings found removing " + instance.Spec.AddressName)
						a.DeleteAddress(instance.Spec.AddressName)
					} else {
						reqLogger.Info("Bindings found, not removing " + instance.Spec.AddressName)
					}
				}
			}
		}
	}

	return err
}

func getPodBrokers(instance *brokerv2alpha1.ActiveMQArtemisAddress, request reconcile.Request, client client.Client) []*mgmt.Artemis {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Getting Pod Brokers")

	var artemisArray []*mgmt.Artemis = nil
	var err error = nil

	ss.NameBuilder.Name()
	if err != nil {
		reqLogger.Error(err, "Failed to ge the statefulset name")
	}

	// Check to see if the statefulset already exists
	ssNamespacedName := types.NamespacedName{
		Name:      ss.NameBuilder.Name(),
		Namespace: request.Namespace,
	}

	statefulset, err := ss.RetrieveStatefulSet(ss.NameBuilder.Name(), ssNamespacedName, client)
	if nil != err {
		reqLogger.Info("Statefulset: " + ssNamespacedName.Name + " not found")
	} else {
		reqLogger.Info("Statefulset: " + ssNamespacedName.Name + " found")

		pod := &corev1.Pod{}
		podNamespacedName := types.NamespacedName{
			Name:      statefulset.Name + "-0",
			Namespace: request.Namespace,
		}

		// For each of the replicas
		var i int = 0
		var replicas int = int(*statefulset.Spec.Replicas)
		artemisArray = make([]*mgmt.Artemis, 0, replicas)
		for i = 0; i < replicas; i++ {
			s := statefulset.Name + "-" + strconv.Itoa(i)
			podNamespacedName.Name = s
			if err = client.Get(context.TODO(), podNamespacedName, pod); err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Error(err, "Pod IsNotFound", "Namespace", request.Namespace, "Name", request.Name)
				} else {
					reqLogger.Error(err, "Pod lookup error", "Namespace", request.Namespace, "Name", request.Name)
				}
			} else {
				reqLogger.Info("Pod found", "Namespace", request.Namespace, "Name", request.Name)
				containers := pod.Spec.Containers //get env from this
				var jolokiaUser string
				var jolokiaPassword string
				if len(containers) == 1 {
					envVars := containers[0].Env
					for _, oneVar := range envVars {
						if "AMQ_USER" == oneVar.Name {
							jolokiaUser = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client)
						}
						if "AMQ_PASSWORD" == oneVar.Name {
							jolokiaPassword = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client)
						}
						if jolokiaUser != "" && jolokiaPassword != "" {
							break
						}
					}
				}

				reqLogger.Info("New Jolokia with ", "User: ", jolokiaUser, "Password: ", jolokiaPassword)
				artemis := mgmt.NewArtemis(pod.Status.PodIP, "8161", "amq-broker", jolokiaUser, jolokiaPassword)
				artemisArray = append(artemisArray, artemis)
			}
		}
	}

	return artemisArray
}

func getEnvVarValue(envVar *corev1.EnvVar, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client client.Client) string {
	var result string
	if envVar.Value == "" {
		result = getEnvVarValueFromSecret(envVar.Name, envVar.ValueFrom, namespace, statefulset, client)
	} else {
		result = envVar.Value
	}
	return result
}

func getEnvVarValueFromSecret(envName string, varSource *corev1.EnvVarSource, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client client.Client) string {

    reqLogger := log.WithValues("Namespace", namespace.Name, "StatefulSet", statefulset.Name)

	var result string = ""

	secretName := varSource.SecretKeyRef.LocalObjectReference.Name
	secretKey := varSource.SecretKeyRef.Key

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: statefulset.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := map[string]string{
		envName: "",
	}
	theSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap)
	var err error = nil
	if err = resources.Retrieve(namespacedName, client, theSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Secret IsNotFound.", "Secret Name", secretName, "Key", secretKey)
		}
	} else {
		elem, ok := theSecret.Data[envName]
		if ok {
			result = string(elem)
		}
	}
	return result
}
