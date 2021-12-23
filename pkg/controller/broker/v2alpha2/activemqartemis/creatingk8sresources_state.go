package v2alpha2activemqartemis

import (
	"context"

	"github.com/RHsyseng/operator-utils/pkg/resource"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/serviceports"
	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/random"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	appsv1 "k8s.io/api/apps/v1"

	//"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
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

func (rs *CreatingK8sResourcesState) generateSecrets() *corev1.Secret {

	credentialsSecretName := secrets.CredentialsNameBuilder.Name()
	namespacedName := types.NamespacedName{
		Name:      credentialsSecretName,
		Namespace: rs.namespacedName.Namespace,
	}

	clusterUser := random.GenerateRandomString(8)
	clusterPassword := random.GenerateRandomString(8)
	adminUser := ""
	adminPassword := ""

	if "" == rs.parentFSM.customResource.Spec.AdminUser {
		adminUser = environments.Defaults.AMQ_USER
	} else {
		adminUser = rs.parentFSM.customResource.Spec.AdminUser
	}
	if "" == rs.parentFSM.customResource.Spec.AdminPassword {
		adminPassword = environments.Defaults.AMQ_PASSWORD
	} else {
		adminPassword = rs.parentFSM.customResource.Spec.AdminPassword
	}

	stringDataMap := map[string]string{
		"clusterUser":     clusterUser,
		"clusterPassword": clusterPassword,
		"AMQ_USER":        adminUser,
		"AMQ_PASSWORD":    adminPassword,
	}
	// TODO: Remove this hack
	environments.GLOBAL_AMQ_CLUSTER_USER = clusterUser
	environments.GLOBAL_AMQ_CLUSTER_PASSWORD = clusterPassword

	secretDefinition := secrets.NewSecret(namespacedName, namespacedName.Name, stringDataMap, selectors.GetLabels(rs.parentFSM.namespacedName.Name))
	//secretDefinition := secrets.Create(rs.parentFSM.customResource, namespacedName, stringDataMap, rs.parentFSM.r.client, rs.parentFSM.r.scheme)
	return secretDefinition
}

// First time entering state
func (rs *CreatingK8sResourcesState) enterFromInvalidState() error {

	var err error = nil
	stepsComplete := rs.stepsComplete
	firstTime := true

	rs.generateNames()
	secretDefinition := rs.generateSecrets()

	//volumes.GLOBAL_DATA_PATH = environments.GetPropertyForCR("AMQ_DATA_DIR", rs.parentFSM.customResource, "/opt/"+rs.parentFSM.customResource.Name+"/data")
	labels := selectors.GetLabels(rs.parentFSM.namespacedName.Name)

	namespacedName := types.NamespacedName{
		Name:      rs.parentFSM.customResource.Name,
		Namespace: rs.parentFSM.customResource.Namespace,
	}
	statefulsetDefinition := NewStatefulSetForCR(rs.parentFSM.customResource)
	headlessServiceDefinition := svc.NewHeadlessServiceForCR(namespacedName, serviceports.GetDefaultPorts(), labels)
	pingServiceDefinition := svc.NewPingServiceDefinitionForCR(namespacedName, labels, labels)

	//for upgrade
	var requestedResources []resource.KubernetesResource
	requestedResources = append(requestedResources, secretDefinition)
	requestedResources = append(requestedResources, statefulsetDefinition)
	requestedResources = append(requestedResources, headlessServiceDefinition)
	requestedResources = append(requestedResources, pingServiceDefinition)
	_, stepsComplete = reconciler.Process(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, statefulsetDefinition, firstTime, requestedResources)

	rs.stepsComplete = stepsComplete

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
	var allObjects []resource.KubernetesResource

	// NOTE: By all service objects here we mean headless and ping service objects
	err, allObjects = getServiceObjects(rs.parentFSM.customResource, rs.parentFSM.r.client, allObjects)

	currentStatefulSet := &appsv1.StatefulSet{}
	ssNamespacedName := types.NamespacedName{Name: ss.NameBuilder.Name(), Namespace: rs.parentFSM.customResource.Namespace}
	namespacedName := types.NamespacedName{
		Name:      rs.parentFSM.customResource.Name,
		Namespace: rs.parentFSM.customResource.Namespace,
	}
	err = rs.parentFSM.r.client.Get(context.TODO(), ssNamespacedName, currentStatefulSet)
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

			firstTime := false

			allObjects = append(allObjects, currentStatefulSet)
			statefulSetUpdates, _ = reconciler.Process(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, currentStatefulSet, firstTime, allObjects)
			if statefulSetUpdates > 0 {
				if err := resources.Update(namespacedName, rs.parentFSM.r.client, currentStatefulSet); err != nil {
					reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
					break
				}
			}
			if rs.parentFSM.customResource.Spec.DeploymentPlan.Size != currentStatefulSet.Status.ReadyReplicas {
				if rs.parentFSM.customResource.Spec.DeploymentPlan.Size > 0 {
					nextStateID = ScalingID
				}
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
	//pods.UpdatePodStatus(rs.parentFSM.customResource, rs.parentFSM.r.client, ssNamespacedName)

	return err, nextStateID
}

func (rs *CreatingK8sResourcesState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting CreatingK8sResourceState")

	return nil
}
