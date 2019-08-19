package activemqartemis

import (
	"context"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources"
	ss "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Entering ContainerRunningState")

	// TODO: Clear up ambiguity in usage between container and pod
	// Check to see how many pods are running atm

	return nil
}

func (rs *ContainerRunningState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating ContainerRunningState")

	var err error = nil
	var nextStateID int = ContainerRunningID
	var statefulSetUpdates uint32 = 0

	reconciler := ActiveMQArtemisReconciler{
		statefulSetUpdates: 0,
	}

	currentStatefulSet := &appsv1.StatefulSet{}
	err = rs.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: ss.NameBuilder.Name(), Namespace: rs.parentFSM.customResource.Namespace}, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		statefulSetUpdates = reconciler.Process(rs.parentFSM.customResource, rs.parentFSM.r.client, currentStatefulSet)

		break
	}

	if statefulSetUpdates > 0 {
		if err := resources.Update(rs.parentFSM.customResource, rs.parentFSM.r.client, currentStatefulSet); err != nil {
			reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
		}
		nextStateID = ScalingID
	}

	return err, nextStateID
}

func (rs *ContainerRunningState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting ContainerRunningState")

	return nil
}
