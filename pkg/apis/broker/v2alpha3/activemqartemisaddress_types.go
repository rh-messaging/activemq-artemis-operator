package v2alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisAddressSpec defines the desired state of ActiveMQArtemisAddress
type ActiveMQArtemisAddressSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	AddressName              string                  `json:"addressName,omitempty"`
	QueueName                *string                 `json:"queueName,omitempty"`
	RoutingType              *string                 `json:"routingType,omitempty"`
	RemoveFromBrokerOnDelete bool                    `json:"removeFromBrokerOnDelete,omitempty"`
	User                     *string                 `json:"user,omitempty"`
	Password                 *string                 `json:"password,omitempty"`
	QueueConfiguration       *QueueConfigurationType `json:"queueConfiguration,omitempty"`
	ApplyToCrNames           []string                `json:"applyToCrNames,omitempty"`
}

type QueueConfigurationType struct {
	IgnoreIfExists              *bool   `json:"ignoreIfExists,omitempty"`
	RoutingType                 *string `json:"routingType,omitempty"`
	FilterString                *string `json:"filterString,omitempty"`
	Durable                     *bool   `json:"durable,omitempty"`
	User                        *string `json:"user,omitempty"`
	MaxConsumers                *int32  `json:"maxConsumers"`
	Exclusive                   *bool   `json:"exclusive,omitempty"`
	GroupRebalance              *bool   `json:"groupRebalance,omitempty"`
	GroupRebalancePauseDispatch *bool   `json:"groupRebalancePauseDispatch,omitempty"`
	GroupBuckets                *int32  `json:"groupBuckets,omitempty"`
	GroupFirstKey               *string `json:"groupFirstKey,omitempty"`
	LastValue                   *bool   `json:"lastValue,omitempty"`
	LastValueKey                *string `json:"lastValueKey,omitempty"`
	NonDestructive              *bool   `json:"nonDestructive,omitempty"`
	PurgeOnNoConsumers          *bool   `json:"purgeOnNoConsumers"`
	Enabled                     *bool   `json:"enabled,omitempty"`
	ConsumersBeforeDispatch     *int32  `json:"consumersBeforeDispatch,omitempty"`
	DelayBeforeDispatch         *int64  `json:"delayBeforeDispatch,omitempty"`
	ConsumerPriority            *int32  `json:"consumerPriority,omitempty"`
	AutoDelete                  *bool   `json:"autoDelete,omitempty"`
	AutoDeleteDelay             *int64  `json:"autoDeleteDelay,omitempty"`
	AutoDeleteMessageCount      *int64  `json:"autoDeleteMessageCount,omitempty"`
	RingSize                    *int64  `json:"ringSize,omitempty"`
	ConfigurationManaged        *bool   `json:"configurationManaged,omitempty"`
	Temporary                   *bool   `json:"temporary,omitempty"`
	AutoCreateAddress           *bool   `json:"autoCreateAddress,omitempty"`
}

// ActiveMQArtemisAddressStatus defines the observed state of ActiveMQArtemisAddress
type ActiveMQArtemisAddressStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisAddress is the Schema for the activemqartemisaddresses API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=activemqartemisaddresses,scope=Namespaced
type ActiveMQArtemisAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisAddressSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisAddressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisAddressList contains a list of ActiveMQArtemisAddress
type ActiveMQArtemisAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisAddress{}, &ActiveMQArtemisAddressList{})
}
