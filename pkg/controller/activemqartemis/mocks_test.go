package activemqartemis

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/pods"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	NameBuilder namer.NamerData

	labels = selectors.LabelBuilder.Labels()

	AMQinstance = brokerv2alpha1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test",
			Namespace: "activemq-artemis-operator-ns",
			Labels:    labels,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: "broker.amq.io/v2alpha1",
		},
		Spec: brokerv2alpha1.ActiveMQArtemisSpec{
			AdminUser:     "admin",
			AdminPassword: "admin",
			DeploymentPlan: brokerv2alpha1.DeploymentPlanType{
				Size:               3,
				Image:              "registry.redhat.io/amq7/amq-broker:7.5",
				PersistenceEnabled: false,
				RequireLogin:       false,
				MessageMigration:   false,
			},
			Console: brokerv2alpha1.ConsoleType{
				Expose: true,
			},
			Acceptors: []brokerv2alpha1.AcceptorType{
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
					AnycastPrefix:       "jms.topic",
					MulticastPrefix:     "/queue/",
				},
			},
			Connectors: []brokerv2alpha1.ConnectorType{
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

	AMQinstance2 = brokerv2alpha1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test-2",
			Namespace: "activemq-artemis-operator-ns-2",
			Labels:    labels,
		},
		Spec: brokerv2alpha1.ActiveMQArtemisSpec{
			AdminUser:     "admin",
			AdminPassword: "admin",
			DeploymentPlan: brokerv2alpha1.DeploymentPlanType{
				Size:               0,
				Image:              "registry.redhat.io/amq7/amq-broker:7.4",
				PersistenceEnabled: false,
				JournalType:        "nio",
				RequireLogin:       false,
				MessageMigration:   false,
			},
			Console: brokerv2alpha1.ConsoleType{
				Expose: true,
			},
			Acceptors: []brokerv2alpha1.AcceptorType{
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
					AnycastPrefix:       "jms.topic",
					MulticastPrefix:     "/queue/",
				},
			},
			Connectors: []brokerv2alpha1.ConnectorType{
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

	AMQInstanceWithoutSpec = brokerv2alpha1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "activemq-artemis-test",
			Namespace: "activemq-artemis-operator-ns",
		},
	}

	podTemplate = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: AMQinstance.Namespace,
			Name:      AMQinstance.Name,
			Labels:    AMQinstance.Labels,
		},
		Spec: pods.NewPodTemplateSpecForCR(&AMQinstance).Spec,
	}
)
