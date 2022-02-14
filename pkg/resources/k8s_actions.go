package resources

import (
	"context"
	"reflect"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var log = ctrl.Log.WithName("package k8s_actions")

func Create(owner v1.Object, namespacedName types.NamespacedName, client client.Client, scheme *runtime.Scheme, clientObject client.Object) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.Info("Creating new " + objectTypeString)

	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(owner, clientObject.(v1.Object), scheme); err != nil {
		// Add error detail for use later
		reqLogger.V(1).Info("Failed to set controller reference for new " + objectTypeString)
	}
	reqLogger.V(1).Info("Set controller reference for new " + objectTypeString)

	if err = client.Create(context.TODO(), clientObject); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to create new " + objectTypeString)
	}
	reqLogger.Info("Created new " + objectTypeString)

	return err
}

func RetrieveWithRetry(namespacedName types.NamespacedName, theClient client.Client, clientObject client.Object, retry bool) error {
	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.Info("Retrieving " + objectTypeString)

	var err error = nil
	if err = theClient.Get(context.TODO(), namespacedName, clientObject); err != nil {
		if errors.IsNotFound(err) {
			if retry {
				reqLogger.Info(objectTypeString+" IsNotFound after retry", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
			} else {
				//retry once using the non-cache client
				reqLogger.V(1).Info("Retry retrieving object using new non-cached client")
				// check to avoid a nil manager that may occur in test
				if common.GetManager() != nil {
					newClient, err := client.New(common.GetManager().GetConfig(), client.Options{})
					if err == nil {
						return RetrieveWithRetry(namespacedName, newClient, clientObject, true)
					}
				}
			}
		} else if runtime.IsNotRegisteredError(err) {
			reqLogger.Info(objectTypeString+" IsNotRegistered", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		} else {
			reqLogger.Info(objectTypeString+" Found", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		}
	}

	return err
}

func Retrieve(namespacedName types.NamespacedName, client client.Client, objectDefinition client.Object) error {
	return RetrieveWithRetry(namespacedName, client, objectDefinition, false)
}

func Update(namespacedName types.NamespacedName, client client.Client, clientObject client.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.V(1).Info("Updating "+objectTypeString, "obj", clientObject)

	var err error = nil
	if err = client.Update(context.TODO(), clientObject); err != nil {
		reqLogger.Error(err, "Failed to update "+objectTypeString)
	}

	return err
}

func UpdateStatus(namespacedName types.NamespacedName, client client.Client, clientObject client.Object) error {
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.V(1).Info("Updating status "+objectTypeString, "obj", clientObject)

	var err error = nil
	if err = client.Status().Update(context.TODO(), clientObject); err != nil {
		reqLogger.Error(err, "Failed to update status on "+objectTypeString)
	}
	return err
}

func Delete(namespacedName types.NamespacedName, client client.Client, clientObject client.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.Info("Deleting " + objectTypeString)

	var err error = nil
	if err = client.Delete(context.TODO(), clientObject); err != nil {
		reqLogger.Error(err, "Failed to delete "+objectTypeString)
	}

	return err
}

func Enable(owner v1.Object, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, clientObject client.Object) (bool, error) {
	causedUpdate, err := configureExposure(owner, client, scheme, namespacedName, clientObject, true)
	return causedUpdate, err
}

func Disable(owner v1.Object, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, clientObject client.Object) (bool, error) {
	causedUpdate, err := configureExposure(owner, client, scheme, namespacedName, clientObject, false)
	return causedUpdate, err
}

func configureExposure(owner v1.Object, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, clientObject client.Object, enable bool) (bool, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name ", namespacedName.Name)
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.Info("Configuring " + objectTypeString)

	var err error = nil
	serviceIsNotFound := false
	causedUpdate := true

	if err = Retrieve(namespacedName, client, clientObject); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(namespacedName.Name + " " + "not found")
			serviceIsNotFound = true
		} else {
			reqLogger.Error(err, "Unexpected error occurred in retrieve", "object def", clientObject)
		}
	}

	// We want a service to be exposed and currently it is not found
	if enable && serviceIsNotFound {
		reqLogger.Info("Creating " + namespacedName.Name)
		if err = Create(owner, namespacedName, client, scheme, clientObject); err != nil {
			causedUpdate = true
		}
	}

	// We do NOT want a service to be exposed and the service IS found
	if !enable && !serviceIsNotFound {
		reqLogger.Info("Deleting " + namespacedName.Name)
		if err = Delete(namespacedName, client, clientObject); err != nil {
			causedUpdate = true
		}
	}

	return causedUpdate, err
}
