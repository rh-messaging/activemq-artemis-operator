package volumes

import (
	corev1 "k8s.io/api/core/v1"
)

func MakeVolumeMount(volumeMountName string) corev1.VolumeMount {

	volumeMountMountPath := "/etc/" + volumeMountName
	volumeMount := corev1.VolumeMount{
		Name:             volumeMountName,
		ReadOnly:         true,
		MountPath:        volumeMountMountPath,
		SubPath:          "",
		MountPropagation: nil,
	}

	return volumeMount
}

func MakeVolume(secretName string) corev1.Volume {

	volumeName := secretName + "-volume"
	// 420(decimal) = 0644(octal)
	var defaultMode int32 = 420
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: &defaultMode,
			},
		},
	}
	return volume
}

// func makePersistentVolume(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {
func MakePersistentVolume(customResourceName string) []corev1.Volume {

	volume := []corev1.Volume{
		{
			Name: customResourceName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: customResourceName,
					ReadOnly:  false,
				},
			},
		},
	}

	return volume
}

func MakePersistentVolumeMount(customResourceName string, mountPath string) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      customResourceName,
			MountPath: mountPath,
			ReadOnly:  false,
		},
	}
	return volumeMounts
}

func MakeVolumeForCfg(name string) corev1.Volume {
	volume := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	return volume
}

func MakeRwVolumeMountForCfg(name string, path string) corev1.VolumeMount {
	return MakeVolumeMountForCfg(name, path, false)
}

func MakeVolumeMountForCfg(name string, path string, readOnly bool) corev1.VolumeMount {

	volumeMount := corev1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	}
	return volumeMount
}

func MakeVolumeForConfigMap(cfgmapName string) corev1.Volume {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	volume := corev1.Volume{
		Name: "configmap-" + cfgmapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cfgmapName},
				Items:                []corev1.KeyToPath{},
				DefaultMode:          &defaultMode,
				Optional:             nil,
			},
		},
	}
	return volume
}

func MakeVolumeForSecret(secretName string) corev1.Volume {
	defaultMode := corev1.SecretVolumeSourceDefaultMode
	volume := corev1.Volume{
		Name: "secret-" + secretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				Items:       []corev1.KeyToPath{},
				DefaultMode: &defaultMode,
			},
		},
	}
	return volume
}

func GetDefaultMountPath(volume *corev1.Volume) string {
	return "/amq/extra/volumes/" + volume.Name
}

func MakeVolumeMountForVolume(vol *corev1.Volume) *corev1.VolumeMount {
	volumeMount := corev1.VolumeMount{
		Name:      vol.Name,
		MountPath: GetDefaultMountPath(vol),
	}
	return &volumeMount
}

func GetDefaultExternalPVCMountPath(nameBase string) *string {
	var path = "/opt/" + nameBase + "/data"
	return &path
}

func NewVolumeMountForPVC(name string) *corev1.VolumeMount {
	vMount := &corev1.VolumeMount{
		Name:      name,
		MountPath: *GetDefaultExternalPVCMountPath(name),
	}
	return vMount
}
