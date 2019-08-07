package serviceports

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"
)

func GetDefaultPorts(cr *brokerv2alpha1.ActiveMQArtemis) *[]corev1.ServicePort {

	ports := []corev1.ServicePort{}
	basicPorts := setBasicPorts()
	ports = append(ports, basicPorts...)

	// if len(cr.Spec.SSLConfig.SecretName) != 0 && len(cr.Spec.SSLConfig.KeyStorePassword) != 0 && len(cr.Spec.SSLConfig.KeystoreFilename) != 0 && len(cr.Spec.SSLConfig.TrustStorePassword) != 0 && len(cr.Spec.SSLConfig.TrustStoreFilename) != 0 {
	if false { // "TODO-FIX-REPLACE"
		sslPorts := setSSLPorts()
		ports = append(ports, sslPorts...)

	}
	return &ports

}

func setSSLPorts() []corev1.ServicePort {

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

func setBasicPorts() []corev1.ServicePort {

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
