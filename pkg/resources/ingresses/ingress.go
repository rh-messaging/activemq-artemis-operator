package ingresses

import (
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package ingresses")

// Create newIngressForCR method to create exposed ingress
func NewIngressForCR(cr *v2alpha1.ActiveMQArtemis, target string) *extv1b1.Ingress {

	ingress := &extv1b1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    selectors.LabelBuilder.Labels(),
			Name:      cr.Name + "-" + target,
			Namespace: cr.Namespace,
		},
		Spec: extv1b1.IngressSpec{
			Rules: []extv1b1.IngressRule{
				{
					Host: os.Getenv("KUBERNETES_SERVICE_HOST"),
					IngressRuleValue: extv1b1.IngressRuleValue{
						HTTP: &extv1b1.HTTPIngressRuleValue{
							Paths: []extv1b1.HTTPIngressPath{
								extv1b1.HTTPIngressPath{
									Path: "/",
									Backend: extv1b1.IngressBackend{
										ServiceName: svc.HeadlessNameBuilder.Name(),
										ServicePort: intstr.FromString(target),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ingress
}
