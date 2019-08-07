package routes

import (
	"context"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
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

func Create(cr *v2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, route *routev1.Route) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new route")

	var err error = nil

	// Set ActiveMQArtemis instance as the owner and controller
	reqLogger.Info("Set controller reference for new route")
	if err = controllerutil.SetControllerReference(cr, route, scheme); err != nil {
		reqLogger.Info("Failed to set controller reference for new route")
	}

	// Call k8s create for route
	if err = client.Create(context.TODO(), route); err != nil {
		reqLogger.Error(err, "Failed to create new route")
	}

	reqLogger.Info("End of Route Creation")

	return err
}

func Retrieve(cr *v2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client, route *routev1.Route) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving the Route ")

	var err error = nil

	// Check if the headless route already exists
	if err = client.Get(context.TODO(), namespacedName, route); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Route Not Found", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Route found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return err
}

func Update(cr *v2alpha1.ActiveMQArtemis, client client.Client, route *routev1.Route) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Updating route")

	var err error = nil
	if err = client.Update(context.TODO(), route); err != nil {
		reqLogger.Error(err, "Failed to update route")
	}

	return err
}

func Delete(cr *v2alpha1.ActiveMQArtemis, client client.Client, route *routev1.Route) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Deleting route")

	var err error = nil
	if err = client.Delete(context.TODO(), route); err != nil {
		reqLogger.Error(err, "Failed to delete route")
	}

	return err
}
