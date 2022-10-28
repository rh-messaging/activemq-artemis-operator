package resources

import (
	"context"
	"net/http"
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

func Create(owner v1.Object, client client.Client, scheme *runtime.Scheme, clientObject client.Object) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", clientObject.GetName(), "Namespace", clientObject.GetNamespace())
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
		reqLogger.Error(err, "Failed to create new "+objectTypeString)
	} else {
		reqLogger.Info("Created new " + objectTypeString)
	}

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
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	objectTypeString := reflect.TypeOf(objectDefinition.(runtime.Object)).String()
	reqLogger.Info("Retrieving " + objectTypeString)

	return client.Get(context.TODO(), namespacedName, objectDefinition)
}

func Update(client client.Client, clientObject client.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", clientObject.GetName(), "Namespace", clientObject.GetNamespace())
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.V(1).Info("Updating "+objectTypeString, "obj", clientObject)

	var err error = nil
	if err = client.Update(context.TODO(), clientObject); err != nil {
		switch checkForForbidden := err.(type) {
		case *errors.StatusError:
			if checkForForbidden.ErrStatus.Status == v1.StatusFailure &&
				checkForForbidden.ErrStatus.Code == http.StatusUnprocessableEntity &&
				checkForForbidden.ErrStatus.Reason == v1.StatusReasonInvalid {

				// "StatefulSet.apps is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'template', 'updateStrategy' and 'minReadySeconds' are forbidden"}
				reqLogger.V(1).Info("Deleting on failed updating "+objectTypeString, "obj", clientObject, "Forbidden", err)
				err = Delete(client, clientObject)
				break
			}
		default:
			reqLogger.Error(err, "Failed to update "+objectTypeString)
		}
	}

	return err
}

func UpdateStatus(client client.Client, clientObject client.Object) error {
	reqLogger := log.WithValues("ActiveMQArtemis Name", clientObject.GetName(), "Namespace", clientObject.GetNamespace())
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.V(1).Info("Updating status "+objectTypeString, "obj", clientObject)

	var err error = nil
	if err = client.Status().Update(context.TODO(), clientObject); err != nil {
		reqLogger.Error(err, "Failed to update status on "+objectTypeString)
	}
	return err
}

func Delete(client client.Client, clientObject client.Object) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", clientObject.GetName(), "Namespace", clientObject.GetNamespace())
	objectTypeString := reflect.TypeOf(clientObject.(runtime.Object)).String()
	reqLogger.Info("Deleting " + objectTypeString)

	var err error = nil
	if err = client.Delete(context.TODO(), clientObject); err != nil {
		reqLogger.Error(err, "Failed to delete "+objectTypeString)
	}

	return err
}
