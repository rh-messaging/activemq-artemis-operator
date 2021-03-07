package v2alpha3activemqartemis

import (
	"context"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	corev1 "k8s.io/api/core/v1"

	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/volumes"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"strconv"
	"time"
)

var reconciler = ActiveMQArtemisReconciler{
	statefulSetUpdates: 0,
}

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
	svc.HeadlessNameBuilder.Prefix(rs.parentFSM.customResource.Name).Base("hdls").Suffix("svc").Generate()
	svc.PingNameBuilder.Prefix(rs.parentFSM.customResource.Name).Base("ping").Suffix("svc").Generate()
	pods.NameBuilder.Base(rs.parentFSM.customResource.Name).Suffix("container").Generate()
	secrets.CredentialsNameBuilder.Prefix(rs.parentFSM.customResource.Name).Base("credentials").Suffix("secret").Generate()
	secrets.ConsoleNameBuilder.Prefix(rs.parentFSM.customResource.Name).Base("console").Suffix("secret").Generate()
	secrets.NettyNameBuilder.Prefix(rs.parentFSM.customResource.Name).Base("netty").Suffix("secret").Generate()
}

// First time entering state
func (rs *CreatingK8sResourcesState) enterFromInvalidState() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("CreateK8sResourceState enterFromInvalidstate")

	var err error = nil

	rs.generateNames()
	selectors.LabelBuilder.Base(rs.parentFSM.customResource.Name).Suffix("app").Generate()
	volumes.GLOBAL_DATA_PATH = "/opt/" + rs.parentFSM.customResource.Name + "/data"

	// Precreate empty credentials and netty secrets
	_ = rs.generateSecrets()

	return err
}

func (rs *CreatingK8sResourcesState) generateSecrets() *corev1.Secret {

	namespacedName := types.NamespacedName{
		Name:      secrets.CredentialsNameBuilder.Name(),
		Namespace: rs.namespacedName.Namespace,
	}
	stringDataMap := map[string]string{
		"AMQ_CLUSTER_USER":     "",
		"AMQ_CLUSTER_PASSWORD": "",
		"AMQ_USER":             "",
		"AMQ_PASSWORD":         "",
	}
	secretDefinition := secrets.Create(rs.parentFSM.customResource, namespacedName, stringDataMap, rs.parentFSM.r.client, rs.parentFSM.r.scheme)

	namespacedName.Name = secrets.NettyNameBuilder.Name()
	nettyDataMap := map[string]string{
		"AMQ_ACCEPTORS":  "",
		"AMQ_CONNECTORS": "",
	}
	secretDefinition = secrets.Create(rs.parentFSM.customResource, namespacedName, nettyDataMap, rs.parentFSM.r.client, rs.parentFSM.r.scheme)

	return secretDefinition
}

func (rs *CreatingK8sResourcesState) Enter(previousStateID int) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Entering CreatingK8sResourcesState from " + strconv.Itoa(previousStateID))

	var stepsComplete uint8 = 0
	firstTime := false

	switch previousStateID {
	case NotCreatedID:
		firstTime = true
		rs.enterFromInvalidState()
		break
		//case ScalingID:
		// No brokers running; safe to touch journals etc...
	}

	_, stepsComplete = reconciler.Process(rs.parentFSM, rs.parentFSM.r.client, rs.parentFSM.r.scheme, firstTime)
	rs.stepsComplete = stepsComplete

	return nil
}

func (rs *CreatingK8sResourcesState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating CreatingK8sResourcesState")

	var err error = nil
	var nextStateID int = CreatingK8sResourcesID

	currentStatefulSet := &appsv1.StatefulSet{}
	ssNamespacedName := types.NamespacedName{Name: ss.NameBuilder.Name(), Namespace: rs.parentFSM.customResource.Namespace}
	err = rs.parentFSM.r.client.Get(context.TODO(), ssNamespacedName, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		} else {
			rs.stepsComplete |= CreatedStatefulSet
		}

		// Do we need to check for and bounce an observed generation change here?
		if rs.stepsComplete&CreatedStatefulSet > 0 { //&&
			firstTime := false

			_, _ = reconciler.Process(rs.parentFSM, rs.parentFSM.r.client, rs.parentFSM.r.scheme, firstTime)
			if rs.parentFSM.customResource.Spec.DeploymentPlan.Size != currentStatefulSet.Status.ReadyReplicas {
				if rs.parentFSM.customResource.Spec.DeploymentPlan.Size > 0 {
					nextStateID = ScalingID
					break
				}
			} else if currentStatefulSet.Status.ReadyReplicas > 0 {
				nextStateID = ContainerRunningID
				break
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

	return err, nextStateID
}

func (rs *CreatingK8sResourcesState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting CreatingK8sResourceState")

	return nil
}
