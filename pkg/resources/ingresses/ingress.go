package ingresses

import (
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewIngressForCRWithSSL(existing *netv1.Ingress, namespacedName types.NamespacedName, labels map[string]string, targetServiceName string, targetPortName string, sslEnabled bool, domain string, brokerHost string, isOpenshift bool) *netv1.Ingress {

	pathType := netv1.PathTypePrefix

	var desired *netv1.Ingress
	if existing == nil {
		desired = &netv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking.k8s.io/v1",
				Kind:       "Ingress",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels:    labels,
				Name:      targetServiceName + "-ing",
				Namespace: namespacedName.Namespace,
			},
			Spec: netv1.IngressSpec{},
		}
	} else {
		desired = existing
	}

	desired.Spec.Rules = []netv1.IngressRule{
		{
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: targetServiceName,
								},
							},
						},
					},
				},
			},
		},
	}

	portName := ""
	portNumber := -1
	portValue := intstr.FromString(targetPortName)
	if portNumber = portValue.IntValue(); portNumber == 0 {
		portName = portValue.String()
	}

	if portName == "" {
		desired.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port = netv1.ServiceBackendPort{
			Number: int32(portNumber),
		}
	} else {
		desired.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port = netv1.ServiceBackendPort{
			Name: portName,
		}
	}

	host := ""
	if brokerHost != "" {
		host = brokerHost
	} else if domain != "" {
		host = desired.GetObjectMeta().GetName() + "-" + namespacedName.Namespace + "." + domain
	}

	desired.Spec.Rules[0].Host = host
	if sslEnabled {
		if desired.Annotations == nil {
			desired.Annotations = make(map[string]string)
		}

		if isOpenshift {
			pathType = netv1.PathTypeImplementationSpecific
			desired.Annotations["route.openshift.io/termination"] = "passthrough"
			desired.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path = ""
			desired.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType = &pathType
		} else {
			desired.Annotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
			desired.Spec.TLS = []netv1.IngressTLS{{Hosts: []string{host}}}
		}
	}
	return desired
}
