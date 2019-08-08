package v2alpha1

import (
	"github.com/RHsyseng/operator-utils/pkg/olm"
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

	DeploymentPlan DeploymentPlanType `json:"deploymentPlan,omitempty"`
	Acceptors      []AcceptorType     `json:"acceptors,omitempty"`
	Connectors     []ConnectorType    `json:"connectors,omitempty"`
}

type DeploymentPlanType struct {
	Image              string `json:"image,omitempty"`
	Size               int32  `json:"size,omitempty"`
	User               string `json:"user,omitempty"`
	Password           string `json:"password,omitempty"`
	Role               string `json:"role,omitempty"`
	RequireLogin       bool   `json:"requireLogin,omitempty"`
	Clustered          bool   `json:"clustered,omitempty"`
	ClusterUser        string `json:"clusterUser,omitempty"`
	ClusterPassword    string `json:"clusterPassword,omitempty"`
	PersistenceEnabled bool   `json:"persistenceEnabled,omitempty"`
	JournalType        string `json:"journalType,omitempty"`
	MessageMigration   bool   `json:"messageMigration,omitempty"`
}

type AcceptorType struct {
	Name       string `json:"name"`
	Port       int32  `json:"port,omitempty"`
	Protocols  string `json:"protocols,omitempty"`
	SSLEnabled bool   `json:"sslEnabled,omitempty"`
	SSLSecret  string `json:"sslSecret,omitempty"`
	Expose     bool   `json:"expose,omitempty"`
}

type ConnectorType struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Host       string `json:"host"`
	Port       int32  `json:"port"`
	SSLEnabled bool   `json:"sslEnabled,omitempty"`
	SSLSecret  string `json:"sslSecret,omitempty"`
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
