package persistentvolumeclaims

import (
	"github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewPersistentVolumeClaim(namespace string, spec *v1beta1.VolumeClaimTemplate) *corev1.PersistentVolumeClaim {

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        spec.Name,
			Namespace:   namespace,
			Annotations: spec.Annotations,
			Labels:      spec.Labels,
		},
		Spec: spec.Spec,
	}

	return pvc
}

func NewPersistentVolumeClaimWithCapacityAndStorageClassName(namespacedName types.NamespacedName, capacity string, labels map[string]string, storageClassName string, accessModes []corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      labels,
			Name:        namespacedName.Name,
			Namespace:   namespacedName.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(capacity),
				},
			},
		},
	}

	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}

	return pvc
}
