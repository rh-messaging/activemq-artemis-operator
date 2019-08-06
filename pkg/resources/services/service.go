package services

import (
	"context"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package services")

var HeadlessNameBuilder namer.NamerData
//var ServiceNameBuilderArray []namer.NamerData
//var RouteNameBuilderArray []namer.NamerData


// newServiceForPod returns an activemqartemis service for the pod just created
func NewHeadlessServiceForCR(cr *brokerv2alpha1.ActiveMQArtemis, servicePorts *[]corev1.ServicePort) *corev1.Service {

	labels := selectors.LabelBuilder.Labels()

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      labels,
			Name:        HeadlessNameBuilder.Name(),
			Namespace:   cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:                     "ClusterIP",
			Ports:                    *servicePorts,
			Selector:                 labels,
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
		},
	}

	return svc
}

// newServiceForPod returns an activemqartemis service for the pod just created
func NewServiceDefinitionForCR(cr *brokerv2alpha1.ActiveMQArtemis, nameSuffix string, portNumber int32, selectorLabels map[string]string) *corev1.Service {

	port := corev1.ServicePort{
		Name:       nameSuffix,
		Protocol:   "TCP",
		Port:       portNumber,
		TargetPort: intstr.FromInt(int(portNumber)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      selectors.LabelBuilder.Labels(),
			Name:        cr.Name + "-service" + "-" + nameSuffix,
			Namespace:   cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:                     "ClusterIP",
			Ports:                    ports,
			Selector:                 selectorLabels,
			SessionAffinity:          "None",
			PublishNotReadyAddresses: true,
		},
	}

	return svc
}

// newServiceForPod returns an activemqartemis service for the pod just created
func newPingServiceDefinitionForCR(cr *brokerv2alpha1.ActiveMQArtemis, labels map[string]string, selectorLabels map[string]string) *corev1.Service {

	port := corev1.ServicePort{
		Protocol:   "TCP",
		Port:       8888,
		TargetPort: intstr.FromInt(int(8888)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      labels,
			Name:        "ping",
			Namespace:   cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:                     "ClusterIP",
			Ports:                    ports,
			Selector:                 selectorLabels,
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
		},
	}

	return svc
}

func GetDefaultPorts(cr *brokerv2alpha1.ActiveMQArtemis) *[]corev1.ServicePort {

	ports := []corev1.ServicePort{}
	basicPorts := SetBasicPorts()
	ports = append(ports, basicPorts...)

	// if len(cr.Spec.SSLConfig.SecretName) != 0 && len(cr.Spec.SSLConfig.KeyStorePassword) != 0 && len(cr.Spec.SSLConfig.KeystoreFilename) != 0 && len(cr.Spec.SSLConfig.TrustStorePassword) != 0 && len(cr.Spec.SSLConfig.TrustStoreFilename) != 0 {
	if false { // "TODO-FIX-REPLACE"
		sslPorts := SetSSLPorts()
		ports = append(ports, sslPorts...)

	}
	return &ports

}

func SetSSLPorts() []corev1.ServicePort {

	ports := []corev1.ServicePort{

		{
			Name:       "amqp-ssl",
			Protocol:   "TCP",
			Port:       5671,
			TargetPort: intstr.FromInt(int(5671)),
		},
		{
			Name:       "mqtt-ssl",
			Protocol:   "TCP",
			Port:       8883,
			TargetPort: intstr.FromInt(int(8883)),
		},
		{
			Name:       "stomp-ssl",
			Protocol:   "TCP",
			Port:       61612,
			TargetPort: intstr.FromInt(int(61612)),
		},
	}

	return ports
}

func SetBasicPorts() []corev1.ServicePort {

	ports := []corev1.ServicePort{
		{
			Name:       "mqtt",
			Protocol:   "TCP",
			Port:       1883,
			TargetPort: intstr.FromInt(int(1883)),
		},
		{
			Name:       "amqp",
			Protocol:   "TCP",
			Port:       5672,
			TargetPort: intstr.FromInt(int(5672)),
		},
		{
			Name:       "console-jolokia",
			Protocol:   "TCP",
			Port:       8161,
			TargetPort: intstr.FromInt(int(8161)),
		},
		{
			Name:       "stomp",
			Protocol:   "TCP",
			Port:       61613,
			TargetPort: intstr.FromInt(int(61613)),
		},
		{
			Name:       "all",
			Protocol:   "TCP",
			Port:       61616,
			TargetPort: intstr.FromInt(int(61616)),
		},
	}

	return ports
}

func CreateService(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, serviceToCreate *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + serviceToCreate.Name + " service")

	var err error = nil
	// Set ActiveMQArtemis instance as the owner and controller
	if err = controllerutil.SetControllerReference(cr, serviceToCreate, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + serviceToCreate.Name + " service")
	}
	reqLogger.Info("Set controller reference for new " + serviceToCreate.Name + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), serviceToCreate); err != nil {
		// Add error detail for use later
		//rs.stepsComplete |= CreatedConsoleJolokiaService
		reqLogger.Info("Failed to creating new " + serviceToCreate.Name + " service")
	}
	reqLogger.Info("Created new " + serviceToCreate.Name + " service")

	return err
}

func RetrieveService(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, namespacedName types.NamespacedName, serviceToRetrieve *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + serviceToRetrieve.Name + " service")

	var err error = nil
	if err = client.Get(context.TODO(), namespacedName, serviceToRetrieve); err != nil {
		reqLogger.Info("Failed to retrieve " + serviceToRetrieve.Name + " service")
	}
	reqLogger.Info("Retrieved " + serviceToRetrieve.Name + " service")

	return err
}

//func CreateServices(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, baseServiceName string, portNumber int32) error {
//
//	// Log where we are and what we're doing
//	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
//	reqLogger.Info("Creating " + baseServiceName + " services")
//
//	var err error = nil
//	var i int32 = 0
//	ordinalString := ""
//	for ; i < cr.Spec.DeploymentPlan.Size; i++ {
//		labels := selectors.LabelBuilder.Labels()
//		ordinalString = strconv.Itoa(int(i))
//		labels["statefulset.kubernetes.io/pod-name"] = ss.NameBuilder.Name() + "-" + ordinalString
//		consoleJolokiaService := NewServiceDefinitionForCR(cr, baseServiceName+"-"+ordinalString, portNumber, labels)
//		if err = CreateService(cr, client, scheme, consoleJolokiaService); err != nil {
//			reqLogger.Info("Failure to create " + baseServiceName + " service " + ordinalString)
//			break
//		}
//	}
//
//	return err
//}

func RetrieveConsoleJolokiaService(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "console-jolokia" + " service")

	var err error = nil
	consoleJolokiaSvc := NewServiceDefinitionForCR(cr, "console-jolokia", 8161, selectors.LabelBuilder.Labels())

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, consoleJolokiaSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Console Jolokia service IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Console Jolokia service found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return consoleJolokiaSvc, err
}

func RetrieveAllProtocolService(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "all-protocol" + " service")

	var err error = nil
	allProtocolSvc := NewServiceDefinitionForCR(cr, "all-protocol", 61616, selectors.LabelBuilder.Labels())

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, allProtocolSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("All Protocol service IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("All Protocol service found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return allProtocolSvc, err
}

func CreatePingService(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "ping" + " service")

	labels := selectors.LabelBuilder.Labels()
	pingSvc := newPingServiceDefinitionForCR(cr, labels, labels)

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, pingSvc, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "ping" + " service")
	}
	reqLogger.Info("Set controller reference for new " + "ping" + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), pingSvc); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "ping" + " service")

	}
	reqLogger.Info("Created new " + "ping" + " service")

	return pingSvc, err
}
func RetrievePingService(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "ping" + " service")

	var err error = nil
	labels := selectors.LabelBuilder.Labels()
	pingSvc := newPingServiceDefinitionForCR(cr, labels, labels)

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, pingSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Ping service IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Ping service found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return pingSvc, err
}

func Create(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, headlessSvc *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "headless" + " service")

	//headlessSvc := newHeadlessServiceForCR(cr, GetDefaultPorts(cr))

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, headlessSvc, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "headless" + " service")
	}
	reqLogger.Info("Set controller reference for new " + "headless" + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), headlessSvc); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "headless" + " service")

	}
	reqLogger.Info("Created new " + "headless" + " service")

	return err
}

func DeleteHeadlessService(instance *brokerv2alpha1.ActiveMQArtemis) {
	// kubectl delete cleans up kubernetes resources, just need to clean up local resources if any
}

//r *ReconcileActiveMQArtemis
func RetrieveHeadlessService(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "headless" + " service")

	var err error = nil
	headlessService := NewHeadlessServiceForCR(cr, GetDefaultPorts(cr))

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, headlessService); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Headless service IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Headless service found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return headlessService, err
}

func Retrieve(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client, headlessService *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "headless" + " service")

	var err error = nil
	//headlessService := NewHeadlessServiceForCR(cr, GetDefaultPorts(cr))

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, headlessService); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Headless service IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Headless service found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return err
}
