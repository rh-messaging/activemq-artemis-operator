package secrets

import (
	"context"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/random"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package secrets")

func MakeUserPasswordStringData(keyName string, valueName string, key string, value string) map[string]string {

	if 0 == len(key) {
		key = random.GenerateRandomString(8)
	}

	if 0 == len(value) {
		value = random.GenerateRandomString(8)
	}

	stringDataMap := map[string]string {
		keyName: key,
		valueName: value,
	}

	return stringDataMap
}

func MakeUserPasswordSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) corev1.Secret {

	userPasswordSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    selectors.LabelBuilder.Labels(),
			Name:      secretName,
			Namespace: customResource.Namespace,

		},
		StringData: stringData,
	}

	return userPasswordSecret
}

func NewUserPasswordSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) *corev1.Secret {

	userPasswordSecret := MakeUserPasswordSecret(customResource, secretName, stringData)

	return &userPasswordSecret
}

func Retrieve(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client, secretDefinition *corev1.Secret) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + secretDefinition.Name + " secret")

	var err error = nil
	if err = client.Get(context.TODO(), namespacedName, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Secret " + secretDefinition.Name + " IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("Secret " + secretDefinition.Name + " found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return err
}

func Create(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, secretDefinition *corev1.Secret) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + secretDefinition.Name + " secret")

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, secretDefinition, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + secretDefinition.Name + " secret")
	}
	reqLogger.Info("Set controller reference for new " + secretDefinition.Name + " secret")

	// Call k8s create for service
	if err = client.Create(context.TODO(), secretDefinition); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + secretDefinition.Name + " secret")
	}
	reqLogger.Info("Created new " + secretDefinition.Name + " secret")

	return err
}
