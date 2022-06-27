package volumes

import (
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime"
)

var log = logf.Log.WithName("package volumes")

//func MakeVolumeMounts(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {
//
//	volumeMounts := []corev1.VolumeMount{}
//	if cr.Spec.DeploymentPlan.PersistenceEnabled {
//		persistentCRVlMnt := makePersistentVolumeMount(cr.Name)
//		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
//	}
//
//	// Scan acceptors for any with sslEnabled
//	for _, acceptor := range cr.Spec.Acceptors {
//		if !acceptor.SSLEnabled {
//			continue
//		}
//		volumeMountName := cr.Name + "-" + acceptor.Name + "-secret-volume"
//		if "" != acceptor.SSLSecret {
//			volumeMountName = acceptor.SSLSecret + "-volume"
//		}
//		volumeMount := makeVolumeMount(volumeMountName)
//		volumeMounts = append(volumeMounts, volumeMount)
//	}
//
//	// Scan connectors for any with sslEnabled
//	for _, connector := range cr.Spec.Connectors {
//		if !connector.SSLEnabled {
//			continue
//		}
//		volumeMountName := cr.Name + "-" + connector.Name + "-secret-volume"
//		if "" != connector.SSLSecret {
//			volumeMountName = connector.SSLSecret + "-volume"
//		}
//		volumeMount := makeVolumeMount(volumeMountName)
//		volumeMounts = append(volumeMounts, volumeMount)
//	}
//
//	if cr.Spec.Console.SSLEnabled {
//		volumeMountName := secrets.ConsoleNameBuilder.Name() + "-volume"
//		if "" != cr.Spec.Console.SSLSecret {
//			volumeMountName = cr.Spec.Console.SSLSecret + "-volume"
//		}
//		volumeMount := makeVolumeMount(volumeMountName)
//		volumeMounts = append(volumeMounts, volumeMount)
//	}
//
//	return volumeMounts
//}

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

//func MakeVolumes(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {
//
//	volumes := []corev1.Volume{}
//	if cr.Spec.DeploymentPlan.PersistenceEnabled {
//		basicCRVolume := makePersistentVolume(cr.Name)
//		volumes = append(volumes, basicCRVolume...)
//	}
//
//	// Scan acceptors for any with sslEnabled
//	for _, acceptor := range cr.Spec.Acceptors {
//		if !acceptor.SSLEnabled {
//			continue
//		}
//		secretName := cr.Name + "-" + acceptor.Name + "-secret"
//		if "" != acceptor.SSLSecret {
//			secretName = acceptor.SSLSecret
//		}
//		volume := makeVolume(secretName)
//		volumes = append(volumes, volume)
//	}
//
//	// Scan connectors for any with sslEnabled
//	for _, connector := range cr.Spec.Connectors {
//		if !connector.SSLEnabled {
//			continue
//		}
//		secretName := cr.Name + "-" + connector.Name + "-secret"
//		if "" != connector.SSLSecret {
//			secretName = connector.SSLSecret
//		}
//		volume := makeVolume(secretName)
//		volumes = append(volumes, volume)
//	}
//
//	if cr.Spec.Console.SSLEnabled {
//		secretName := secrets.ConsoleNameBuilder.Name()
//		if "" != cr.Spec.Console.SSLSecret {
//			secretName = cr.Spec.Console.SSLSecret
//		}
//		volume := makeVolume(secretName)
//		volumes = append(volumes, volume)
//	}
//
//	return volumes
//}

func MakeVolume(secretName string) corev1.Volume {

	volumeName := secretName + "-volume"
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	return volume
}

//func makePersistentVolume(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {
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

//func makePersistentVolumeMount(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {
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
	volume := corev1.Volume{
		Name: "secret-" + secretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	return volume
}
