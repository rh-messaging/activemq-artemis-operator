package resources

import (
	"context"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package k8s_actions")

func Create(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, objectDefinition runtime.Object) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	objectTypeString := reflect.TypeOf(objectDefinition).String()
	reqLogger.Info("Creating new " + objectTypeString)

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, objectDefinition.(v1.Object), scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + objectTypeString)
	}
	reqLogger.Info("Set controller reference for new " + objectTypeString)

	// Call k8s create for service
	if err = client.Create(context.TODO(), objectDefinition); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + objectTypeString)
	}
	reqLogger.Info("Created new " + objectTypeString)

	return err
}

// TODO: Evaluate performance impact of using reflect
func Retrieve(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client, objectDefinition runtime.Object) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	objectTypeString := reflect.TypeOf(objectDefinition).String()
	reqLogger.Info("Retrieving " + objectTypeString)

	var err error = nil
	if err = client.Get(context.TODO(), namespacedName, objectDefinition); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(objectTypeString+" IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info(objectTypeString+" found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return err
}

func Update(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, objectDefinition runtime.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	objectTypeString := reflect.TypeOf(objectDefinition).String()
	reqLogger.Info("Updating " + objectTypeString)

	var err error = nil
	if err = client.Update(context.TODO(), objectDefinition); err != nil {
		reqLogger.Error(err, "Failed to update "+objectTypeString)
	}

	return err
}

func Delete(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, objectDefinition runtime.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	objectTypeString := reflect.TypeOf(objectDefinition).String()
	reqLogger.Info("Deleting " + objectTypeString)

	var err error = nil
	if err = client.Delete(context.TODO(), objectDefinition); err != nil {
		reqLogger.Error(err, "Failed to delete "+objectTypeString)
	}

	return err
}
