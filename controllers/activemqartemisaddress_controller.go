/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"strconv"

	mgmt "github.com/artemiscloud/activemq-artemis-management"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/channels"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/lsrcrs"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var glog = ctrl.Log.WithName("controller_v1beta1activemqartemisaddress")

type AddressDeployment struct {
	AddressResource brokerv1beta1.ActiveMQArtemisAddress
	//a 0-len array means all statefulsets
	SsTargetNameBuilders []SSInfoData
}

var namespacedNameToAddressName = make(map[types.NamespacedName]AddressDeployment)

// ActiveMQArtemisAddressReconciler reconciles a ActiveMQArtemisAddress object
type ActiveMQArtemisAddressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemisaddresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemisaddresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemisaddresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ActiveMQArtemisAddress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ActiveMQArtemisAddressReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx).WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemisAddress")

	//remove nsoptions package once migration to new sdk ready
	//as we use the built-in api in main.go to watch
	//multiple namespaces
	if !nsoptions.Match(request.Namespace) {
		reqLogger.Info("Request not in watch list, ignore", "request", request)
		return ctrl.Result{}, nil
	}

	addressInstance, lookupSucceeded := namespacedNameToAddressName[request.NamespacedName]
	// Fetch the ActiveMQArtemisAddress instance
	instance := &brokerv1beta1.ActiveMQArtemisAddress{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Delete action
			if lookupSucceeded {
				if addressInstance.AddressResource.Spec.RemoveFromBrokerOnDelete {
					deleteQueue(&addressInstance, request, r.Client, r.Scheme)
				} else {
					glog.Info("Not to delete address", "address", addressInstance)
				}
				delete(namespacedNameToAddressName, request.NamespacedName)
				lsrcrs.DeleteLastSuccessfulReconciledCR(request.NamespacedName, "address", getAddressLabels(&addressInstance.AddressResource), r.Client)
			}
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		glog.Error(err, "Requeue the request for error")
		return ctrl.Result{}, err
	}

	if !lookupSucceeded {
		//check stored cr
		if existingCr := lsrcrs.RetrieveLastSuccessfulReconciledCR(request.NamespacedName, "address", r.Client, getAddressLabels(instance)); existingCr != nil {
			//compare resource version
			if existingCr.Checksum == instance.ResourceVersion {
				glog.V(1).Info("The incoming address CR is identical to stored CR, don't do reconcile")
				return ctrl.Result{}, nil
			}
		}
	}

	addressDeployment := AddressDeployment{
		AddressResource:      *instance,
		SsTargetNameBuilders: createNameBuilders(instance),
	}
	err = createQueue(&addressDeployment, request, r.Client, r.Scheme)
	if nil == err {
		namespacedNameToAddressName[request.NamespacedName] = addressDeployment
		crstr, merr := common.ToJson(instance)
		if merr != nil {
			glog.Error(merr, "failed to marshal cr")
		}
		lsrcrs.StoreLastSuccessfulReconciledCR(instance, instance.Name, instance.Namespace, "address", crstr, "", instance.ResourceVersion, getAddressLabels(instance), r.Client, r.Scheme)
	}

	return ctrl.Result{}, nil
}

func getAddressLabels(cr *brokerv1beta1.ActiveMQArtemisAddress) map[string]string {
	labelBuilder := selectors.LabelerData{}
	labelBuilder.Base(cr.Name).Suffix("addr").Generate()
	return labelBuilder.Labels()
}

type SSInfoData struct {
	NameBuilder namer.NamerData
	Labels      map[string]string
}

func createStatefulSetNameBuilder(crName string) SSInfoData {
	ssNameBuilder := namer.NamerData{}
	ssNameBuilder.Base(crName).Suffix("ss").Generate()
	ssLabelData := selectors.LabelerData{}
	ssLabelData.Base(crName).Suffix("app").Generate()

	return SSInfoData{
		NameBuilder: ssNameBuilder,
		Labels:      ssLabelData.Labels(),
	}
}

func createNameBuilders(instance *brokerv1beta1.ActiveMQArtemisAddress) []SSInfoData {
	var nameBuilders []SSInfoData = nil
	for _, crName := range instance.Spec.ApplyToCrNames {
		if crName != "*" {
			builder := createStatefulSetNameBuilder(crName)
			glog.Info("created a new name builder", "builder", builder, "buldername", builder.NameBuilder.Name())
			nameBuilders = append(nameBuilders, builder)
			glog.Info("added one builder for "+crName, "builders", nameBuilders, "len", len(nameBuilders))
		} else {
			return nil
		}
	}
	glog.Info("Created ss name builder for addr", "instance", instance, "builders", nameBuilders)
	return nameBuilders
}

// SetupWithManager sets up the controller with the Manager.
func (r *ActiveMQArtemisAddressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	go setupAddressObserver(mgr, channels.AddressListeningCh)
	return ctrl.NewControllerManagedBy(mgr).
		For(&brokerv1beta1.ActiveMQArtemisAddress{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// This method deals with creating queues and addresses.
func createQueue(instance *AddressDeployment, request ctrl.Request, client client.Client, scheme *runtime.Scheme) error {

	reqLogger := ctrl.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Creating ActiveMQArtemisAddress")

	var err error = nil
	artemisArray := getPodBrokers(instance, request, client, scheme)
	if nil != artemisArray {
		for _, a := range artemisArray {
			if nil == a {
				reqLogger.Info("Creating ActiveMQArtemisAddress artemisArray had a nil!")
				continue
			}
			createAddressResource(a, &instance.AddressResource)
		}
	}

	return err
}

func createAddressResource(a *mgmt.Artemis, addressRes *brokerv1beta1.ActiveMQArtemisAddress) {
	//Now checking if create queue or address
	var err error
	if addressRes.Spec.QueueName == nil || *addressRes.Spec.QueueName == "" {
		//create address
		_, err = a.CreateAddress(addressRes.Spec.AddressName, *addressRes.Spec.RoutingType)
		if nil != err {
			glog.Error(err, "Creating ActiveMQArtemisAddress error for address", addressRes.Spec.AddressName)
			return
		} else {
			glog.Info("Created ActiveMQArtemisAddress for address " + addressRes.Spec.AddressName)
		}
	} else {
		glog.Info("Queue name is not empty so create queue", "name", *addressRes.Spec.QueueName)
		if addressRes.Spec.QueueConfiguration == nil {
			routingType := "MULTICAST"
			if addressRes.Spec.RoutingType != nil {
				routingType = *addressRes.Spec.RoutingType
			}
			_, err := a.CreateQueue(addressRes.Spec.AddressName, *addressRes.Spec.QueueName, routingType)
			if nil != err {
				glog.Error(err, "Creating ActiveMQArtemisAddress error for "+*addressRes.Spec.QueueName)
				return
			} else {
				glog.Info("Created ActiveMQArtemisAddress for " + *addressRes.Spec.QueueName)
			}
		} else {
			//create queue using queueconfig
			queueCfg, ignoreIfExists, err := GetQueueConfig(addressRes)
			if err != nil {
				glog.Error(err, "Failed to get queue config json string")
				return
			}
			respData, err := a.CreateQueueFromConfig(queueCfg, ignoreIfExists)
			if nil != err {
				if mgmt.GetCreationError(respData) == mgmt.QUEUE_ALREADY_EXISTS {
					glog.Info("The queue already exists, updating", "queue", queueCfg)
					respData, err := a.UpdateQueue(queueCfg)
					if err != nil {
						glog.Error(err, "Failed to update queue", "details", respData)
					}
					return
				}
				glog.Error(err, "Creating ActiveMQArtemisAddress error for "+*addressRes.Spec.QueueName)
				return
			} else {
				glog.Info("Created ActiveMQArtemisAddress for " + *addressRes.Spec.QueueName)
			}
		}
	}
}

// This method deals with deleting queues and addresses.
func deleteQueue(instance *AddressDeployment, request ctrl.Request, client client.Client, scheme *runtime.Scheme) error {

	reqLogger := ctrl.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Deleting ActiveMQArtemisAddress")

	var err error = nil
	artemisArray := getPodBrokers(instance, request, client, scheme)
	if nil != artemisArray {
		for _, a := range artemisArray {
			if instance.AddressResource.Spec.QueueName == nil || *instance.AddressResource.Spec.QueueName == "" {
				//delete address
				_, err := a.DeleteAddress(instance.AddressResource.Spec.AddressName)
				if nil != err {
					reqLogger.Error(err, "Deleting ActiveMQArtemisAddress error for address ", instance.AddressResource.Spec.AddressName)
					break
				}
				reqLogger.Info("Deleted ActiveMQArtemisAddress for address " + instance.AddressResource.Spec.AddressName)
			} else {
				//delete queues
				_, err := a.DeleteQueue(*instance.AddressResource.Spec.QueueName)
				if nil != err {
					reqLogger.Error(err, "Deleting ActiveMQArtemisAddress error for queue "+*instance.AddressResource.Spec.QueueName)
					break
				} else {
					reqLogger.Info("Deleted ActiveMQArtemisAddress for queue " + *instance.AddressResource.Spec.QueueName)
					reqLogger.Info("Checking parent address for bindings " + instance.AddressResource.Spec.AddressName)
					bindingsData, err := a.ListBindingsForAddress(instance.AddressResource.Spec.AddressName)
					if nil == err {
						if "" == bindingsData.Value {
							reqLogger.Info("No bindings found removing " + instance.AddressResource.Spec.AddressName)
							a.DeleteAddress(instance.AddressResource.Spec.AddressName)
						} else {
							reqLogger.Info("Bindings found, not removing " + instance.AddressResource.Spec.AddressName)
						}
					}
				}
			}
		}
	}

	return err
}

func getPodBrokers(instance *AddressDeployment, request ctrl.Request, client client.Client, scheme *runtime.Scheme) []*mgmt.Artemis {

	reqLogger := ctrl.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Getting Pod Brokers", "instance", instance)

	var artemisArray []*mgmt.Artemis = nil
	var jolokiaSecretName string = request.Name + "-jolokia-secret"

	targetCrNamespacedNames := createTargetCrNamespacedNames(request.Namespace, instance.AddressResource.Spec.ApplyToCrNames)

	reqLogger.Info("target Cr names", "result", targetCrNamespacedNames)

	ssInfos := GetDeployedStatefuleSetNames(targetCrNamespacedNames)

	reqLogger.Info("got taget ssNames from broker controller", "ssInfos", ssInfos)

	for n, info := range ssInfos {
		reqLogger.Info("Now retrieve ss", "order", n, "ssName", info.NamespacedName)
		statefulset, err := ss.RetrieveStatefulSet(info.NamespacedName.Name, info.NamespacedName, info.Labels, client)
		if nil != err {
			reqLogger.Error(err, "error retriving ss")
			reqLogger.Info("Statefulset: " + info.NamespacedName.Name + " not found")
		} else {
			reqLogger.Info("Statefulset: " + info.NamespacedName.Name + " found")
			pod := &corev1.Pod{}
			podNamespacedName := types.NamespacedName{
				Name:      statefulset.Name + "-0",
				Namespace: request.Namespace,
			}

			// For each of the replicas
			var i int = 0
			var replicas int = int(*statefulset.Spec.Replicas)
			reqLogger.Info("finding pods in ss", "replicas", replicas)
			for i = 0; i < replicas; i++ {
				s := statefulset.Name + "-" + strconv.Itoa(i)
				podNamespacedName.Name = s
				reqLogger.Info("Trying finding pod " + s)
				if err = client.Get(context.TODO(), podNamespacedName, pod); err != nil {
					if errors.IsNotFound(err) {
						reqLogger.Error(err, "Pod IsNotFound", "Namespace", request.Namespace, "Name", request.Name)
					} else {
						reqLogger.Error(err, "Pod lookup error", "Namespace", request.Namespace, "Name", request.Name)
					}
				} else {
					reqLogger.Info("Pod found", "Namespace", request.Namespace, "Name", request.Name)
					containers := pod.Spec.Containers //get env from this

					jolokiaUser, jolokiaPassword, jolokiaProtocol := resolveJolokiaRequestParams(request.Namespace, &instance.AddressResource, client, scheme, jolokiaSecretName, &containers, podNamespacedName, statefulset, info.Labels)

					reqLogger.Info("New Jolokia with ", "User: ", jolokiaUser, "Protocol: ", jolokiaProtocol)
					artemis := mgmt.GetArtemis(pod.Status.PodIP, "8161", "amq-broker", jolokiaUser, jolokiaPassword, jolokiaProtocol)
					artemisArray = append(artemisArray, artemis)
				}
			}
		}
	}

	reqLogger.Info("Finally we gathered some mgmt arry", "size", len(artemisArray))
	return artemisArray
}

func resolveJolokiaRequestParams(namespace string,
	addressRes *brokerv1beta1.ActiveMQArtemisAddress,
	client client.Client,
	scheme *runtime.Scheme,
	jolokiaSecretName string,
	containers *[]corev1.Container,
	podNamespacedName types.NamespacedName,
	statefulset *appsv1.StatefulSet,
	labels map[string]string) (string, string, string) {

	var jolokiaUser string
	var jolokiaPassword string
	var jolokiaProtocol string

	userDefined := false
	if addressRes.Spec.User != nil {
		userDefined = true
		jolokiaUser = *addressRes.Spec.User
	} else {
		jolokiaUserFromSecret := secrets.GetValueFromSecret(namespace, false, false, jolokiaSecretName, "jolokiaUser", labels, client, scheme, addressRes)
		if jolokiaUserFromSecret != nil {
			userDefined = true
			jolokiaUser = *jolokiaUserFromSecret
		}
	}
	if userDefined {
		if addressRes.Spec.Password != nil {
			jolokiaPassword = *addressRes.Spec.Password
		} else {
			jolokiaPasswordFromSecret := secrets.GetValueFromSecret(namespace, false, false, jolokiaSecretName, "jolokiaPassword", labels, client, scheme, addressRes)
			if jolokiaPasswordFromSecret != nil {
				jolokiaPassword = *jolokiaPasswordFromSecret
			}
		}
	}
	if len(*containers) == 1 {
		envVars := (*containers)[0].Env
		for _, oneVar := range envVars {
			if !userDefined && "AMQ_USER" == oneVar.Name {
				jolokiaUser = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if !userDefined && "AMQ_PASSWORD" == oneVar.Name {
				jolokiaPassword = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if "AMQ_CONSOLE_ARGS" == oneVar.Name {
				jolokiaProtocol = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if jolokiaUser != "" && jolokiaPassword != "" && jolokiaProtocol != "" {
				break
			}
		}
	}

	if jolokiaProtocol == "" {
		jolokiaProtocol = "http"
	} else {
		jolokiaProtocol = "https"
	}

	return jolokiaUser, jolokiaPassword, jolokiaProtocol
}

func getEnvVarValue(envVar *corev1.EnvVar, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client client.Client, labels map[string]string) string {
	var result string
	if envVar.Value == "" {
		result = getEnvVarValueFromSecret(envVar.Name, envVar.ValueFrom, namespace, statefulset, client, labels)
	} else {
		result = envVar.Value
	}
	return result
}

func getEnvVarValueFromSecret(envName string, varSource *corev1.EnvVarSource, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client client.Client, labels map[string]string) string {

	reqLogger := ctrl.Log.WithValues("Namespace", namespace.Name, "StatefulSet", statefulset.Name)

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
	theSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, labels)
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

func createTargetCrNamespacedNames(namespace string, targetCrNames []string) []types.NamespacedName {
	var result []types.NamespacedName = nil
	for _, crName := range targetCrNames {
		result = append(result, types.NamespacedName{
			Namespace: namespace,
			Name:      crName,
		})
		if crName == "" || crName == "*" {
			glog.Info("Found empty or * in target crName, return nil for all")
			return nil
		}
	}
	return result
}

func GetStatefulSetNameForPod(pod *types.NamespacedName) (string, int, map[string]string) {
	glog.Info("Trying to find SS name for pod", "pod name", pod.Name, "pod ns", pod.Namespace)
	for crName, addressDeployment := range namespacedNameToAddressName {
		glog.Info("checking address cr in stock", "cr", crName)
		if crName.Namespace != pod.Namespace {
			glog.Info("this cr doesn't match pod's namespace", "cr's ns", crName.Namespace)
			return "", -1, nil
		}
		if len(addressDeployment.SsTargetNameBuilders) == 0 {
			glog.Info("this cr doesn't have target specified, it will be applied to all")
			//deploy to all sts, need get from broker controller
			ssInfos := GetDeployedStatefuleSetNames(nil)
			if len(ssInfos) == 0 {
				glog.Info("No statefulset found")
				return "", -1, nil
			}
			for _, info := range ssInfos {
				glog.Info("checking if this ss belong", "ss", info.NamespacedName.Name)
				if _, ok, podSerial := namer.PodBelongsToStatefulset(pod, &info.NamespacedName); ok {
					glog.Info("got a match", "ss", info.NamespacedName.Name, "podSerial", podSerial)
					return info.NamespacedName.Name, podSerial, info.Labels
				}
			}
			glog.Info("no match at all")
			return "", -1, nil
		}
		//iterate and check the ss name
		glog.Info("Now processing cr with applyToCrNames")
		for _, ssNameBuilder := range addressDeployment.SsTargetNameBuilders {
			ssName := ssNameBuilder.NameBuilder.Name()
			glog.Info("checking one applyTo", "ss", ssName)
			//at this point the ss name space is sure the same
			ssNameSpace := types.NamespacedName{
				Name:      ssName,
				Namespace: pod.Namespace,
			}
			if _, ok, podSerial := namer.PodBelongsToStatefulset(pod, &ssNameSpace); ok {
				glog.Info("yes this ssName match, returning results", "ssName", ssName, "podSerial", podSerial)
				return ssName, podSerial, ssNameBuilder.Labels
			}
		}
	}
	glog.Info("all through, but none")
	return "", -1, nil
}

func setupAddressObserver(mgr manager.Manager, c chan types.NamespacedName) {
	glog.Info("Setting up address observer")
	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Error(err, "Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Error(err, "Error building kubernetes clientset: %s", err.Error())
	}

	observer := NewAddressObserver(kubeClient, mgr.GetClient(), mgr.GetScheme())

	if err = observer.Run(channels.AddressListeningCh); err != nil {
		glog.Error(err, "Error running controller: %s", err.Error())
	}

	glog.Info("Finish setup address observer")
	return
}
