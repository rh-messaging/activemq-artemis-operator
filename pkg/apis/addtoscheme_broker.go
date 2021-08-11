package apis

import (
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha4"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha5"
	routev1 "github.com/openshift/api/route/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v2alpha1.SchemeBuilder.AddToScheme, routev1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, v2alpha2.SchemeBuilder.AddToScheme, routev1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, v2alpha3.SchemeBuilder.AddToScheme, routev1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, v2alpha4.SchemeBuilder.AddToScheme, routev1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, v2alpha5.SchemeBuilder.AddToScheme, routev1.SchemeBuilder.AddToScheme)
}
