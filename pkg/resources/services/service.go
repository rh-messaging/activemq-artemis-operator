package services

import (
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package services")

var HeadlessNameBuilder namer.NamerData
var PingNameBuilder namer.NamerData

//var ServiceNameBuilderArray []namer.NamerData
//var RouteNameBuilderArray []namer.NamerData

// newServiceForPod returns an activemqartemis service for the pod just created
func NewHeadlessServiceForCR(namespacedName types.NamespacedName, servicePorts *[]corev1.ServicePort) *corev1.Service {

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
			Namespace:   namespacedName.Namespace,
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
//func NewServiceDefinitionForCR(cr *brokerv2alpha1.ActiveMQArtemis, nameSuffix string, portNumber int32, selectorLabels map[string]string) *corev1.Service {
func NewServiceDefinitionForCR(namespacedName types.NamespacedName, nameSuffix string, portNumber int32, selectorLabels map[string]string) *corev1.Service {

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
			Name:        namespacedName.Name + "-" + nameSuffix + "-svc",
			Namespace:   namespacedName.Namespace,
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
func NewPingServiceDefinitionForCR(namespacedName types.NamespacedName, labels map[string]string, selectorLabels map[string]string) *corev1.Service {

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
			Name:        PingNameBuilder.Name(),
			Namespace:   namespacedName.Namespace,
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
