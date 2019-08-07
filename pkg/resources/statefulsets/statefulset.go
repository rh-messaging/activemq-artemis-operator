package statefulsets

import (
	"context"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	pvc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/persistentvolumeclaims"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/pods"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package statefulsets")
var NameBuilder namer.NamerData

//const (
//	graceTime       = 30
//	TCPLivenessPort = 8161
//)

//func newPodTemplateSpecForCR(cr *brokerv2alpha1.ActiveMQArtemis) corev1.PodTemplateSpec {
//
//	// Log where we are and what we're doing
//	reqLogger := log.WithName(cr.Name)
//	reqLogger.Info("Creating new pod template spec for custom resource")
//
//	//var pts corev1.PodTemplateSpec
//	terminationGracePeriodSeconds := int64(60)
//	pts := corev1.PodTemplateSpec{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      cr.Name,
//			Namespace: cr.Namespace,
//			Labels:    cr.Labels,
//		},
//	}
//	Spec := corev1.PodSpec{}
//	Containers := []corev1.Container{}
//	container := corev1.Container{
//		Name:    cr.Name + "-container",
//		Image:   cr.Spec.DeploymentPlan.Image,
//		Command: []string{"/opt/amq/bin/launch.sh", "start"},
//		Env:     environments.MakeEnvVarArrayForCR(cr),
//		ReadinessProbe: &corev1.Probe{
//			InitialDelaySeconds: graceTime,
//			TimeoutSeconds:      5,
//			Handler: corev1.Handler{
//				Exec: &corev1.ExecAction{
//					Command: []string{
//						"/bin/bash",
//						"-c",
//						"/opt/amq/bin/readinessProbe.sh",
//					},
//				},
//			},
//		},
//		LivenessProbe: &corev1.Probe{
//			InitialDelaySeconds: graceTime,
//			TimeoutSeconds:      5,
//			Handler: corev1.Handler{
//				TCPSocket: &corev1.TCPSocketAction{
//					Port: intstr.FromInt(TCPLivenessPort),
//				},
//			},
//		},
//	}
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

func NewStatefulSetForCR(cr *brokerv2alpha1.ActiveMQArtemis) *appsv1.StatefulSet {

	// Log where we are and what we're doing
	reqLogger := log.WithName(cr.Name)
	reqLogger.Info("Creating new statefulset for custom resource")
	replicas := cr.Spec.DeploymentPlan.Size

	labels := selectors.LabelBuilder.Labels()

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        NameBuilder.Name(),
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: cr.Annotations,
		},
	}
	Spec := appsv1.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: svc.HeadlessNameBuilder.Name(),
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: pods.NewPodTemplateSpecForCR(cr),
	}

	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		Spec.VolumeClaimTemplates = *pvc.NewPersistentVolumeClaimArrayForCR(cr, 1)
	}
	ss.Spec = Spec

	return ss
}

var GLOBAL_CRNAME string = ""

func Create(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, ss *appsv1.StatefulSet) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new statefulset")
	var err error = nil

	// Set ActiveMQArtemis instance as the owner and controller
	if err = controllerutil.SetControllerReference(cr, ss, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "statefulset")
	}
	reqLogger.Info("Set controller reference for new " + "statefulset")

	// Call k8s create for statefulset
	if err = client.Create(context.TODO(), ss); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "statefulset")
	}
	reqLogger.Info("Created new " + "statefulset")

	//TODO: Remove this blatant hack
	GLOBAL_CRNAME = cr.Name

	return err
}
func RetrieveStatefulSet(statefulsetName string, namespacedName types.NamespacedName, client client.Client) (*appsv1.StatefulSet, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.Info("Retrieving " + "statefulset")

	var err error = nil

	//// TODO: Remove this hack
	//var crName string = statefulsetName

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        statefulsetName,
			Namespace:   namespacedName.Namespace,
			Labels:      selectors.LabelBuilder.Labels(),
			Annotations: nil,
		},
	}
	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, ss); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Statefulset claim IsNotFound", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		} else {
			reqLogger.Info("Statefulset claim found", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		}
	}

	return ss, err
}

func Retrieve(namespacedName types.NamespacedName, client client.Client, statefulset *appsv1.StatefulSet) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.Info("Retrieving " + "statefulset " + statefulset.Name)

	// Check if the headless service already exists
	var err error = nil
	if err = client.Get(context.TODO(), namespacedName, statefulset); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Statefulset claim IsNotFound", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		} else {
			reqLogger.Info("Statefulset claim found", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		}
	}

	return err
}
