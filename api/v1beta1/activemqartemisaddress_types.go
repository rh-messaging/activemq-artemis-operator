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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisAddressSpec defines the desired state of ActiveMQArtemisAddress
type ActiveMQArtemisAddressSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Address Name
	AddressName string `json:"addressName,omitempty"`
	// Queue Name
	QueueName *string `json:"queueName,omitempty"`
	// The Routing Type
	RoutingType *string `json:"routingType,omitempty"`
	// Whether or not delete the queue from broker when CR is undeployed(default false)
	RemoveFromBrokerOnDelete bool `json:"removeFromBrokerOnDelete,omitempty"`
	// User name for creating the queue or address
	User *string `json:"user,omitempty"`
	// The user's password
	Password           *string                 `json:"password,omitempty"`
	QueueConfiguration *QueueConfigurationType `json:"queueConfiguration,omitempty"`
	// Apply to the broker crs in the current namespace. A value of * or empty string means applying to all broker crs. Default apply to all broker crs
	ApplyToCrNames []string `json:"applyToCrNames,omitempty"`
}

type QueueConfigurationType struct {
	// If ignore if the target queue already exists
	IgnoreIfExists *bool `json:"ignoreIfExists,omitempty"`
	// The routing type of the queue
	RoutingType *string `json:"routingType,omitempty"`
	// The filter string for the queue
	FilterString *string `json:"filterString,omitempty"`
	// If the queue is durable or not
	Durable *bool `json:"durable,omitempty"`
	// The user associated with the queue
	User *string `json:"user,omitempty"`
	// Max number of consumers allowed on this queue
	MaxConsumers *int32 `json:"maxConsumers,omitempty"`
	// If the queue is exclusive
	Exclusive *bool `json:"exclusive,omitempty"`
	// If rebalance the message group
	GroupRebalance *bool `json:"groupRebalance,omitempty"`
	// If pause message dispatch when rebalancing groups
	GroupRebalancePauseDispatch *bool `json:"groupRebalancePauseDispatch,omitempty"`
	// Number of messaging group buckets
	GroupBuckets *int32 `json:"groupBuckets,omitempty"`
	// Header set on the first group message
	GroupFirstKey *string `json:"groupFirstKey,omitempty"`
	// If it is a last value queue
	LastValue *bool `json:"lastValue,omitempty"`
	// The property used for last value queue to identify last values
	LastValueKey *string `json:"lastValueKey,omitempty"`
	// If force non-destructive consumers on the queue
	NonDestructive *bool `json:"nonDestructive,omitempty"`
	// Whether to delete all messages when no consumers connected to the queue
	PurgeOnNoConsumers *bool `json:"purgeOnNoConsumers,omitempty"`
	// If the queue is enabled
	Enabled *bool `json:"enabled,omitempty"`
	// Number of consumers required before dispatching messages
	ConsumersBeforeDispatch *int32 `json:"consumersBeforeDispatch,omitempty"`
	// Milliseconds to wait for `consumers-before-dispatch` to be met before dispatching messages anyway
	DelayBeforeDispatch *int64 `json:"delayBeforeDispatch,omitempty"`
	// Consumer Priority
	ConsumerPriority *int32 `json:"consumerPriority,omitempty"`
	// Auto-delete the queue
	AutoDelete *bool `json:"autoDelete,omitempty"`
	// Delay (Milliseconds) before auto-delete the queue
	AutoDeleteDelay *int64 `json:"autoDeleteDelay,omitempty"`
	// Message count of the queue to allow auto delete
	AutoDeleteMessageCount *int64 `json:"autoDeleteMessageCount,omitempty"`
	// The size the queue should maintain according to ring semantics
	RingSize *int64 `json:"ringSize,omitempty"`
	//  If the queue is configuration managed
	ConfigurationManaged *bool `json:"configurationManaged,omitempty"`
	// If the queue is temporary
	Temporary *bool `json:"temporary,omitempty"`
	// Whether auto create address
	AutoCreateAddress *bool `json:"autoCreateAddress,omitempty"`
}

// ActiveMQArtemisAddressStatus defines the observed state of ActiveMQArtemisAddress
type ActiveMQArtemisAddressStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// ActiveMQArtemisAddress is the Schema for the activemqartemisaddresses API
type ActiveMQArtemisAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisAddressSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisAddressStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ActiveMQArtemisAddressList contains a list of ActiveMQArtemisAddress
type ActiveMQArtemisAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisAddress{}, &ActiveMQArtemisAddressList{})
}

func (r *ActiveMQArtemisAddress) Hub() {
}
