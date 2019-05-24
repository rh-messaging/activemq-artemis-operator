package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisScaledownSpec defines the desired state of ActiveMQArtemisScaledown
// +k8s:openapi-gen=true
type ActiveMQArtemisScaledownSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	MasterURL  string `json:"masterURL"`
	Kubeconfig string `json:"kubeconfig"`
	Namespace  string `json:"namespace"`
	LocalOnly  bool   `json:"localOnly"`
}

// ActiveMQArtemisScaledownStatus defines the observed state of ActiveMQArtemisScaledown
// +k8s:openapi-gen=true
type ActiveMQArtemisScaledownStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisScaledown is the Schema for the activemqartemisscaledowns API
// +k8s:openapi-gen=true
type ActiveMQArtemisScaledown struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisScaledownSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisScaledownStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisScaledownList contains a list of ActiveMQArtemisScaledown
type ActiveMQArtemisScaledownList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisScaledown `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisScaledown{}, &ActiveMQArtemisScaledownList{})
}
