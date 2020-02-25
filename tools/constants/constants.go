package constants

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RedHatImageRegistry = "registry.redhat.io"
	QuayURLBase         = "https://quay.io/api/v1/repository/"

	BrokerVar         = "BROKER_IMAGE_"
	Broker75Image     = "amq-broker"
	Broker75ImageTag  = "7.5"
	Broker75ImageURL  = RedHatImageRegistry + "/amq7/" + Broker75Image + ":" + Broker75ImageTag
	Broker75Component = "amq-broker-openshift-container"
)

type ImageEnv struct {
	Var       string
	Component string
	Registry  string
}
type ImageRef struct {
	metav1.TypeMeta `json:",inline"`
	Spec            ImageRefSpec `json:"spec"`
}
type ImageRefSpec struct {
	Tags []ImageRefTag `json:"tags"`
}
type ImageRefTag struct {
	Name string                  `json:"name"`
	From *corev1.ObjectReference `json:"from"`
}
