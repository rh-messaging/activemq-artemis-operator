package controllers

import (
	"context"
	"strconv"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// This is the state we should be in whenever kubernetes
// resources are stable and only configuration is changing
type ContainerRunningState struct {
	s              fsm.State
	namespacedName types.NamespacedName
	parentFSM      *ActiveMQArtemisFSM
}

func MakeContainerRunningState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) ContainerRunningState {

	rs := ContainerRunningState{
		s:              fsm.MakeState(ContainerRunning, ContainerRunningID),
		namespacedName: _namespacedName,
		parentFSM:      _parentFSM,
	}

	return rs
}

func NewContainerRunningState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) *ContainerRunningState {

	rs := MakeContainerRunningState(_parentFSM, _namespacedName)

	return &rs
}

func (rs *ContainerRunningState) ID() int {

	return ContainerRunningID
}

func (rs *ContainerRunningState) Enter(previousStateID int) error {

	// Log where we are and what we're doing
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Entering ContainerRunningState from " + strconv.Itoa(previousStateID))

	// TODO: Clear up ambiguity in usage between container and pod
	// Check to see how many pods are running atm

	return nil
}

func (rs *ContainerRunningState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := ctrl.Log.WithValues("state", "running", "ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating ContainerRunningState")

	var err error = nil
	var nextStateID int = ContainerRunningID
	var statefulSetUpdates uint32 = 0

	reconciler := ActiveMQArtemisReconcilerImpl{
		statefulSetUpdates: 0,
	}
	ssNamespacedName := types.NamespacedName{Name: rs.parentFSM.GetStatefulSetName(), Namespace: rs.parentFSM.customResource.Namespace}
	currentStatefulSet := &appsv1.StatefulSet{}
	//	var newss *appsv1.StatefulSet

	reqLogger.Info("Retrieving current ss", "in ns", ssNamespacedName)
	err = rs.parentFSM.r.Client.Get(context.TODO(), ssNamespacedName, currentStatefulSet)

	firstTime := false
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		if *currentStatefulSet.Spec.Replicas != currentStatefulSet.Status.ReadyReplicas {
			rs.parentFSM.r.Result = reconcile.Result{Requeue: true}
			reqLogger.Info("ContainerRunningState requesting reconcile requeue for immediate reissue due to continued scaling")
			nextStateID = ScalingID
			break
		}

		reqLogger.Info("Calling reconciler.Process()...")
		statefulSetUpdates, _, _ = reconciler.Process(rs.parentFSM, rs.parentFSM.r.Client, rs.parentFSM.r.Scheme, firstTime)
		break
	}

	if statefulSetUpdates > 0 {
		reqLogger.Info("Next need be Scaling as ss updated", "ssUpdate", statefulSetUpdates, "saclestate", ScalingID)
		/*
			//https://stackoverflow.com/questions/65987577/kubectl-apply-reports-error-operation-cannot-be-fulfilled-on-serviceaccounts
			currentStatefulSet.ResourceVersion = ""
			if err := resources.Update(rs.parentFSM.namespacedName, rs.parentFSM.r.Client, newss); err != nil {
				reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", newss.Namespace, "Deployment.Name", newss.Name)
			}
		*/
		nextStateID = ScalingID
	}

	reqLogger.Info("returning", "err", err, "nextstate", nextStateID)
	return err, nextStateID
}

func (rs *ContainerRunningState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting ContainerRunningState")

	return nil
}
