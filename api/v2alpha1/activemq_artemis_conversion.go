package v2alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("v2alpha1Conversion")

func (r *ActiveMQArtemis) ConvertTo(dst conversion.Hub) error {
	return nil
}

func (r *ActiveMQArtemis) ConvertFrom(src conversion.Hub) error {
	return nil
}
