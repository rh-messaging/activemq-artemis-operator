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

func makeUserPasswordStringData(keyName string, valueName string, key string, value string) map[string]string {

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

func makeUserPasswordSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) corev1.Secret {

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

func newUserPasswordSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) *corev1.Secret {

	userPasswordSecret := makeUserPasswordSecret(customResource, secretName, stringData)

	return &userPasswordSecret
}

func RetrieveUserPasswordSecret(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Secret, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "UserPassword" + " secret")

	var err error = nil

	userPasswordStringData := makeUserPasswordStringData("user", "password", cr.Spec.DeploymentPlan.User, cr.Spec.DeploymentPlan.Password)
	userPasswordSecret := newUserPasswordSecret(cr, "amq-app-secret", userPasswordStringData)

	if err = client.Get(context.TODO(), namespacedName, userPasswordSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("UserPassword secret IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("UserPassword secret found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return userPasswordSecret, err
}

func CreateUserPasswordSecret(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Secret, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "UserPassword" + " secret")

	//labels := selectors.LabelsForActiveMQArtemis(cr.Name)
	userPasswordStringData := makeUserPasswordStringData("user", "password", cr.Spec.DeploymentPlan.User, cr.Spec.DeploymentPlan.Password)
	userPasswordSecret := newUserPasswordSecret(cr, "amq-app-secret", userPasswordStringData)

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, userPasswordSecret, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "UserPassword" + " secret")
	}
	reqLogger.Info("Set controller reference for new " + "UserPassword" + " secret")

	// Call k8s create for service
	if err = client.Create(context.TODO(), userPasswordSecret); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "UserPassword" + " secret")
	}
	reqLogger.Info("Created new " + "UserPassword" + " secret")

	return userPasswordSecret, err
}

func RetrieveClusterUserPasswordSecret(cr *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.Secret, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Retrieving " + "ClusterUserPassword" + " secret")

	var err error = nil

	userPasswordStringData := makeUserPasswordStringData("clusterUser", "clusterPassword", cr.Spec.DeploymentPlan.ClusterUser, cr.Spec.DeploymentPlan.ClusterPassword)
	userPasswordSecret := newUserPasswordSecret(cr, "amq-credentials-secret", userPasswordStringData)

	if err = client.Get(context.TODO(), namespacedName, userPasswordSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ClusterUserPassword secret IsNotFound", "Namespace", cr.Namespace, "Name", cr.Name)
		} else {
			reqLogger.Info("ClusterUserPassword secret found", "Namespace", cr.Namespace, "Name", cr.Name)
		}
	}

	return userPasswordSecret, err
}

func CreateClusterUserPasswordSecret(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.Secret, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new " + "ClusterUserPassword" + " secret")

	//labels := selectors.LabelsForActiveMQArtemis(cr.Name)
	userPasswordStringData := makeUserPasswordStringData("clusterUser", "clusterPassword", cr.Spec.DeploymentPlan.ClusterUser, cr.Spec.DeploymentPlan.ClusterPassword)
	userPasswordSecret := newUserPasswordSecret(cr, "amq-credentials-secret", userPasswordStringData)

	// Define the headless Service for the StatefulSet
	// Set ActiveMQArtemis instance as the owner and controller
	var err error = nil
	if err = controllerutil.SetControllerReference(cr, userPasswordSecret, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "ClusterUserPassword" + " secret")
	}
	reqLogger.Info("Set controller reference for new " + "ClusterUserPassword" + " secret")

	// Call k8s create for service
	if err = client.Create(context.TODO(), userPasswordSecret); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "ClusterUserPassword" + " secret")
	}
	reqLogger.Info("Created new " + "ClusterUserPassword" + " secret")

	return userPasswordSecret, err
}
