package services

import (
	"context"
	brokerv1alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package services")

// newServiceForPod returns an activemqartemis service for the pod just created
func newHeadlessServiceForCR(cr *brokerv1alpha1.ActiveMQArtemis, servicePorts *[]corev1.ServicePort) *corev1.Service {

	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      labels,
			Name:        "hs", //"headless" + "-service",
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
func newServiceForCR(cr *brokerv1alpha1.ActiveMQArtemis, name_suffix string, port_number int32) *corev1.Service {

	// Log where we are and what we're doing
	//reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	//reqLogger.Info("Creating new " + name_suffix + " service")

	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

	port := corev1.ServicePort{
		Name:       cr.Name + "-" + name_suffix + "-port",
		Protocol:   "TCP",
		Port:       port_number,
		TargetPort: intstr.FromInt(int(port_number)),
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
			Name:        cr.Name + "-" + name_suffix + "-service",
			Namespace:   cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:            "LoadBalancer",
			Ports:           ports,
			Selector:        labels,
			SessionAffinity: "ClientIP",
		},
	}

	return svc
}

// newServiceForPod returns an activemqartemis service for the pod just created
func newPingServiceForCR(cr *brokerv1alpha1.ActiveMQArtemis) *corev1.Service {

	// Log where we are and what we're doing
	//reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	//reqLogger.Info("Creating new " + name_suffix + " service")

	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

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
			Selector:                 labels,
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
		},
	}

	return svc
}

func GetDefaultPorts(cr *brokerv1alpha1.ActiveMQArtemis) *[]corev1.ServicePort {

	ports := []corev1.ServicePort{}
	basicPorts := SetBasicPorts()
	ports = append(ports, basicPorts...)

	if cr.Spec.SSLEnabled && len(cr.Spec.SSLConfig.SecretName) > 0 {
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
func CreateConsoleJolokiaService(cr *brokerv1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "console-jolokia" + " service")

	// These next two should be considered "hard coded" and temporary
	// Define the console-jolokia Service for this Pod
	consoleJolokiaSvc := newServiceForCR(cr, "console-jolokia", 8161)

	var err error = nil
	// Set ActiveMQArtemis instance as the owner and controller
	if err = controllerutil.SetControllerReference(cr, consoleJolokiaSvc, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "console-jolokia" + " service")
	}
	reqLogger.Info("Set controller reference for new " + "console-jolokia" + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), consoleJolokiaSvc); err != nil {
		// Add error detail for use later
		//rs.stepsComplete |= CreatedConsoleJolokiaService
		reqLogger.Info("Failed to creating new " + "console-jolokia" + " service")
	}
	reqLogger.Info("Created new " + "console-jolokia" + " service")

	return consoleJolokiaSvc, err
}
func RetrieveConsoleJolokiaService(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", instance.Name)
	reqLogger.Info("Retrieving " + "console-jolokia" + " service")

	var err error = nil
	consoleJolokiaSvc := newServiceForCR(instance, "console-jolokia", 8161)

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, consoleJolokiaSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Console Jolokia service IsNotFound", "Namespace", instance.Namespace, "Name", instance.Name)
		} else {
			reqLogger.Error(err, "Console Jolokia service found", "Namespace", instance.Namespace, "Name", instance.Name)
		}
	}

	return consoleJolokiaSvc, err
}

func CreateMuxProtocolService(cr *brokerv1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "mux-protocol" + " service")

	// Define the console-jolokia Service for this Pod
	muxProtocolSvc := newServiceForCR(cr, "mux-protocol", 61616)

	var err error = nil
	// Set ActiveMQArtemis instance as the owner and controller
	if err = controllerutil.SetControllerReference(cr, muxProtocolSvc, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "mux-protocol" + " service")
	}
	reqLogger.Info("Set controller reference for new " + "mux-protocol" + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), muxProtocolSvc); err != nil {
		// Add error detail for use later
		//rs.stepsComplete |= CreatedMuxProtocolService
		reqLogger.Info("Failed to creating new " + "mux-protocol" + " service")
	}
	reqLogger.Info("Created new " + "mux-protocol" + " service")

	return muxProtocolSvc, err
}
func RetrieveMuxProtocolService(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", instance.Name)
	reqLogger.Info("Retrieving " + "mux-protocol" + " service")

	var err error = nil
	muxProtocolSvc := newServiceForCR(instance, "mux-protocol", 61616)

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, muxProtocolSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Mux Protocol service IsNotFound", "Namespace", instance.Namespace, "Name", instance.Name)
		} else {
			reqLogger.Error(err, "Mux Protocol service found", "Namespace", instance.Namespace, "Name", instance.Name)
		}
	}

	return muxProtocolSvc, err
}

func CreatePingService(cr *brokerv1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "ping" + " service")

	pingSvc := newPingServiceForCR(cr)

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
func RetrievePingService(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", instance.Name)
	reqLogger.Info("Retrieving " + "ping" + " service")

	var err error = nil
	pingSvc := newPingServiceForCR(instance)

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, pingSvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Ping service IsNotFound", "Namespace", instance.Namespace, "Name", instance.Name)
		} else {
			reqLogger.Error(err, "Ping service found", "Namespace", instance.Namespace, "Name", instance.Name)
		}
	}

	return pingSvc, err
}

func CreateHeadlessService(cr *brokerv1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "headless" + " service")

	headlessSvc := newHeadlessServiceForCR(cr, GetDefaultPorts(cr))

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

	return headlessSvc, err
}

func DeleteHeadlessService(instance *brokerv1alpha1.ActiveMQArtemis) {
	// kubectl delete cleans up kubernetes resources, just need to clean up local resources if any
}

//r *ReconcileActiveMQArtemis
func RetrieveHeadlessService(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Service, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", instance.Name)
	reqLogger.Info("Retrieving " + "headless" + " service")

	var err error = nil
	headlessService := newHeadlessServiceForCR(instance, GetDefaultPorts(instance)) //&corev1.Service{}

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, headlessService); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Headless service IsNotFound", "Namespace", instance.Namespace, "Name", instance.Name)
		} else {
			reqLogger.Error(err, "Headless service found", "Namespace", instance.Namespace, "Name", instance.Name)
		}
	}

	return headlessService, err
}
