package v2alpha2_test

import (
	brokerv2alpha2 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	. "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha2/activemqartemis"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/containers"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	NameBuilder namer.NamerData

	labels = selectors.LabelBuilder.Labels()
	f      = false
	t      = true

	AMQinstance = brokerv2alpha2.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test",
			Namespace: "activemq-artemis-operator-ns",
			Labels:    labels,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: "broker.amq.io/v2alpha1",
		},
		Spec: brokerv2alpha2.ActiveMQArtemisSpec{
			AdminUser:     "admin",
			AdminPassword: "admin",
			DeploymentPlan: brokerv2alpha2.DeploymentPlanType{
				Size:               2,
				Image:              "quay.io/artemiscloud/activemq-artemis-operator:0.16.0",
				PersistenceEnabled: false,
				RequireLogin:       false,
				MessageMigration:   &f,
			},
			Console: brokerv2alpha2.ConsoleType{
				Expose: true,
			},
			Acceptors: []brokerv2alpha2.AcceptorType{
				{
					Name:                "my",
					Protocols:           "amqp",
					Port:                5672,
					SSLEnabled:          false,
					EnabledCipherSuites: "SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA",
					EnabledProtocols:    " TLSv1,TLSv1.1,TLSv1.2",
					NeedClientAuth:      true,
					WantClientAuth:      true,
					VerifyHost:          true,
					SSLProvider:         "JDK",
					SNIHost:             "localhost",
					Expose:              true,
					AnycastPrefix:       "jms.queue.",
					MulticastPrefix:     "/topic/",
				},
			},
			Connectors: []brokerv2alpha2.ConnectorType{
				{
					Name:                "my-c",
					Host:                "localhost",
					Port:                22222,
					SSLEnabled:          false,
					EnabledCipherSuites: "SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA",
					EnabledProtocols:    " TLSv1,TLSv1.1,TLSv1.2",
					NeedClientAuth:      true,
					WantClientAuth:      true,
					VerifyHost:          true,
					SSLProvider:         "JDK",
					SNIHost:             "localhost",
					Expose:              true,
				},
			},
		},
	}

	AMQinstance2 = brokerv2alpha2.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test-2",
			Namespace: "activemq-artemis-operator-ns-2",
			Labels:    labels,
		},
		Spec: brokerv2alpha2.ActiveMQArtemisSpec{
			AdminUser:     "admin",
			AdminPassword: "admin",
			DeploymentPlan: brokerv2alpha2.DeploymentPlanType{
				Size:               0,
				Image:              "quay.io/artemiscloud/activemq-artemis-broker-kubernetes:0.1.0",
				PersistenceEnabled: false,
				JournalType:        "nio",
				RequireLogin:       false,
				MessageMigration:   &f,
			},
			Console: brokerv2alpha2.ConsoleType{
				Expose: true,
			},
			Acceptors: []brokerv2alpha2.AcceptorType{
				{
					Name:                "my",
					Protocols:           "amqp",
					Port:                5672,
					SSLEnabled:          true,
					EnabledCipherSuites: "SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA",
					EnabledProtocols:    " TLSv1,TLSv1.1,TLSv1.2",
					NeedClientAuth:      true,
					WantClientAuth:      true,
					VerifyHost:          true,
					SSLProvider:         "JDK",
					SNIHost:             "localhost",
					Expose:              true,
					AnycastPrefix:       "jms.queue.",
					MulticastPrefix:     "/topic/",
				},
			},
			Connectors: []brokerv2alpha2.ConnectorType{
				{
					Name:                "my-c",
					Host:                "localhost",
					Port:                22222,
					SSLEnabled:          true,
					EnabledCipherSuites: "SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA",
					EnabledProtocols:    " TLSv1,TLSv1.1,TLSv1.2",
					NeedClientAuth:      true,
					WantClientAuth:      true,
					VerifyHost:          true,
					SSLProvider:         "JDK",
					SNIHost:             "localhost",
					Expose:              true,
				},
			},
		},
	}

	AMQInstanceWithoutSpec = brokerv2alpha2.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test",
			Namespace: "activemq-artemis-operator-ns",
		},
	}

	namespacedName = types.NamespacedName{
		Namespace: AMQinstance.Namespace,
		Name:      AMQinstance.Name,
	}
	container   = containers.MakeContainer(namespacedName.Name, "quay.io/artemiscloud/activemq-artemis-operator:0.16.0", MakeEnvVarArrayForCR(&AMQinstance))
	podTemplate = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: AMQinstance.Namespace,
			Name:      AMQinstance.Name,
			Labels:    AMQinstance.Labels,
		},
		//Spec: pods.NewPodTemplateSpecForCR(&AMQinstance).Spec,
		Spec: pods.MakePodTemplateSpec(namespacedName, selectors.LabelBuilder.Labels()).Spec,
	}
)
