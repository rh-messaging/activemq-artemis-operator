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

package v2alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisAddressSpec defines the desired state of ActiveMQArtemisAddress
type ActiveMQArtemisAddressSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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
	// Important: Run "make" to regenerate code after modifying this file

	// Current state of the resource
	// Conditions represent the latest available observations of an object's state
	//+optional
	//+patchMergeKey=type
	//+patchStrategy=merge
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions",xDescriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=activemqartemisaddresses,shortName=aaa
//+operator-sdk:csv:customresourcedefinitions:resources={{"Secret", "v1"}}

// +kubebuilder:deprecatedversion:warning="The ActiveMQArtemisAddress CRD is deprecated. Use the spec.brokerProperties attribute in the ActiveMQArtemis CR to create addresses and queues instead"
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
