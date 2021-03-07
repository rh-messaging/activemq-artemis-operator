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

	Broker76Image     = "amq-broker"
	Broker76ImageTag  = "7.6"
	Broker76ImageURL  = RedHatImageRegistry + "/amq7/" + Broker76Image + ":" + Broker76ImageTag
	Broker76Component = "amq-broker-openshift-container"

	Broker77Image     = "amq-broker"
	Broker77ImageTag  = "7.7"
	Broker77ImageURL  = RedHatImageRegistry + "/amq7/" + Broker77Image + ":" + Broker77ImageTag
	Broker77Component = "amq-broker-openshift-container"

	Broker78Image     = "amq-broker"
	Broker78ImageTag  = "7.8"
	Broker78ImageURL  = RedHatImageRegistry + "/amq7/" + Broker78Image + ":" + Broker78ImageTag
	Broker78Component = "amq-broker-openshift-container"
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
