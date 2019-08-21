package activemqartemis

import (
	"context"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/pods"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/serviceports"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	ss "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (rs *CreatingK8sResourcesState) generateNames() {

	// Initialize the kubernetes names
	ss.NameBuilder.Base(rs.parentFSM.customResource.Name).Suffix("ss").Generate()
	svc.HeadlessNameBuilder.Prefix("amq-broker").Base("amq").Suffix("headless").Generate()
	pods.NameBuilder.Base(rs.parentFSM.customResource.Name).Suffix("container").Generate()
}

// First time entering state
func (rs *CreatingK8sResourcesState) enterFromInvalidState() error {

	var err error = nil
	var retrieveError error = nil

	rs.generateNames()
	selectors.LabelBuilder.Base(rs.parentFSM.customResource.Name).Suffix("app").Generate()
	statefulsetDefinition := ss.NewStatefulSetForCR(rs.parentFSM.customResource)

	// Check to see if the statefulset already exists
	if err := resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, statefulsetDefinition); err != nil {
		// err means not found, so create
		if retrieveError := resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, statefulsetDefinition); retrieveError == nil {
			rs.stepsComplete |= CreatedStatefulSet

			//TODO: Remove this blatant hack
			ss.GLOBAL_CRNAME = rs.parentFSM.customResource.Name
		}
	}

	headlessServiceDefinition := svc.NewHeadlessServiceForCR(rs.parentFSM.customResource, serviceports.GetDefaultPorts(rs.parentFSM.customResource))
	// Check to see if the headless service already exists
	if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, headlessServiceDefinition); err != nil {
		// err means not found, so create
		if retrieveError = resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, headlessServiceDefinition); retrieveError == nil {
			rs.stepsComplete |= CreatedHeadlessService
		}
	}

	// Check to see if the ping service already exists
	labels := selectors.LabelBuilder.Labels()
	pingServiceDefinition := svc.NewPingServiceDefinitionForCR(rs.parentFSM.customResource, labels, labels)
	if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, pingServiceDefinition); err != nil {
		// err means not found, so create
		if retrieveError = resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, pingServiceDefinition); retrieveError == nil {
			rs.stepsComplete |= CreatedPingService
		}
	}

	userPasswordStringData := secrets.MakeStringDataMap("user", "password", rs.parentFSM.customResource.Spec.DeploymentPlan.User, rs.parentFSM.customResource.Spec.DeploymentPlan.Password)
	userPasswordSecret := secrets.NewSecret(rs.parentFSM.customResource, "amq-app-secret", userPasswordStringData)
	if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, userPasswordSecret); err != nil {
		// err means not found so create
		if retrieveError = resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, userPasswordSecret); retrieveError == nil {
			rs.stepsComplete |= CreatedUserPasswordSecret
		}
	}

	clusterUserPasswordStringData := secrets.MakeStringDataMap("clusterUser", "clusterPassword", rs.parentFSM.customResource.Spec.DeploymentPlan.ClusterUser, rs.parentFSM.customResource.Spec.DeploymentPlan.ClusterPassword)
	environments.GLOBAL_AMQ_CLUSTER_USER = rs.parentFSM.customResource.Spec.DeploymentPlan.ClusterUser
	environments.GLOBAL_AMQ_CLUSTER_PASSWORD = rs.parentFSM.customResource.Spec.DeploymentPlan.ClusterPassword
	clusterUserPasswordSecret := secrets.NewSecret(rs.parentFSM.customResource, "amq-credentials-secret", clusterUserPasswordStringData)
	if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, clusterUserPasswordSecret); err != nil {
		// err means not found so create
		if retrieveError = resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, clusterUserPasswordSecret); retrieveError == nil {
			rs.stepsComplete |= CreatedClusterUserPasswordSecret
		}
	}

	return err
}

func (rs *CreatingK8sResourcesState) syncMessageMigration(cr *v2alpha1.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) {

	var err error = nil
	var retrieveError error = nil

	scaledown := &v1alpha1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    selectors.LabelBuilder.Labels(),
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: v1alpha1.ActiveMQArtemisScaledownSpec{
			MasterURL:  "",
			Kubeconfig: "",
			Namespace:  cr.Namespace,
			LocalOnly:  true,
		},
		Status: v1alpha1.ActiveMQArtemisScaledownStatus{},
	}

	if rs.parentFSM.customResource.Spec.DeploymentPlan.MessageMigration {
		if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, scaledown); err != nil {
			// err means not found so create
			if retrieveError = resources.Create(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, scaledown); retrieveError == nil {
			}
		}
	} else {
		if err = resources.Retrieve(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, scaledown); err == nil {
			// no err means found so delete
			if retrieveError = resources.Delete(rs.parentFSM.customResource, rs.parentFSM.r.client, scaledown); retrieveError == nil {
			}
		}
	}
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
	err = rs.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: ss.NameBuilder.Name(), Namespace: rs.parentFSM.customResource.Namespace}, currentStatefulSet)
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

			reconciler.SyncMessageMigration(rs.parentFSM.customResource, rs.parentFSM.r)
			statefulSetUpdates = reconciler.Process(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, currentStatefulSet)
			if 0 == statefulSetUpdates {
				if rs.parentFSM.customResource.Spec.DeploymentPlan.Size > 0 {
					nextStateID = ScalingID
				}
				break
			}
			if statefulSetUpdates > 0 {
				if err := resources.Update(rs.parentFSM.customResource, rs.parentFSM.r.client, currentStatefulSet); err != nil {
					reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
					break
				}
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
	pods.UpdatePodStatus(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.namespacedName)

	return err, nextStateID
}

func (rs *CreatingK8sResourcesState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting CreatingK8sResourceState")

	return nil
}
