package v1alpha1

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

	Image         string        `json:"image"`
	Size          int32         `json:"size,omitempty"`
	Persistent    bool          `json:"persistent"`
	Aio           bool          `json:"aio"`
	ClusterConfig ClusterConfig `json:"clusterConfig,omitempty"`
	SSLConfig     SSLConfig     `json:"sslConfig,omitempty"`
	CommonConfig  CommonConfig  `json:"commonConfig,omitempty"`
}

type CommonConfig struct {
	UserName string `json:"userName,omitempty"`
	Password string `json:"password,omitempty"`
}

// ActiveMQArtemisStatus defines the observed state of ActiveMQArtemis
// +k8s:openapi-gen=true
type ActiveMQArtemisStatus struct {
	PodStatus olm.DeploymentStatus `json:"pods"`
}

type SSLConfig struct {
	SecretName         string `json:"secretName,omitempty"`
	TrustStoreFilename string `json:"trustStoreFilename,omitempty"`
	TrustStorePassword string `json:"trustStorePassword,omitempty"`
	KeystoreFilename   string `json:"keystoreFilename,omitempty"`
	KeyStorePassword   string `json:"keyStorePassword,omitempty"`
}

type ClusterConfig struct {
	ClusterUserName string `json:"clusterUserName,omitempty"`
	ClusterPassword string `json:"clusterPassword,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemis is the Schema for the activemqartemises API
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
