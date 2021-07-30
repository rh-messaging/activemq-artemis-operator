package v2alpha5

import (
	"github.com/RHsyseng/operator-utils/pkg/olm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisSpec defines the desired state of ActiveMQArtemis
// +k8s:openapi-gen=true
type ActiveMQArtemisSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	AdminUser      string                  `json:"adminUser,omitempty"`
	AdminPassword  string                  `json:"adminPassword,omitempty"`
	DeploymentPlan DeploymentPlanType      `json:"deploymentPlan,omitempty"`
	Acceptors      []AcceptorType          `json:"acceptors,omitempty"`
	Connectors     []ConnectorType         `json:"connectors,omitempty"`
	Console        ConsoleType             `json:"console,omitempty"`
	Version        string                  `json:"version,omitempty"`
	Upgrades       ActiveMQArtemisUpgrades `json:"upgrades,omitempty"`
	//below are v2alpha4 types
	AddressSettings AddressSettingsType `json:"addressSettings,omitempty"`
}

type AddressSettingsType struct {
	ApplyRule      *string              `json:"applyRule,omitempty"`
	AddressSetting []AddressSettingType `json:"addressSetting,omitempty"`
}

type AddressSettingType struct {
	DeadLetterAddress                  *string  `json:"deadLetterAddress,omitempty"`
	AutoCreateDeadLetterResources      *bool    `json:"autoCreateDeadLetterResources,omitempty"`
	DeadLetterQueuePrefix              *string  `json:"deadLetterQueuePrefix,omitempty"`
	DeadLetterQueueSuffix              *string  `json:"deadLetterQueueSuffix,omitempty"`
	ExpiryAddress                      *string  `json:"expiryAddress,omitempty"`
	AutoCreateExpiryResources          *bool    `json:"autoCreateExpiryResources,omitempty"`
	ExpiryQueuePrefix                  *string  `json:"expiryQueuePrefix,omitempty"`
	ExpiryQueueSuffix                  *string  `json:"expiryQueueSuffix,omitempty"`
	ExpiryDelay                        *int32   `json:"expiryDelay,omitempty"`
	MinExpiryDelay                     *int32   `json:"minExpiryDelay,omitempty"`
	MaxExpiryDelay                     *int32   `json:"maxExpiryDelay,omitempty"`
	RedeliveryDelay                    *int32   `json:"redeliveryDelay,omitempty"`
	RedeliveryDelayMultiplier          *int32   `json:"redeliveryDelayMultiplier,omitempty"`
	RedeliveryCollisionAvoidanceFactor *float32 `json:"redeliveryCollisionAvoidanceFactor,omitempty"`
	MaxRedeliveryDelay                 *int32   `json:"maxRedeliveryDelay,omitempty"`
	MaxDeliveryAttempts                *int32   `json:"maxDeliveryAttempts,omitempty"`
	MaxSizeBytes                       *string  `json:"maxSizeBytes,omitempty"`
	MaxSizeBytesRejectThreshold        *int32   `json:"maxSizeBytesRejectThreshold,omitempty"`
	PageSizeBytes                      *string  `json:"pageSizeBytes,omitempty"`
	PageMaxCacheSize                   *int32   `json:"pageMaxCacheSize,omitempty"`
	AddressFullPolicy                  *string  `json:"addressFullPolicy,omitempty"`
	MessageCounterHistoryDayLimit      *int32   `json:"messageCounterHistoryDayLimit,omitempty"`
	LastValueQueue                     *bool    `json:"lastValueQueue,omitempty"`
	DefaultLastValueQueue              *bool    `json:"defaultLastValueQueue,omitempty"`
	DefaultLastValueKey                *string  `json:"defaultLastValueKey,omitempty"`
	DefaultNonDestructive              *bool    `json:"defaultNonDestructive,omitempty"`
	DefaultExclusiveQueue              *bool    `json:"defaultExclusiveQueue,omitempty"`
	DefaultGroupRebalance              *bool    `json:"defaultGroupRebalance,omitempty"`
	DefaultGroupRebalancePauseDispatch *bool    `json:"defaultGroupRebalancePauseDispatch,omitempty"`
	DefaultGroupBuckets                *int32   `json:"defaultGroupBuckets,omitempty"`
	DefaultGroupFirstKey               *string  `json:"defaultGroupFirstKey,omitempty"`
	DefaultConsumersBeforeDispatch     *int32   `json:"defaultConsumersBeforeDispatch,omitempty"`
	DefaultDelayBeforeDispatch         *int32   `json:"defaultDelayBeforeDispatch,omitempty"`
	RedistributionDelay                *int32   `json:"redistributionDelay,omitempty"`
	SendToDlaOnNoRoute                 *bool    `json:"sendToDlaOnNoRoute,omitempty"`
	SlowConsumerThreshold              *int32   `json:"slowConsumerThreshold,omitempty"`
	SlowConsumerPolicy                 *string  `json:"slowConsumerPolicy,omitempty"`
	SlowConsumerCheckPeriod            *int32   `json:"slowConsumerCheckPeriod,omitempty"`
	AutoCreateJmsQueues                *bool    `json:"autoCreateJmsQueues,omitempty"`
	AutoDeleteJmsQueues                *bool    `json:"autoDeleteJmsQueues,omitempty"`
	AutoCreateJmsTopics                *bool    `json:"autoCreateJmsTopics,omitempty"`
	AutoDeleteJmsTopics                *bool    `json:"autoDeleteJmsTopics,omitempty"`
	AutoCreateQueues                   *bool    `json:"autoCreateQueues,omitempty"`
	AutoDeleteQueues                   *bool    `json:"autoDeleteQueues,omitempty"`
	AutoDeleteCreatedQueues            *bool    `json:"autoDeleteCreatedQueues,omitempty"`
	AutoDeleteQueuesDelay              *int32   `json:"autoDeleteQueuesDelay,omitempty"`
	AutoDeleteQueuesMessageCount       *int32   `json:"autoDeleteQueuesMessageCount,omitempty"`
	ConfigDeleteQueues                 *string  `json:"configDeleteQueues,omitempty"`
	AutoCreateAddresses                *bool    `json:"autoCreateAddresses,omitempty"`
	AutoDeleteAddresses                *bool    `json:"autoDeleteAddresses,omitempty"`
	AutoDeleteAddressesDelay           *int32   `json:"autoDeleteAddressesDelay,omitempty"`
	ConfigDeleteAddresses              *string  `json:"configDeleteAddresses,omitempty"`
	ManagementBrowsePageSize           *int32   `json:"managementBrowsePageSize,omitempty"`
	DefaultPurgeOnNoConsumers          *bool    `json:"defaultPurgeOnNoConsumers,omitempty"`
	DefaultMaxConsumers                *int32   `json:"defaultMaxConsumers,omitempty"`
	DefaultQueueRoutingType            *string  `json:"defaultQueueRoutingType,omitempty"`
	DefaultAddressRoutingType          *string  `json:"defaultAddressRoutingType,omitempty"`
	DefaultConsumerWindowSize          *int32   `json:"defaultConsumerWindowSize,omitempty"`
	DefaultRingSize                    *int32   `json:"defaultRingSize,omitempty"`
	RetroactiveMessageCount            *int32   `json:"retroactiveMessageCount,omitempty"`
	EnableMetrics                      *bool    `json:"enableMetrics,omitempty"`
	Match                              string   `json:"match,omitempty"`
}

type DeploymentPlanType struct {
	Image                 string                      `json:"image,omitempty"`
	InitImage             string                      `json:"initImage,omitempty"`
	Size                  int32                       `json:"size,omitempty"`
	RequireLogin          bool                        `json:"requireLogin,omitempty"`
	PersistenceEnabled    bool                        `json:"persistenceEnabled,omitempty"`
	JournalType           string                      `json:"journalType,omitempty"`
	MessageMigration      *bool                       `json:"messageMigration,omitempty"`
	Resources             corev1.ResourceRequirements `json:"resources,omitempty"`
	Storage               StorageType                 `json:"storage,omitempty"`
	JolokiaAgentEnabled   bool                        `json:"jolokiaAgentEnabled,omitempty"`
	ManagementRBACEnabled bool                        `json:"managementRBACEnabled,omitempty"`
	ExtraMounts           ExtraMountsType             `json:"extraMounts,omitempty"`
	Clustered             *bool                       `json:"clustered,omitempty"`
}

type ExtraMountsType struct {
	ConfigMaps []string `json:"configMaps,omitempty"`
	Secrets    []string `json:"secrets,omitempty"`
}

type StorageType struct {
	Size string `json:"size,omitempty"`
}

type AcceptorType struct {
	Name                    string `json:"name"`
	Port                    int32  `json:"port,omitempty"`
	Protocols               string `json:"protocols,omitempty"`
	SSLEnabled              bool   `json:"sslEnabled,omitempty"`
	SSLSecret               string `json:"sslSecret,omitempty"`
	EnabledCipherSuites     string `json:"enabledCipherSuites,omitempty"`
	EnabledProtocols        string `json:"enabledProtocols,omitempty"`
	NeedClientAuth          bool   `json:"needClientAuth,omitempty"`
	WantClientAuth          bool   `json:"wantClientAuth,omitempty"`
	VerifyHost              bool   `json:"verifyHost,omitempty"`
	SSLProvider             string `json:"sslProvider,omitempty"`
	SNIHost                 string `json:"sniHost,omitempty"`
	Expose                  bool   `json:"expose,omitempty"`
	AnycastPrefix           string `json:"anycastPrefix,omitempty"`
	MulticastPrefix         string `json:"multicastPrefix,omitempty"`
	ConnectionsAllowed      int    `json:"connectionsAllowed,omitempty"`
	AMQPMinLargeMessageSize int    `json:"amqpMinLargeMessageSize,omitempty"`
}

type ConnectorType struct {
	Name                string `json:"name"`
	Type                string `json:"type,omitempty"`
	Host                string `json:"host"`
	Port                int32  `json:"port"`
	SSLEnabled          bool   `json:"sslEnabled,omitempty"`
	SSLSecret           string `json:"sslSecret,omitempty"`
	EnabledCipherSuites string `json:"enabledCipherSuites,omitempty"`
	EnabledProtocols    string `json:"enabledProtocols,omitempty"`
	NeedClientAuth      bool   `json:"needClientAuth,omitempty"`
	WantClientAuth      bool   `json:"wantClientAuth,omitempty"`
	VerifyHost          bool   `json:"verifyHost,omitempty"`
	SSLProvider         string `json:"sslProvider,omitempty"`
	SNIHost             string `json:"sniHost,omitempty"`
	Expose              bool   `json:"expose,omitempty"`
}

type ConsoleType struct {
	Expose        bool   `json:"expose,omitempty"`
	SSLEnabled    bool   `json:"sslEnabled,omitempty"`
	SSLSecret     string `json:"sslSecret,omitempty"`
	UseClientAuth bool   `json:"useClientAuth,omitempty"`
}

// ActiveMQArtemis App product upgrade flags
type ActiveMQArtemisUpgrades struct {
	Enabled bool `json:"enabled"`
	Minor   bool `json:"minor"`
}

// ActiveMQArtemisStatus defines the observed state of ActiveMQArtemis
// +k8s:openapi-gen=true
type ActiveMQArtemisStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	PodStatus olm.DeploymentStatus `json:"podStatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemis is the Schema for the activemqartemis API
// +k8s:openapi-gen=true
// +genclient
type ActiveMQArtemis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisList contains a list of ActiveMQArtemis
type ActiveMQArtemisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemis{}, &ActiveMQArtemisList{})
}
