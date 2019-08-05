package activemqartemis

import (
	"context"
	"github.com/RHsyseng/operator-utils/pkg/olm"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	ss "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"
)

// This is the state we should be in whenever something happens that
// requires a change to the kubernetes resources
type CreatingK8sResourcesState struct {
	s              fsm.State
	namespacedName types.NamespacedName
	parentFSM      *ActiveMQArtemisFSM
	stepsComplete  uint8
}

func MakeCreatingK8sResourcesState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) CreatingK8sResourcesState {

	rs := CreatingK8sResourcesState{
		s:              fsm.MakeState(CreatingK8sResources, CreatingK8sResourcesID),
		namespacedName: _namespacedName,
		parentFSM:      _parentFSM,
		stepsComplete:  None,
	}

	return rs
}

func NewCreatingK8sResourcesState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) *CreatingK8sResourcesState {

	rs := MakeCreatingK8sResourcesState(_parentFSM, _namespacedName)

	return &rs
}

func (rs *CreatingK8sResourcesState) ID() int {

	return CreatingK8sResourcesID
}

// First time entering state
func (rs *CreatingK8sResourcesState) enterFromInvalidState() error {

	var err error = nil
	var retrieveError error = nil

	// Check to see if the statefulset already exists
	if _, err := ss.RetrieveStatefulSet(rs.parentFSM.customResource.Name+"-ss", rs.parentFSM.namespacedName, rs.parentFSM.r.client); err != nil {
		// err means not found, so create
		if _, retrieveError := ss.CreateStatefulSet(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme); retrieveError == nil {
			rs.stepsComplete |= CreatedStatefulSet
		}
	}

	// Check to see if the headless service already exists
	if _, err = svc.RetrieveHeadlessService(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client); err != nil {
		// err means not found, so create
		if _, retrieveError = svc.CreateHeadlessService(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme); retrieveError == nil {
			rs.stepsComplete |= CreatedHeadlessService
		}
	}

	// Check to see if the ping service already exists
	if _, err = svc.RetrievePingService(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client); err != nil {
		// err means not found, so create
		if _, retrieveError = svc.CreatePingService(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme); retrieveError == nil {
			rs.stepsComplete |= CreatedPingService
		}
	}

	return err
}

func (rs *CreatingK8sResourcesState) Enter(previousStateID int) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Entering CreateK8sResourceState from " + strconv.Itoa(previousStateID))

	switch previousStateID {
	case NotCreatedID:
		rs.enterFromInvalidState()
		break
		//case ScalingID:
		// No brokers running; safe to touch journals etc...
	}

	return nil
}

func (rs *CreatingK8sResourcesState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating CreatingK8sResourcesState")

	var err error = nil
	var nextStateID int = CreatingK8sResourcesID
	var statefulSetUpdates uint32 = 0

	reconciler := ActiveMQArtemisReconciler{
		statefulSetUpdates: 0,
	}

	currentStatefulSet := &appsv1.StatefulSet{}
	err = rs.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: rs.parentFSM.customResource.Name + "-ss", Namespace: rs.parentFSM.customResource.Namespace}, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		// Do we need to check for and bounce an observed generation change here?
		if (rs.stepsComplete&CreatedStatefulSet > 0) &&
			(rs.stepsComplete&CreatedHeadlessService) > 0 &&
			(rs.stepsComplete&CreatedPingService > 0) {

			statefulSetUpdates = reconciler.Process(rs.parentFSM.customResource, currentStatefulSet)
			if statefulSetUpdates > 0 {
				err = rs.parentFSM.r.client.Update(context.TODO(), currentStatefulSet)
				if err != nil {
					reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
					break
				}
			}
			if rs.parentFSM.customResource.Spec.DeploymentPlan.Size > 0 {
				nextStateID = ScalingID
			}
		} else {
			// Not ready... requeue to wait? What other action is required - try to recreate?
			rs.parentFSM.r.result = reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}
			rs.enterFromInvalidState()
			reqLogger.Info("CreatingK8sResourcesState requesting reconcile requeue for 5 seconds due to k8s resources not created")
			break
		}

		break
	}
	updataPodStatus(rs)
	return err, nextStateID
}

func (rs *CreatingK8sResourcesState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting CreatingK8sResourceState")

	return nil
}

func updataPodStatus(rs *CreatingK8sResourcesState) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating pods status")

	podStatus := getPodStatus(rs.parentFSM.r, rs.parentFSM.customResource)

	reqLogger.Info("PodStatus are to be updated.............................", "info:", podStatus)
	reqLogger.Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.Info("Stopped Count........................", "info:", len(podStatus.Stopped))
	reqLogger.Info("Starting Count........................", "info:", len(podStatus.Starting))

	if !reflect.DeepEqual(podStatus, rs.parentFSM.customResource.Status.PodStatus) {

		rs.parentFSM.customResource.Status.PodStatus = podStatus

		err := rs.parentFSM.r.client.Status().Update(context.TODO(), rs.parentFSM.customResource)
		if err != nil {
			reqLogger.Error(err, "Failed to update pods status")
			return err
		}
		reqLogger.Info("Pods status updated")
		return nil
	}

	return nil

}

func getPodStatus(r *ReconcileActiveMQArtemis, instance *brokerv2alpha1.ActiveMQArtemis) olm.DeploymentStatus {
	// List the pods for this deployment
	var status olm.DeploymentStatus
	sfsFound := &appsv1.StatefulSet{}

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name + "-ss", Namespace: instance.Namespace}, sfsFound)
	if err == nil {
		status = olm.GetSingleStatefulSetStatus(*sfsFound)
	} else {
		dsFound := &appsv1.DaemonSet{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, dsFound)
		if err == nil {
			status = olm.GetSingleDaemonSetStatus(*dsFound)
		}
	}

	return status
}
