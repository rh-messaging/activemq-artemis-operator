package statefulsets

import (
	"context"

	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime"
)

var log = logf.Log.WithName("package statefulsets")
var NameBuilder namer.NamerData

func MakeStatefulSet(namespacedName types.NamespacedName, annotations map[string]string, labels map[string]string, replicas int32, pts corev1.PodTemplateSpec) (*appsv1.StatefulSet, appsv1.StatefulSetSpec) {

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        NameBuilder.Name(),
			Namespace:   namespacedName.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	Spec := appsv1.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: svc.HeadlessNameBuilder.Name(),
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: pts,
	}

	return ss, Spec
}

func MakeStatefulSet2(ssName string, svcHeadlessName string, namespacedName types.NamespacedName, annotations map[string]string, labels map[string]string, replicas int32, pts corev1.PodTemplateSpec) (*appsv1.StatefulSet, appsv1.StatefulSetSpec) {

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        ssName,
			Namespace:   namespacedName.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	Spec := appsv1.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: svcHeadlessName,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: pts,
	}

	log.Info("created statefulset", "spec", Spec)
	return ss, Spec
}

var GLOBAL_CRNAME string = ""

func RetrieveStatefulSet(statefulsetName string, namespacedName types.NamespacedName, labels map[string]string, client client.Client) (*appsv1.StatefulSet, error) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.Info("Retrieving " + "StatefulSet " + statefulsetName)

	var err error = nil

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

	if err = client.Get(context.TODO(), namespacedName, ss); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(1).Info("StatefulSet claim IsNotFound", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		} else {
			reqLogger.V(1).Info("StatefulSet claim found", "Namespace", namespacedName.Namespace, "Name", namespacedName.Name)
		}
	}

	return ss, err
}
