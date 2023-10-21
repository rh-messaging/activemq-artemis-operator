package services

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewHeadlessServiceForCR2(client client.Client, serviceName string, namespace string, servicePorts *[]corev1.ServicePort, labels map[string]string) *corev1.Service {

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       corev1.ServiceSpec{},
	}

	// fetch  existing state
	client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: serviceName}, svc)

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

func NewServiceDefinitionForCR(serviceName string, client client.Client, crNameSpacedName types.NamespacedName, nameSuffix string, portNumber int32, selectorLabels map[string]string, labels map[string]string) *corev1.Service {

	port := corev1.ServicePort{
		Name:       nameSuffix,
		Protocol:   "TCP",
		Port:       portNumber,
		TargetPort: intstr.FromInt(int(portNumber)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	svcName := types.NamespacedName{Namespace: crNameSpacedName.Namespace, Name: crNameSpacedName.Name + "-" + nameSuffix + "-svc"}
	if serviceName != "" {
		svcName = types.NamespacedName{Namespace: crNameSpacedName.Namespace, Name: serviceName}
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       corev1.ServiceSpec{},
	}

	// fetch  existing state
	client.Get(context.TODO(), svcName, svc)

	// apply desired
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

func NewPingServiceDefinitionForCR2(client client.Client, serviceName string, namespace string, labels map[string]string, selectorLabels map[string]string) *corev1.Service {

	port := corev1.ServicePort{
		Protocol:   "TCP",
		Port:       8888,
		TargetPort: intstr.FromInt(int(8888)),
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	// fetch any existing svc state
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       corev1.ServiceSpec{},
	}
	client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: serviceName}, svc)

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
