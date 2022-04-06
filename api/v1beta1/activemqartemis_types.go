/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"github.com/RHsyseng/operator-utils/pkg/olm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisSpec defines the desired state of ActiveMQArtemis
type ActiveMQArtemisSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// User name for standard broker user. It is required for connecting to the broker and the web console. If left empty, it will be generated.
	AdminUser string `json:"adminUser,omitempty"`
	// Password for standard broker user. It is required for connecting to the broker and the web console. If left empty, it will be generated.
	AdminPassword string `json:"adminPassword,omitempty"`
	//
	DeploymentPlan DeploymentPlanType `json:"deploymentPlan,omitempty"`
	// Acceptor configuration
	Acceptors  []AcceptorType  `json:"acceptors,omitempty"`
	Connectors []ConnectorType `json:"connectors,omitempty"`
	Console    ConsoleType     `json:"console,omitempty"`
	// The version of the broker deployment.
	Version         string                  `json:"version,omitempty"`
	Upgrades        ActiveMQArtemisUpgrades `json:"upgrades,omitempty"`
	AddressSettings AddressSettingsType     `json:"addressSettings,omitempty"`

	// Optional list of key=value properties that are applied to the broker configuration bean.
	BrokerProperties []string `json:"brokerProperties,omitempty"`
}

type AddressSettingsType struct {
	// How to merge the address settings to broker configuration
	ApplyRule      *string              `json:"applyRule,omitempty"`
	AddressSetting []AddressSettingType `json:"addressSetting,omitempty"`
}

type AddressSettingType struct {
	// the address to send dead messages to
	DeadLetterAddress *string `json:"deadLetterAddress,omitempty"`
	// whether or not to automatically create the dead-letter-address and/or a corresponding queue on that address when a message found to be undeliverable
	AutoCreateDeadLetterResources *bool `json:"autoCreateDeadLetterResources,omitempty"`
	// the prefix to use for auto-created dead letter queues
	DeadLetterQueuePrefix *string `json:"deadLetterQueuePrefix,omitempty"`
	// the suffix to use for auto-created dead letter queues
	DeadLetterQueueSuffix *string `json:"deadLetterQueueSuffix,omitempty"`
	// the address to send expired messages to
	ExpiryAddress *string `json:"expiryAddress,omitempty"`
	// whether or not to automatically create the expiry-address and/or a corresponding queue on that address when a message is sent to a matching queue
	AutoCreateExpiryResources *bool `json:"autoCreateExpiryResources,omitempty"`
	// the prefix to use for auto-created expiry queues
	ExpiryQueuePrefix *string `json:"expiryQueuePrefix,omitempty"`
	// the suffix to use for auto-created expiry queues
	ExpiryQueueSuffix *string `json:"expiryQueueSuffix,omitempty"`
	// Overrides the expiration time for messages using the default value for expiration time. "-1" disables this setting.
	ExpiryDelay *int32 `json:"expiryDelay,omitempty"`
	// Overrides the expiration time for messages using a lower value. "-1" disables this setting.
	MinExpiryDelay *int32 `json:"minExpiryDelay,omitempty"`
	// Overrides the expiration time for messages using a higher value. "-1" disables this setting.
	MaxExpiryDelay *int32 `json:"maxExpiryDelay,omitempty"`
	// the time (in ms) to wait before redelivering a cancelled message.
	RedeliveryDelay *int32 `json:"redeliveryDelay,omitempty"`
	// multiplier to apply to the redelivery-delay
	RedeliveryDelayMultiplier *string `json:"redeliveryDelayMultiplier,omitempty"`
	// factor by which to modify the redelivery delay slightly to avoid collisions
	RedeliveryCollisionAvoidanceFactor *string `json:"redeliveryCollisionAvoidanceFactor,omitempty"`
	// Maximum value for the redelivery-delay
	MaxRedeliveryDelay *int32 `json:"maxRedeliveryDelay,omitempty"`
	// how many times to attempt to deliver a message before sending to dead letter address
	MaxDeliveryAttempts *int32 `json:"maxDeliveryAttempts,omitempty"`
	// the maximum size in bytes for an address. -1 means no limits. This is used in PAGING, BLOCK and FAIL policies. Supports byte notation like K, Mb, GB, etc.
	MaxSizeBytes *string `json:"maxSizeBytes,omitempty"`
	// used with the address full BLOCK policy, the maximum size in bytes an address can reach before messages start getting rejected. Works in combination with max-size-bytes for AMQP protocol only.  Default = -1 (no limit).
	MaxSizeBytesRejectThreshold *int32 `json:"maxSizeBytesRejectThreshold,omitempty"`
	// The page size in bytes to use for an address. Supports byte notation like K, Mb, GB, etc.
	PageSizeBytes *string `json:"pageSizeBytes,omitempty"`
	// Number of paging files to cache in memory to avoid IO during paging navigation
	PageMaxCacheSize *int32 `json:"pageMaxCacheSize,omitempty"`
	// what happens when an address where maxSizeBytes is specified becomes full
	AddressFullPolicy *string `json:"addressFullPolicy,omitempty"`
	// how many days to keep message counter history for this address
	MessageCounterHistoryDayLimit *int32 `json:"messageCounterHistoryDayLimit,omitempty"`
	// This is deprecated please use default-last-value-queue instead.
	LastValueQueue *bool `json:"lastValueQueue,omitempty"`
	// whether to treat the queues under the address as a last value queues by default
	DefaultLastValueQueue *bool `json:"defaultLastValueQueue,omitempty"`
	// the property to use as the key for a last value queue by default
	DefaultLastValueKey *string `json:"defaultLastValueKey,omitempty"`
	// whether the queue should be non-destructive by default
	DefaultNonDestructive *bool `json:"defaultNonDestructive,omitempty"`
	// whether to treat the queues under the address as exclusive queues by default
	DefaultExclusiveQueue *bool `json:"defaultExclusiveQueue,omitempty"`
	// whether to rebalance groups when a consumer is added
	DefaultGroupRebalance *bool `json:"defaultGroupRebalance,omitempty"`
	// whether to pause dispatch when rebalancing groups
	DefaultGroupRebalancePauseDispatch *bool `json:"defaultGroupRebalancePauseDispatch,omitempty"`
	// number of buckets to use for grouping, -1 (default) is unlimited and uses the raw group, 0 disables message groups.
	DefaultGroupBuckets *int32 `json:"defaultGroupBuckets,omitempty"`
	// key used to mark a message is first in a group for a consumer
	DefaultGroupFirstKey *string `json:"defaultGroupFirstKey,omitempty"`
	// the default number of consumers needed before dispatch can start for queues under the address.
	DefaultConsumersBeforeDispatch *int32 `json:"defaultConsumersBeforeDispatch,omitempty"`
	// the default delay (in milliseconds) to wait before dispatching if number of consumers before dispatch is not met for queues under the address.
	DefaultDelayBeforeDispatch *int32 `json:"defaultDelayBeforeDispatch,omitempty"`
	// how long (in ms) to wait after the last consumer is closed on a queue before redistributing messages.
	RedistributionDelay *int32 `json:"redistributionDelay,omitempty"`
	// if there are no queues matching this address, whether to forward message to DLA (if it exists for this address)
	SendToDlaOnNoRoute *bool `json:"sendToDlaOnNoRoute,omitempty"`
	// The minimum rate of message consumption allowed before a consumer is considered "slow." Measured in messages-per-second.
	SlowConsumerThreshold *int32 `json:"slowConsumerThreshold,omitempty"`
	// what happens when a slow consumer is identified
	SlowConsumerPolicy *string `json:"slowConsumerPolicy,omitempty"`
	// How often to check for slow consumers on a particular queue. Measured in seconds.
	SlowConsumerCheckPeriod *int32 `json:"slowConsumerCheckPeriod,omitempty"`
	// DEPRECATED. whether or not to automatically create JMS queues when a producer sends or a consumer connects to a queue
	AutoCreateJmsQueues *bool `json:"autoCreateJmsQueues,omitempty"`
	// DEPRECATED. whether or not to delete auto-created JMS queues when the queue has 0 consumers and 0 messages
	AutoDeleteJmsQueues *bool `json:"autoDeleteJmsQueues,omitempty"`
	// DEPRECATED. whether or not to automatically create JMS topics when a producer sends or a consumer subscribes to a topic
	AutoCreateJmsTopics *bool `json:"autoCreateJmsTopics,omitempty"`
	// DEPRECATED. whether or not to delete auto-created JMS topics when the last subscription is closed
	AutoDeleteJmsTopics *bool `json:"autoDeleteJmsTopics,omitempty"`
	// whether or not to automatically create a queue when a client sends a message to or attempts to consume a message from a queue
	AutoCreateQueues *bool `json:"autoCreateQueues,omitempty"`
	// whether or not to delete auto-created queues when the queue has 0 consumers and 0 messages
	AutoDeleteQueues *bool `json:"autoDeleteQueues,omitempty"`
	// whether or not to delete created queues when the queue has 0 consumers and 0 messages
	AutoDeleteCreatedQueues *bool `json:"autoDeleteCreatedQueues,omitempty"`
	// how long to wait (in milliseconds) before deleting auto-created queues after the queue has 0 consumers.
	AutoDeleteQueuesDelay *int32 `json:"autoDeleteQueuesDelay,omitempty"`
	// the message count the queue must be at or below before it can be evaluated to be auto deleted, 0 waits until empty queue (default) and -1 disables this check.
	AutoDeleteQueuesMessageCount *int32 `json:"autoDeleteQueuesMessageCount,omitempty"`
	//What to do when a queue is no longer in broker.xml.  OFF = will do nothing queues will remain, FORCE = delete queues even if messages remaining.
	ConfigDeleteQueues *string `json:"configDeleteQueues,omitempty"`
	// whether or not to automatically create addresses when a client sends a message to or attempts to consume a message from a queue mapped to an address that doesnt exist
	AutoCreateAddresses *bool `json:"autoCreateAddresses,omitempty"`
	// whether or not to delete auto-created addresses when it no longer has any queues
	AutoDeleteAddresses *bool `json:"autoDeleteAddresses,omitempty"`
	// how long to wait (in milliseconds) before deleting auto-created addresses after they no longer have any queues
	AutoDeleteAddressesDelay *int32 `json:"autoDeleteAddressesDelay,omitempty"`
	// What to do when an address is no longer in broker.xml.  OFF = will do nothing addresses will remain, FORCE = delete address and its queues even if messages remaining.
	ConfigDeleteAddresses *string `json:"configDeleteAddresses,omitempty"`
	// how many message a management resource can browse
	ManagementBrowsePageSize *int32 `json:"managementBrowsePageSize,omitempty"`
	// purge the contents of the queue once there are no consumers
	DefaultPurgeOnNoConsumers *bool `json:"defaultPurgeOnNoConsumers,omitempty"`
	// the maximum number of consumers allowed on this queue at any one time
	DefaultMaxConsumers *int32 `json:"defaultMaxConsumers,omitempty"`
	// the routing-type used on auto-created queues
	DefaultQueueRoutingType *string `json:"defaultQueueRoutingType,omitempty"`
	// the routing-type used on auto-created addresses
	DefaultAddressRoutingType *string `json:"defaultAddressRoutingType,omitempty"`
	// the default window size for a consumer
	DefaultConsumerWindowSize *int32 `json:"defaultConsumerWindowSize,omitempty"`
	// the default ring-size value for any matching queue which doesnt have ring-size explicitly defined
	DefaultRingSize *int32 `json:"defaultRingSize,omitempty"`
	// the number of messages to preserve for future queues created on the matching address
	RetroactiveMessageCount *int32 `json:"retroactiveMessageCount,omitempty"`
	// whether or not to enable metrics for metrics plugins on the matching address
	EnableMetrics *bool `json:"enableMetrics,omitempty"`
	// pattern for matching settings against addresses; can use wildards
	Match string `json:"match,omitempty"`
	// max size of the message returned from management API, default 256
	ManagementMessageAttributeSizeLimit *int32 `json:"managementMessageAttributeSizeLimit,omitempty"`
	// Unit used in specifying slow consumer threshold, default is MESSAGE_PER_SECOND
	SlowConsumerThresholdMeasurementUnit *string `json:"slowConsumerThresholdMeasurementUnit,omitempty"`
	// Whether or not set the timestamp of arrival on messages. default false
	EnableIngressTimestamp *bool `json:"enableIngressTimestamp,omitempty"`
}

type DeploymentPlanType struct {
	//The image used for the broker deployment
	Image string `json:"image,omitempty"`
	// The init container image used to configure broker
	InitImage string `json:"initImage,omitempty"`
	// The number of broker pods to deploy
	Size int32 `json:"size,omitempty"`
	// If true require user password login credentials for broker protocol ports
	RequireLogin bool `json:"requireLogin,omitempty"`
	// If true use persistent volume via persistent volume claim for journal storage
	PersistenceEnabled bool `json:"persistenceEnabled,omitempty"`
	// If aio use ASYNCIO, if nio use NIO for journal IO
	JournalType string `json:"journalType,omitempty"`
	//If true migrate messages on scaledown
	MessageMigration *bool                       `json:"messageMigration,omitempty"`
	Resources        corev1.ResourceRequirements `json:"resources,omitempty"`
	Storage          StorageType                 `json:"storage,omitempty"`
	// If true enable the Jolokia JVM Agent
	JolokiaAgentEnabled bool `json:"jolokiaAgentEnabled,omitempty"`
	// If true enable the management role based access control
	ManagementRBACEnabled bool            `json:"managementRBACEnabled,omitempty"`
	ExtraMounts           ExtraMountsType `json:"extraMounts,omitempty"`
	// Whether broker is clustered
	Clustered      *bool           `json:"clustered,omitempty"`
	PodSecurity    PodSecurityType `json:"podSecurity,omitempty"`
	LivenessProbe  *corev1.Probe   `json:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe   `json:"readinessProbe,omitempty"`
	// Whether or not to install the artemis metrics plugin
	EnableMetricsPlugin *bool               `json:"enableMetricsPlugin,omitempty"`
	Tolerations         []corev1.Toleration `json:"tolerations,omitempty"`
	//custom labels provided in the cr
	Labels map[string]string `json:"labels,omitempty"`
	//custom node selector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	//custom Affinity
	Affinity           corev1.Affinity            `json:"affinity,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

type PodSecurityType struct {
	// ServiceAccount Name of the pod
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// runAsUser as defined in PodSecurityContext for the pod
	RunAsUser *int64 `json:"runAsUser,omitempty"`
}

type ExtraMountsType struct {
	// Name of ConfigMap
	ConfigMaps []string `json:"configMaps,omitempty"`
	// Name of Secret
	Secrets []string `json:"secrets,omitempty"`
}

type StorageType struct {
	Size string `json:"size,omitempty"`
	// The storageClassName to be used in PVC
	StorageClassName string `json:"storageClassName,omitempty"`
}

type AcceptorType struct {
	Name string `json:"name"`
	// Port number
	Port int32 `json:"port,omitempty"`
	// The protocols to enable for this acceptor
	Protocols string `json:"protocols,omitempty"`
	// Whether or not to enable SSL on this port
	SSLEnabled bool `json:"sslEnabled,omitempty"`
	// Name of the secret to use for ssl information
	SSLSecret string `json:"sslSecret,omitempty"`
	// Comma separated list of cipher suites used for SSL communication.
	EnabledCipherSuites string `json:"enabledCipherSuites,omitempty"`
	// Comma separated list of protocols used for SSL communication.
	EnabledProtocols string `json:"enabledProtocols,omitempty"`
	// Tells a client connecting to this acceptor that 2-way SSL is required. This property takes precedence over wantClientAuth.
	NeedClientAuth bool `json:"needClientAuth,omitempty"`
	// Tells a client connecting to this acceptor that 2-way SSL is requested but not required. Overridden by needClientAuth.
	WantClientAuth bool `json:"wantClientAuth,omitempty"`
	// The CN of the connecting client's SSL certificate will be compared to its hostname to verify they match. This is useful only for 2-way SSL.
	VerifyHost bool `json:"verifyHost,omitempty"`
	// Used to change the SSL Provider between JDK and OPENSSL. The default is JDK.
	SSLProvider string `json:"sslProvider,omitempty"`
	// A regular expression used to match the server_name extension on incoming SSL connections. If the name doesn't match then the connection to the acceptor will be rejected.
	SNIHost string `json:"sniHost,omitempty"`
	// Whether or not to expose this acceptor
	Expose bool `json:"expose,omitempty"`
	// To indicate which kind of routing type to use.
	AnycastPrefix string `json:"anycastPrefix,omitempty"`
	// To indicate which kind of routing type to use
	MulticastPrefix string `json:"multicastPrefix,omitempty"`
	// Max number of connections allowed to make
	ConnectionsAllowed int `json:"connectionsAllowed,omitempty"`
	// AMQP Minimum Large Message Size
	AMQPMinLargeMessageSize int `json:"amqpMinLargeMessageSize,omitempty"`
	// For openwire protocol if advisory topics are enabled, default false
	SupportAdvisory *bool `json:"supportAdvisory,omitempty"`
	// If prevents advisory addresses/queues to be registered to management service, default false
	SuppressInternalManagementObjects *bool `json:"suppressInternalManagementObjects,omitempty"`
	// Whether to let the acceptor to bind to all interfaces (0.0.0.0)
	BindToAllInterfaces *bool `json:"bindToAllInterfaces,omitempty"`
	// Type of keystore being used; "JKS", "JCEKS", "PKCS12", etc. Default is "JKS"
	KeyStoreType string `json:"keyStoreType,omitempty"`
	// Provider used for the keystore; "SUN", "SunJCE", etc. Default is null
	KeyStoreProvider string `json:"keyStoreProvider,omitempty"`
	// Type of truststore being used; "JKS", "JCEKS", "PKCS12", etc. Default in broker is "JKS"
	TrustStoreType string `json:"trustStoreType,omitempty"`
	// Provider used for the truststore; "SUN", "SunJCE", etc. Default in broker is null
	TrustStoreProvider string `json:"trustStoreProvider,omitempty"`
}

type ConnectorType struct {
	// The name of the connector
	Name string `json:"name"`
	// The type either tcp or vm
	Type string `json:"type,omitempty"`
	// Hostname or IP to connect to
	Host string `json:"host"`
	// Port number
	Port int32 `json:"port"`
	//  Whether or not to enable SSL on this port
	SSLEnabled bool `json:"sslEnabled,omitempty"`
	// Name of the secret to use for ssl information
	SSLSecret string `json:"sslSecret,omitempty"`
	// Comma separated list of cipher suites used for SSL communication.
	EnabledCipherSuites string `json:"enabledCipherSuites,omitempty"`
	// Comma separated list of protocols used for SSL communication.
	EnabledProtocols string `json:"enabledProtocols,omitempty"`
	// Tells a client connecting to this connector that 2-way SSL is required. This property takes precedence over wantClientAuth.
	NeedClientAuth bool `json:"needClientAuth,omitempty"`
	// Tells a client connecting to this connector that 2-way SSL is requested but not required. Overridden by needClientAuth.
	WantClientAuth bool `json:"wantClientAuth,omitempty"`
	// The CN of the connecting client's SSL certificate will be compared to its hostname to verify they match. This is useful only for 2-way SSL.
	VerifyHost bool `json:"verifyHost,omitempty"`
	// Used to change the SSL Provider between JDK and OPENSSL. The default is JDK.
	SSLProvider string `json:"sslProvider,omitempty"`
	// A regular expression used to match the server_name extension on incoming SSL connections. If the name doesn't match then the connection to the acceptor will be rejected.
	SNIHost string `json:"sniHost,omitempty"`
	// Whether or not to expose this connector
	Expose bool `json:"expose,omitempty"`
	// Type of keystore being used; "JKS", "JCEKS", "PKCS12", etc. Default is "JKS"
	KeyStoreType string `json:"keyStoreType,omitempty"`
	// Provider used for the keystore; "SUN", "SunJCE", etc. Default is null
	KeyStoreProvider string `json:"keyStoreProvider,omitempty"`
	// Type of truststore being used; "JKS", "JCEKS", "PKCS12", etc. Default in broker is "JKS"
	TrustStoreType string `json:"trustStoreType,omitempty"`
	// Provider used for the truststore; "SUN", "SunJCE", etc. Default in broker is null
	TrustStoreProvider string `json:"trustStoreProvider,omitempty"`
}

type ConsoleType struct {
	// Whether or not to expose this port
	Expose bool `json:"expose,omitempty"`
	// Whether or not to enable SSL on this port
	SSLEnabled bool `json:"sslEnabled,omitempty"`
	// Name of the secret to use for ssl information
	SSLSecret string `json:"sslSecret,omitempty"`
	// If the embedded server requires client authentication
	UseClientAuth bool `json:"useClientAuth,omitempty"`
}

// ActiveMQArtemis App product upgrade flags
type ActiveMQArtemisUpgrades struct {
	// Set to true to enable automatic micro version product upgrades, disabled by default.
	Enabled bool `json:"enabled"`
	// Set to true to enable automatic micro version product upgrades, disabled by default. Requires spec.upgrades.enabled true.
	Minor bool `json:"minor"`
}

// ActiveMQArtemisStatus defines the observed state of ActiveMQArtemis
type ActiveMQArtemisStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Pod Status
	PodStatus olm.DeploymentStatus `json:"podStatus"`
	//Deployments olm.DeploymentStatus `json:"podStatus"`
	//we probably use Deployments as operatorHub shows invalid field podStatus
	//see 3scale https://github.com/3scale/3scale-operator/blob/8abbabd926616b98db0e7e736e68e5ceba90ed9d/apis/apps/v1alpha1/apimanager_types.go#L87
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:path=activemqartemises

// ActiveMQArtemis is the Schema for the activemqartemises API
type ActiveMQArtemis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ActiveMQArtemisList contains a list of ActiveMQArtemis
type ActiveMQArtemisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemis{}, &ActiveMQArtemisList{})
}

func (r *ActiveMQArtemis) Hub() {
}
