// NOTE: Boilerplate only.  Ignore this file.

// Package v2alpha5 contains API Schema definitions for the broker v2alpha5 API group
// +k8s:deepcopy-gen=package,register
// +groupName=broker.amq.io
package v2alpha5

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: "broker.amq.io", Version: "v2alpha5"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
