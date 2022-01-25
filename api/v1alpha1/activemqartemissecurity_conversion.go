package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("v1alpha1conversion")

func (r *ActiveMQArtemisSecurity) ConvertTo(dst conversion.Hub) error {
	return nil
}

func (r *ActiveMQArtemisSecurity) ConvertFrom(src conversion.Hub) error {
	return nil
}
