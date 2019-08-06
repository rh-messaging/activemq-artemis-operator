package statefulsets

import (
	"context"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	pvc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/persistentvolumeclaims"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/namer"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package statefulsets")
var NameBuilder namer.NamerData

const (
	graceTime       = 30
	TCPLivenessPort = 8161
)






func makeVolumeMounts(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.VolumeMount {

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

func makeVolumes(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.Volume {

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

func newPodTemplateSpecForCR(cr *brokerv2alpha1.ActiveMQArtemis) corev1.PodTemplateSpec {

	// Log where we are and what we're doing
	reqLogger := log.WithName(cr.Name)
	reqLogger.Info("Creating new pod template spec for custom resource")

	//var pts corev1.PodTemplateSpec
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
	volumeMounts := makeVolumeMounts(cr)
	if len(volumeMounts) > 0 {
		container.VolumeMounts = volumeMounts
	}
	Spec.Containers = append(Containers, container)
	volumes := makeVolumes(cr)
	if len(volumes) > 0 {
		Spec.Volumes = volumes
	}
	Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
	pts.Spec = Spec

	return pts
}

func NewStatefulSetForCR(cr *brokerv2alpha1.ActiveMQArtemis) *appsv1.StatefulSet {

	// Log where we are and what we're doing
	reqLogger := log.WithName(cr.Name)
	reqLogger.Info("Creating new statefulset for custom resource")
	replicas := cr.Spec.DeploymentPlan.Size

	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

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
		ServiceName: "amq-broker-amq-headless", //cr.Name + "-headless" + "-service",
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: newPodTemplateSpecForCR(cr),
	}

	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		Spec.VolumeClaimTemplates = *pvc.NewPersistentVolumeClaimArrayForCR(cr, 1)
	}
	ss.Spec = Spec

	return ss
}

var GLOBAL_CRNAME string = ""

//func CreateStatefulSet(cr *brokerv2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*appsv1.StatefulSet, error) {
//
//	// Log where we are and what we're doing
//	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
//	reqLogger.Info("Creating new statefulset")
//	var err error = nil
//
//	// Define the StatefulSet
//	ss := NewStatefulSetForCR(cr)
//
//	// Set ActiveMQArtemis instance as the owner and controller
//	if err = controllerutil.SetControllerReference(cr, ss, scheme); err != nil {
//		// Add error detail for use later
//		reqLogger.Info("Failed to set controller reference for new " + "statefulset")
//	}
//	reqLogger.Info("Set controller reference for new " + "statefulset")
//
//	// Call k8s create for statefulset
//	if err = client.Create(context.TODO(), ss); err != nil {
//		// Add error detail for use later
//		reqLogger.Info("Failed to creating new " + "statefulset")
//	}
//	reqLogger.Info("Created new " + "statefulset")
//
//	//TODO: Remove this blatant hack
//	GLOBAL_CRNAME = cr.Name
//
//	return ss, err
//}
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

	// TODO: Remove this hack
	var crName string = statefulsetName

	labels := selectors.LabelsForActiveMQArtemis(crName)

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        statefulsetName,
			Namespace:   namespacedName.Namespace,
			Labels:      labels,
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
