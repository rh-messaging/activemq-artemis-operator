package containers

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	graceTime       = 30
	TCPLivenessPort = 8161
)

func MakeContainer(hostingPodSpec *corev1.PodSpec, customResourceName string, imageName string, envVarArray []corev1.EnvVar) *corev1.Container {

	name := customResourceName + "-container"

	var container *corev1.Container

	if hostingPodSpec != nil {
		for _, existingContainer := range hostingPodSpec.Containers {
			if existingContainer.Name == name {
				container = &existingContainer
				break
			}
		}
	}
	if container == nil {
		container = &corev1.Container{
			Name:    name,
			Command: []string{"/bin/bash", "-c", "export STATEFUL_SET_ORDINAL=${HOSTNAME##*-}; export JDK_JAVA_OPTIONS=${JDK_JAVA_OPTIONS//\\$\\{STATEFUL_SET_ORDINAL\\}/${HOSTNAME##*-}}; export FQ_HOST_NAME=$(hostname -f); export JAVA_ARGS_APPEND=$( echo ${JAVA_ARGS_APPEND} | sed \"s/FQ_HOST_NAME/${FQ_HOST_NAME}/\"); exec /opt/amq/bin/launch.sh", "start"},
		}
	}

	container.Image = imageName
	container.Env = envVarArray

	return container
}

func MakeInitContainer(hostingPodSpec *corev1.PodSpec, customResourceName string, imageName string, envVarArray []corev1.EnvVar) *corev1.Container {

	name := customResourceName + "-container-init"

	var container *corev1.Container

	if hostingPodSpec != nil {
		for _, existingContainer := range hostingPodSpec.InitContainers {
			if existingContainer.Name == name {
				container = &existingContainer
				break
			}
		}
	}
	if container == nil {
		container = &corev1.Container{
			Name:    name,
			Command: []string{"/bin/bash"},
		}
	}

	container.Image = imageName
	container.Env = envVarArray

	return container
}
