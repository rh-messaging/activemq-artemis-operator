package services

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewHeadlessServiceForCR2(client client.Client, serviceName string, namespace string, servicePorts *[]corev1.ServicePort, labels map[string]string, svc *corev1.Service) *corev1.Service {

	if svc == nil {
		svc = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       corev1.ServiceSpec{},
		}
	}

	// apply desired state
	svc.ObjectMeta.Labels = labels
	svc.ObjectMeta.Name = serviceName
	svc.ObjectMeta.Namespace = namespace

	svc.Spec.Type = "ClusterIP"
	svc.Spec.Ports = *servicePorts
	svc.Spec.Selector = labels
	svc.Spec.ClusterIP = "None"
	svc.Spec.PublishNotReadyAddresses = true

	return svc
}

func NewServiceDefinitionForCR(svcName types.NamespacedName, client client.Client, nameSuffix string, portNumber int32, selectorLabels map[string]string, labels map[string]string, svc *corev1.Service) *corev1.Service {

	if svc == nil {
		svc = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       corev1.ServiceSpec{},
		}
	}

	// apply desired
	port := corev1.ServicePort{
		Name:       nameSuffix,
		Protocol:   "TCP",
		Port:       portNumber,
		TargetPort: intstr.FromInt(int(portNumber)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	svc.ObjectMeta.Labels = labels
	svc.ObjectMeta.Name = svcName.Name
	svc.ObjectMeta.Namespace = svcName.Namespace

	svc.Spec.Type = "ClusterIP"
	svc.Spec.Ports = ports
	svc.Spec.Selector = selectorLabels
	svc.Spec.SessionAffinity = "None"
	svc.Spec.PublishNotReadyAddresses = true

	return svc
}

func NewPingServiceDefinitionForCR2(client client.Client, serviceName string, namespace string, labels map[string]string, selectorLabels map[string]string, svc *corev1.Service) *corev1.Service {

	if svc == nil {
		svc = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       corev1.ServiceSpec{},
		}
	}

	port := corev1.ServicePort{
		Protocol:   "TCP",
		Port:       8888,
		TargetPort: intstr.FromInt(int(8888)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	// apply desired
	svc.ObjectMeta.Labels = labels
	svc.ObjectMeta.Name = serviceName
	svc.ObjectMeta.Namespace = namespace

	svc.Spec.Type = "ClusterIP"
	svc.Spec.Ports = ports
	svc.Spec.Selector = selectorLabels
	svc.Spec.ClusterIP = "None"
	svc.Spec.PublishNotReadyAddresses = true

	return svc
}
