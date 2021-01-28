package containers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	graceTime       = 30
	TCPLivenessPort = 8161
)

//func MakeContainer(cr *brokerv2alpha1.ActiveMQArtemis) corev1.Container {
func MakeContainer(customResourceName string, imageName string, envVarArray []corev1.EnvVar) corev1.Container {

	container := corev1.Container{
		Name:    customResourceName + "-container",
		Image:   imageName, //cr.Spec.DeploymentPlan.Image,
		Command: []string{"/opt/amq/bin/launch.sh", "start"},
		Env:     envVarArray, //environments.MakeEnvVarArrayForCR(cr),
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: graceTime,
			TimeoutSeconds:      5,
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/bash",
						"-c",
						"/opt/amq/bin/readinessProbe.sh",
					},
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: graceTime,
			TimeoutSeconds:      5,
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(TCPLivenessPort),
				},
			},
		},
	}

	return container
}

func MakeInitContainer(containerName string, imageName string, envVarArray []corev1.EnvVar) corev1.Container {

	container := corev1.Container{
		Name:  containerName,
		Image: imageName,
		Env:   envVarArray,
	}

	return container
}
