package volumes

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package volumes")

// TODO: Remove this ugly hack
var GLOBAL_DATA_PATH string

func MakeVolumeMounts(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		persistentCRVlMnt := makePersistentVolumeMount(cr)
		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range cr.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		volumeMountName := cr.Name + "-" + acceptor.Name + "-secret-volume"
		if "" != acceptor.SSLSecret {
			volumeMountName = acceptor.SSLSecret + "-volume"
		}
		volumeMountMountPath := "/etc/" + volumeMountName
		volumeMount := corev1.VolumeMount{
			Name: 			  volumeMountName,
			ReadOnly:         true,
			MountPath:        volumeMountMountPath,
			SubPath:          "",
			MountPropagation: nil,
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range cr.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		volumeMountName := cr.Name + "-" + connector.Name + "-secret-volume"
		if "" != connector.SSLSecret {
			volumeMountName = connector.SSLSecret + "-volume"
		}
		volumeMountMountPath := "/etc/" + volumeMountName
		volumeMount := corev1.VolumeMount{
			Name: 			  volumeMountName,
			ReadOnly:         true,
			MountPath:        volumeMountMountPath,
			SubPath:          "",
			MountPropagation: nil,
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

func MakeVolumes(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {

	volumes := []corev1.Volume{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		basicCRVolume := makePersistentVolume(cr)
		volumes = append(volumes, basicCRVolume...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range cr.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		secretName := cr.Name + "-" + acceptor.Name + "-secret"
		if "" != acceptor.SSLSecret {
			secretName = acceptor.SSLSecret
		}
		volumeName := secretName + "-volume"
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumes = append(volumes, volume)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range cr.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		secretName := cr.Name + "-" + connector.Name + "-secret"
		if "" != connector.SSLSecret {
			secretName = connector.SSLSecret
		}
		volumeName := secretName + "-volume"
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumes = append(volumes, volume)
	}

	return volumes
}

func makePersistentVolume(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {

	volume := []corev1.Volume{
		{
			Name: cr.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cr.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	return volume
}

func makePersistentVolumeMount(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

	// TODO: Ensure consistent path usage
	GLOBAL_DATA_PATH := environments.GetPropertyForCR("AMQ_DATA_DIR", cr, "/opt/"+cr.Name+"/data")
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      cr.Name,
			MountPath: GLOBAL_DATA_PATH,
			ReadOnly:  false,
		},
	}
	return volumeMounts
}
