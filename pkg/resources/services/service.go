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
func NewPingServiceDefinitionForCR(cr *brokerv2alpha1.ActiveMQArtemis, labels map[string]string, selectorLabels map[string]string) *corev1.Service {

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

func Create(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, serviceToCreate *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + serviceToCreate.Name + " service")

	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, serviceToCreate, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + serviceToCreate.Name + " service")
	}
	reqLogger.Info("Set controller reference for new " + serviceToCreate.Name + " service")

	// Call k8s create for service
	if err = client.Create(context.TODO(), serviceToCreate); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + serviceToCreate.Name + " service")
	}
	reqLogger.Info("Created new " + serviceToCreate.Name + " service")

	return err
}

func Retrieve(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client, serviceToRetrieve *corev1.Service) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + serviceToRetrieve.Name + " service")

	var err error = nil
	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, serviceToRetrieve); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Service " + serviceToRetrieve.Name + " IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Service " + serviceToRetrieve.Name + "found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return err
}
