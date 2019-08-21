package activemqartemis

import (
	"context"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type ScalingState struct {
	s                          fsm.State
	namespacedName             types.NamespacedName
	parentFSM                  *ActiveMQArtemisFSM
	enteringObservedGeneration int64
}

func MakeScalingState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) ScalingState {

	ss := ScalingState{
		s:                          fsm.MakeState(Scaling, ScalingID),
		namespacedName:             _namespacedName,
		parentFSM:                  _parentFSM,
		enteringObservedGeneration: 0,
	}

	return ss
}

func (ss *ScalingState) ID() int {

	return ScalingID
}

func (ss *ScalingState) Enter(previousStateID int) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Entering ScalingState")

	var err error = nil

	currentStatefulSet := &appsv1.StatefulSet{}
	err = ss.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: ss.parentFSM.customResource.Namespace}, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		// Take note, as this will change if a custom resource update is made. We want to requeue
		// these for later when not scaling
		ss.enteringObservedGeneration = currentStatefulSet.Status.ObservedGeneration

		break
	}

	return err
}

func (ss *ScalingState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Updating ScalingState")

	var err error = nil
	var nextStateID int = ScalingID

	currentStatefulSet := &appsv1.StatefulSet{}
	err = ss.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: ss.parentFSM.customResource.Namespace}, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		//if currentStatefulSet.Status.Replicas == currentStatefulSet.Status.ReadyReplicas {
		if *currentStatefulSet.Spec.Replicas == currentStatefulSet.Status.ReadyReplicas {
			ss.parentFSM.r.result = reconcile.Result{Requeue: true}
			reqLogger.Info("ScalingState requesting reconcile requeue for immediate reissue due to scaling completion")

			if *currentStatefulSet.Spec.Replicas > 0 {
				nextStateID = ContainerRunningID
			}

			if 0 == *currentStatefulSet.Spec.Replicas {
				nextStateID = CreatingK8sResourcesID
			}

			break
		}

		// Do we have an incoming change to the custom resource and not just an update?
		if ss.enteringObservedGeneration != currentStatefulSet.Status.ObservedGeneration {
			ss.parentFSM.r.result = reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}
			reqLogger.Info("ScalingState requesting reconcile requeue for 5 seconds due to scaling")
			break
		}

		break
	}

	return err, nextStateID
}

func (ss *ScalingState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Exiting ScalingState")

	var err error = nil

	return err
}
