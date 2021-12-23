package secrets

import (
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/random"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package secrets")
var CredentialsNameBuilder namer.NamerData
var ConsoleNameBuilder namer.NamerData
var NettyNameBuilder namer.NamerData

func MakeStringDataMap(keyName string, valueName string, key string, value string) map[string]string {

	if 0 == len(key) {
		key = random.GenerateRandomString(8)
	}

	if 0 == len(value) {
		value = random.GenerateRandomString(8)
	}

	stringDataMap := map[string]string{
		keyName:   key,
		valueName: value,
	}

	return stringDataMap
}

//func MakeSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) corev1.Secret {
func MakeSecret(namespacedName types.NamespacedName, secretName string, stringData map[string]string, labels map[string]string) corev1.Secret {

	secretDefinition := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      secretName,
			Namespace: namespacedName.Namespace,
		},
		StringData: stringData,
	}

	return secretDefinition
}

//func NewSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) *corev1.Secret {
func NewSecret(namespacedName types.NamespacedName, secretName string, stringData map[string]string, labels map[string]string) *corev1.Secret {

	secretDefinition := MakeSecret(namespacedName, secretName, stringData, labels)

	return &secretDefinition
}

func CreateOrUpdate(owner metav1.Object, namespacedName types.NamespacedName, stringDataMap map[string]string, labels map[string]string, client client.Client, scheme *runtime.Scheme) error {

	var err error = nil
	secretDefinition := NewSecret(namespacedName, namespacedName.Name, stringDataMap, labels)

	if err = resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			err = resources.Create(owner, namespacedName, client, scheme, secretDefinition)
			if err != nil {
				log.Error(err, "failed to create secret", "secret", namespacedName)
			}
		} else {
			log.Error(err, "Error retrieving secret", "secret", namespacedName.Name)
		}
	} else {
		//Update
		secretDefinition = NewSecret(namespacedName, namespacedName.Name, stringDataMap, labels)
		if err = resources.Update(namespacedName, client, secretDefinition); err != nil {
			log.Error(err, "Failed to update secret", "secret", namespacedName.Name)
		}
	}

	return err
}

func Create(owner metav1.Object, namespacedName types.NamespacedName, stringDataMap map[string]string, labels map[string]string, client client.Client, scheme *runtime.Scheme) *corev1.Secret {

	var err error = nil
	secretDefinition := NewSecret(namespacedName, namespacedName.Name, stringDataMap, labels)

	if err = resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			err = resources.Create(owner, namespacedName, client, scheme, secretDefinition)
			if err != nil {
				log.Error(err, "failed to create secret", "secret", namespacedName)
			}
		}
	}

	return secretDefinition
}

func Delete(namespacedName types.NamespacedName, stringDataMap map[string]string, labels map[string]string, client client.Client) {
	secretDefinition := NewSecret(namespacedName, namespacedName.Name, stringDataMap, labels)
	resources.Delete(namespacedName, client, secretDefinition)
}

func RetriveSecret(namespacedName types.NamespacedName, secretName string, labels map[string]string, client client.Client) (*corev1.Secret, error) {
	stringData := make(map[string]string)
	secretDefinition := MakeSecret(namespacedName, secretName, stringData, labels)
	if err := resources.Retrieve(namespacedName, client, &secretDefinition); err != nil {
		return nil, err
	}
	return &secretDefinition, nil
}

func GetValueFromSecret(namespace string, autoCreateSecret bool, autoGenValue bool,
	secretName string, key string, labels map[string]string, client client.Client, scheme *runtime.Scheme, owner metav1.Object) *string {
	//check if the secret exists.
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := make(map[string]string)

	secretDefinition := NewSecret(namespacedName, secretName, stringDataMap, labels)

	if err := resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			if autoCreateSecret {
				log.Info("Auto create secret", "name", secretName)
				//create the secret
				resources.Create(owner, namespacedName, client, scheme, secretDefinition)
			} else {
				log.Info("No secret found", "name", secretName)
				return nil
			}
		}
	} else {
		log.Info("Found secret " + secretName)
		if elem, ok := secretDefinition.Data[key]; ok {
			//the value exists
			value := string(elem)
			return &value
		}
	}
	//not found
	if autoGenValue {
		value := random.GenerateRandomString(8)
		//update the secret
		if secretDefinition.Data == nil {
			secretDefinition.Data = make(map[string][]byte)
		}
		secretDefinition.Data[key] = []byte(value)
		log.Info("Updating secret", "secret", namespacedName.Name)
		if err := resources.Update(namespacedName, client, secretDefinition); err != nil {
			log.Error(err, "failed to update secret", "secret", secretName)
		}
		return &value

	}
	return nil
}
