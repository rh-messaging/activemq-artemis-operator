package routes

import (
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewRouteDefinitionForCR(existing *routev1.Route, namespacedName types.NamespacedName, labels map[string]string, targetServiceName string, targetPortName string, passthroughTLS bool, domain string, brokerHost string) *routev1.Route {

	var desired *routev1.Route = nil
	if existing == nil {
		desired = &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels:    labels,
				Name:      targetServiceName + "-rte",
				Namespace: namespacedName.Namespace,
			},
			Spec: routev1.RouteSpec{},
		}

	} else {
		desired = &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels:    existing.Labels,
				Name:      existing.Name,
				Namespace: namespacedName.Namespace,
			},
			Spec: existing.Spec,
		}
	}

	if brokerHost != "" {
		desired.Spec.Host = brokerHost
	} else if domain != "" {
		desired.Spec.Host = desired.GetObjectMeta().GetName() + "-" + namespacedName.Namespace + "." + domain
	}

	desired.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString(targetPortName),
	}

	desired.Spec.To = routev1.RouteTargetReference{
		Kind: "Service",
		Name: targetServiceName,
	}

	if passthroughTLS {
		desired.Spec.TLS = &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationPassthrough,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
		}
	} else {
		desired.Spec.TLS = nil
	}

	return desired
}
