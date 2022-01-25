package controllers

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Entering ScalingState from " + strconv.Itoa(previousStateID))

	var err error = nil

	currentStatefulSet := &appsv1.StatefulSet{}
	err = ss.parentFSM.r.Client.Get(context.TODO(), types.NamespacedName{Name: ss.parentFSM.GetStatefulSetName(), Namespace: ss.parentFSM.customResource.Namespace}, currentStatefulSet)
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
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Updating ScalingState")

	var err error = nil
	var nextStateID int = ScalingID

	currentStatefulSet := &appsv1.StatefulSet{}
	ssNamespacedName := types.NamespacedName{Name: ss.parentFSM.GetStatefulSetName(), Namespace: ss.parentFSM.customResource.Namespace}
	err = ss.parentFSM.r.Client.Get(context.TODO(), ssNamespacedName, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		if (*currentStatefulSet.Spec.Replicas == currentStatefulSet.Status.ReadyReplicas) &&
			(0 == strings.Compare(currentStatefulSet.Status.CurrentRevision, currentStatefulSet.Status.UpdateRevision)) {
			ss.parentFSM.r.Result = reconcile.Result{Requeue: true}
			reqLogger.Info("ScalingState requesting reconcile requeue for immediate reissue due to scaling completion")

			if 0 == *currentStatefulSet.Spec.Replicas {
				nextStateID = CreatingK8sResourcesID
				break
			}

			if *currentStatefulSet.Spec.Replicas > 0 {
				nextStateID = ContainerRunningID
				break
			}
		}

		// Do we have an incoming change to the custom resource and not just an update?
		if ss.enteringObservedGeneration != currentStatefulSet.Status.ObservedGeneration {
			ss.parentFSM.r.Result = reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}
			reqLogger.Info("ScalingState requesting reconcile requeue for 5 seconds due to scaling")
			break
		}

		break
	}
	//pods.UpdatePodStatus(ss.parentFSM.customResource, ss.parentFSM.r.client, ssNamespacedName)

	return err, nextStateID
}

func (ss *ScalingState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", ss.parentFSM.customResource.Name)
	reqLogger.Info("Exiting ScalingState")

	var err error = nil

	return err
}
