package v2alpha5

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("v2alpha5Conversion")

func (r *ActiveMQArtemis) ConvertTo(dst conversion.Hub) error {
	return nil
}

//may not need it if the Hub (storage version) is the latest
func (r *ActiveMQArtemis) ConvertFrom(src conversion.Hub) error {
	return nil
}
