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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisScaledownSpec defines the desired state of ActiveMQArtemisScaledown
type ActiveMQArtemisScaledownSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Triggered by main ActiveMQArtemis CRD messageMigration entry
	LocalOnly bool `json:"localOnly"`
}

// ActiveMQArtemisScaledownStatus defines the observed state of ActiveMQArtemisScaledown
type ActiveMQArtemisScaledownStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ActiveMQArtemisScaledown is the Schema for the activemqartemisscaledowns API
type ActiveMQArtemisScaledown struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisScaledownSpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisScaledownStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ActiveMQArtemisScaledownList contains a list of ActiveMQArtemisScaledown
type ActiveMQArtemisScaledownList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisScaledown `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisScaledown{}, &ActiveMQArtemisScaledownList{})
}
