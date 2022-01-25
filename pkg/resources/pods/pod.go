package pods

import (
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime"
)

var log = logf.Log.WithName("package pods")
var NameBuilder namer.NamerData

//func NewPodTemplateSpecForCR(namespacedName types.NamespacedName, envVarArray []corev1.EnvVar) corev1.PodTemplateSpec {
//
//	// Log where we are and what we're doing
//	reqLogger := log.WithName(namespacedName.Name)
//	reqLogger.Info("Creating new pod template spec for custom resource")
//
//	terminationGracePeriodSeconds := int64(60)
//
//	pts := makePodTemplateSpec(namespacedName, selectors.LabelBuilder.Labels())
//	Spec := corev1.PodSpec{}
//	Containers := []corev1.Container{}
//
//	volumeMounts := volumes.MakeVolumeMounts(cr)
//	if len(volumeMounts) > 0 {
//		container.VolumeMounts = volumeMounts
//	}
//	Spec.Containers = append(Containers, container)
//	volumes := volumes.MakeVolumes(cr)
//	if len(volumes) > 0 {
//		Spec.Volumes = volumes
//	}
//	Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
//	pts.Spec = Spec
//
//	return pts
//}

func MakePodTemplateSpec(namespacedName types.NamespacedName, labels map[string]string) corev1.PodTemplateSpec {

	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels, //selectors.LabelBuilder.Labels(),
		},
	}
	return pts
}
