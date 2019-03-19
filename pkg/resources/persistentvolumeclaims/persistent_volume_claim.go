package persistentvolumeclaims

import (
	"context"
	brokerv1alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package persistentvolumeclaims")

func newPersistentVolumeClaimForCR(cr *brokerv1alpha1.ActiveMQArtemis) *corev1.PersistentVolumeClaim {

	labels := selectors.LabelsForActiveMQArtemis(cr.Name)

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: nil,
			Labels:      labels,
			Name:        cr.Name,
			Namespace:   cr.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("2Gi"),
				},
			},
		},
	}

	return pvc
}
func NewPersistentVolumeClaimArrayForCR(cr *brokerv1alpha1.ActiveMQArtemis, arrayLength int) *[]corev1.PersistentVolumeClaim {

	pvcArray := make([]corev1.PersistentVolumeClaim, 0, arrayLength)

	var pvc *corev1.PersistentVolumeClaim = nil
	for i := 0; i < arrayLength; i++ {
		pvc = newPersistentVolumeClaimForCR(cr)
		pvcArray = append(pvcArray, *pvc)
	}

	return &pvcArray
}

func CreatePersistentVolumeClaim(cr *brokerv1alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (*corev1.PersistentVolumeClaim, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.Info("Creating new persistent volume claim")

	var err error = nil

	// Define the PersistentVolumeClaim for this Pod
	brokerPvc := newPersistentVolumeClaimForCR(cr)
	// Set ActiveMQArtemis instance as the owner and controller
	if err = controllerutil.SetControllerReference(cr, brokerPvc, scheme); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to set controller reference for new " + "persistent volume claim")
	}
	reqLogger.Info("Set controller reference for new " + "persistent volume claim")

	// Call k8s create for service
	if err = client.Create(context.TODO(), brokerPvc); err != nil {
		// Add error detail for use later
		reqLogger.Info("Failed to creating new " + "persistent volume claim")
	}
	reqLogger.Info("Created new " + "persistent volume claim")

	return brokerPvc, err
}

//func (rs *CreatingK8sResourcesState) RetrievePersistentVolumeClaim(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, r *ReconcileActiveMQArtemis) (*corev1.PersistentVolumeClaim, error) {
func RetrievePersistentVolumeClaim(instance *brokerv1alpha1.ActiveMQArtemis, namespacedName types.NamespacedName, client client.Client) (*corev1.PersistentVolumeClaim, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", instance.Name)
	reqLogger.Info("Retrieving " + "persistent volume claim")

	var err error = nil
	pvc := &corev1.PersistentVolumeClaim{}

	// Check if the headless service already exists
	if err = client.Get(context.TODO(), namespacedName, pvc); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Persistent volume claim IsNotFound", "Namespace", instance.Namespace, "Name", instance.Name)
		} else {
			reqLogger.Error(err, "Persistent volume claim found", "Namespace", instance.Namespace, "Name", instance.Name)
		}
	}

	return pvc, err
}
