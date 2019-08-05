package apis

import (
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v2alpha1.SchemeBuilder.AddToScheme)
}
