package volumes

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package volumes")


func MakeVolumeMounts(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		persistentCRVlMnt := makePersistentVolumeMount(cr)
		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
	}
	if environments.CheckSSLEnabled(cr) {
		sslCRVlMnt := makeSSLVolumeMount(cr)
		volumeMounts = append(volumeMounts, sslCRVlMnt...)
	}
	return volumeMounts

}

func MakeVolumes(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {

	volume := []corev1.Volume{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		basicCRVolume := makePersistentVolume(cr)
		volume = append(volume, basicCRVolume...)
	}
	if environments.CheckSSLEnabled(cr) {
		sslCRVolume := makeSSLSecretVolume(cr)
		volume = append(volume, sslCRVolume...)
	}
	return volume
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

func makeSSLSecretVolume(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {

	volume := []corev1.Volume{
		{
			Name: "broker-secret-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "TODO-FIX-REPLACE",//cr.Spec.SSLConfig.SecretName,
				},
			},
		},
	}

	return volume
}

func makePersistentVolumeMount(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

	dataPath := environments.GetPropertyForCR("AMQ_DATA_DIR", cr, "/opt/"+cr.Name+"/data")
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      cr.Name,
			MountPath: dataPath,
			ReadOnly:  false,
		},
	}
	return volumeMounts
}

func makeSSLVolumeMount(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "broker-secret-volume",
			MountPath: "/etc/amq-secret-volume",
			ReadOnly:  true,
		},
	}
	return volumeMounts
}
