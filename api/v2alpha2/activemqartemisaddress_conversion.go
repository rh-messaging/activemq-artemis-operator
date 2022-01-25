package v2alpha2

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var loga = logf.Log.WithName("v2alpha2conversion")

func (r *ActiveMQArtemisAddress) ConvertTo(dst conversion.Hub) error {
	return nil
}

func (r *ActiveMQArtemisAddress) ConvertFrom(src conversion.Hub) error {
	return nil
}
