package statefulsets

import (
	"context"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime"
)

var log = logf.Log.WithName("package statefulsets")

func MakeStatefulSet2(currentStateFulSet *appsv1.StatefulSet, ssName string, svcHeadlessName string, namespacedName types.NamespacedName, annotations map[string]string, labels map[string]string, replicas int32) *appsv1.StatefulSet {

	if currentStateFulSet == nil {
		currentStateFulSet = &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ssName,
				Namespace: namespacedName.Namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: *pods.MakePodTemplateSpec(nil, namespacedName, labels),
			},
		}
	}
	currentStateFulSet.ObjectMeta.Labels = labels
	currentStateFulSet.ObjectMeta.Annotations = annotations

	currentStateFulSet.Spec.Replicas = &replicas
	currentStateFulSet.Spec.ServiceName = svcHeadlessName
	currentStateFulSet.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labels,
	}

	currentStateFulSet.Spec.Template = *pods.MakePodTemplateSpec(&currentStateFulSet.Spec.Template, namespacedName, labels)

	return currentStateFulSet
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
