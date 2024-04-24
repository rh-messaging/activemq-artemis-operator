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

package v2alpha1

import (
	"github.com/RHsyseng/operator-utils/pkg/olm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisSpec defines the desired state of ActiveMQArtemis
type ActiveMQArtemisSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	AdminUser      string             `json:"adminUser,omitempty"`
	AdminPassword  string             `json:"adminPassword,omitempty"`
	DeploymentPlan DeploymentPlanType `json:"deploymentPlan,omitempty"`
	Acceptors      []AcceptorType     `json:"acceptors,omitempty"`
	Connectors     []ConnectorType    `json:"connectors,omitempty"`
	Console        ConsoleType        `json:"console,omitempty"`
}

type DeploymentPlanType struct {
	Image              string `json:"image,omitempty"`
	Size               int32  `json:"size,omitempty"`
	RequireLogin       bool   `json:"requireLogin,omitempty"`
	PersistenceEnabled bool   `json:"persistenceEnabled,omitempty"`
	JournalType        string `json:"journalType,omitempty"`
	MessageMigration   *bool  `json:"messageMigration,omitempty"`
}

type AcceptorType struct {
	Name                string `json:"name"`
	Port                int32  `json:"port,omitempty"`
	Protocols           string `json:"protocols,omitempty"`
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
	AnycastPrefix       string `json:"anycastPrefix,omitempty"`
	MulticastPrefix     string `json:"multicastPrefix,omitempty"`
	ConnectionsAllowed  int    `json:"connectionsAllowed,omitempty"`
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

// ActiveMQArtemisStatus defines the observed state of ActiveMQArtemis
type ActiveMQArtemisStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	PodStatus olm.DeploymentStatus `json:"podStatus"`

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
//+kubebuilder:resource:path=activemqartemises
//+kubebuilder:resource:path=activemqartemises,shortName=aa
//+operator-sdk:csv:customresourcedefinitions:resources={{"Secret", "v1"}}

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
