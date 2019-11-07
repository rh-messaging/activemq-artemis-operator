package secrets

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/random"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
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

func MakeSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) corev1.Secret {

	secretDefinition := corev1.Secret{
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

	return secretDefinition
}

func NewSecret(customResource *brokerv2alpha1.ActiveMQArtemis, secretName string, stringData map[string]string) *corev1.Secret {

	secretDefinition := MakeSecret(customResource, secretName, stringData)

	return &secretDefinition
}

func Create(customResource *brokerv2alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, stringDataMap map[string]string, client client.Client, scheme *runtime.Scheme) *corev1.Secret {

	var err error = nil
	secretDefinition := NewSecret(customResource, namespacedName.Name, stringDataMap)

	if err = resources.Retrieve(customResource, namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			err = resources.Create(customResource, client, scheme, secretDefinition)
		}
	}

	return secretDefinition
}
