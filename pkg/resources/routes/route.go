package routes

import (
	routev1 "github.com/openshift/api/route/v1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package routes")

// Create newRouteForCR method to create exposed route
func NewRouteDefinitionForCR(cr *v2alpha1.ActiveMQArtemis, labels map[string]string, targetServiceName string, targetPortName string, passthroughTLS bool) *routev1.Route {

	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      targetServiceName + "-rte",
			Namespace: cr.Namespace,
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(targetPortName),
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: targetServiceName,
			},
		},
	}

	if passthroughTLS {
		route.Spec.TLS = &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationPassthrough,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
		}
	}

	return route
}
