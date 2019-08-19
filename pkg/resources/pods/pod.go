package pods

import (
	"github.com/RHsyseng/operator-utils/pkg/olm"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/volumes"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"context"
	appsv1 "k8s.io/api/apps/v1"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package pods")
var NameBuilder namer.NamerData

const (
	graceTime       = 30
	TCPLivenessPort = 8161
)

func NewPodTemplateSpecForCR(cr *brokerv2alpha1.ActiveMQArtemis) corev1.PodTemplateSpec {

	// Log where we are and what we're doing
	reqLogger := log.WithName(cr.Name)
	reqLogger.Info("Creating new pod template spec for custom resource")

	terminationGracePeriodSeconds := int64(60)
	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    cr.Labels,
		},
	}
	Spec := corev1.PodSpec{}
	Containers := []corev1.Container{}
	container := corev1.Container{
		Name:    cr.Name + "-container",
		Image:   cr.Spec.DeploymentPlan.Image,
		Command: []string{"/opt/amq/bin/launch.sh", "start"},
		Env:     environments.MakeEnvVarArrayForCR(cr),
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
	volumeMounts := volumes.MakeVolumeMounts(cr)
	if len(volumeMounts) > 0 {
		container.VolumeMounts = volumeMounts
	}
	Spec.Containers = append(Containers, container)
	volumes := volumes.MakeVolumes(cr)
	if len(volumes) > 0 {
		Spec.Volumes = volumes
	}
	Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
	pts.Spec = Spec

	return pts
}

// TODO: Test namespacedName to ensure it's the right namespacedName
func UpdatePodStatus(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, namespacedName types.NamespacedName) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Updating pods status")

	podStatus := getPodStatus(cr, client, namespacedName)

	reqLogger.Info("PodStatus are to be updated.............................", "info:", podStatus)
	reqLogger.Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.Info("Stopped Count........................", "info:", len(podStatus.Stopped))
	reqLogger.Info("Starting Count........................", "info:", len(podStatus.Starting))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		cr.Status.PodStatus = podStatus

		err := client.Status().Update(context.TODO(), cr)
		if err != nil {
			reqLogger.Error(err, "Failed to update pods status")
			return err
		}
		reqLogger.Info("Pods status updated")
		return nil
	}

	return nil
}

func getPodStatus(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {
	// List the pods for this deployment
	var status olm.DeploymentStatus
	sfsFound := &appsv1.StatefulSet{}

	err := client.Get(context.TODO(), namespacedName, sfsFound)
	if err == nil {
		status = olm.GetSingleStatefulSetStatus(*sfsFound)
	} else {
		dsFound := &appsv1.DaemonSet{}
		err = client.Get(context.TODO(), namespacedName, dsFound)
		if err == nil {
			status = olm.GetSingleDaemonSetStatus(*dsFound)
		}
	}

	return status
}
