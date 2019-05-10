package routes

import (
	"context"

	selectors "github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"

	routev1 "github.com/openshift/api/route/v1"
	v1alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package routes")

// Create newRouteForCR method to create exposed route
func NewRouteForCR(cr *v1alpha1.ActiveMQArtemis, target string) *routev1.Route {
	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      cr.Name + "-" + target,
			Namespace: cr.Namespace,
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(target),
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "hs",
			},
		},
	}
	if len(cr.Spec.SSLConfig.SecretName) != 0 && len(cr.Spec.SSLConfig.KeyStorePassword) != 0 && len(cr.Spec.SSLConfig.KeystoreFilename) != 0 && len(cr.Spec.SSLConfig.TrustStorePassword) != 0 && len(cr.Spec.SSLConfig.TrustStoreFilename) != 0 {

		route.Spec.TLS = &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationPassthrough,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
		}

	}
	return route
}

func CreateNewRoute(cr *v1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*routev1.Route, error) {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new route")

	// Define the console-jolokia route for this Pod
	route := NewRouteForCR(cr, "console-jolokia")

	var err error = nil
	// Set ActiveMQArtemis instance as the owner and controller
	reqLogger.Info("Set controller reference for new  route")
	if err = controllerutil.SetControllerReference(cr, route, scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference for new route")
	}

	// Call k8s create for route
	if err = client.Create(context.TODO(), route); err != nil {
		reqLogger.Error(err, "Failed to creating new route")
	}
	reqLogger.Info("End of Route Creation")

	return route, err
}

func RetrieveRoute(cr *v1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*routev1.Route, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving the Route ")

	var err error = nil
	route := NewRouteForCR(cr, "console-jolokia")

	// Check if the headless route already exists
	if err = client.Get(context.TODO(), namespacedName, route); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Route Not Found", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Error(err, "Route found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return route, err
}
