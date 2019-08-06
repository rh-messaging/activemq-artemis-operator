package activemqartemis

import (
	"context"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/ingresses"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/routes"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/env"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
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

	// Did we...
	err = ss.configureServices()
	err = ss.configureExternalNetworkAccess()

	return err
}

func (rs *ScalingState) configureServices() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("CreatingK8sResourcesState configureServices")

	var err error = nil
	var i int32 = 0
	ordinalString := ""

	portNumber := int32(8161)
	cr := rs.parentFSM.customResource
	client := rs.parentFSM.r.client
	scheme := rs.parentFSM.r.scheme
	labels := selectors.LabelBuilder.Labels()
	for ; i < cr.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		labels["statefulset.kubernetes.io/pod-name"] = statefulsets.NameBuilder.Name() + "-" + ordinalString

		baseServiceName := "console-jolokia"
		serviceDefinition := svc.NewServiceDefinitionForCR(cr, baseServiceName+"-"+ordinalString, portNumber, labels)
		if err = svc.CreateService(cr, client, scheme, serviceDefinition); err != nil {
			reqLogger.Info("Failure to create " + baseServiceName + " service " + ordinalString)
			continue
		}

		baseServiceName = "all-protocol"
		portNumber = int32(61616)
		serviceDefinition = svc.NewServiceDefinitionForCR(cr, baseServiceName+"-"+ordinalString, portNumber, labels)
		if err = svc.CreateService(cr, client, scheme, serviceDefinition); err != nil {
			reqLogger.Info("Failure to create " + baseServiceName + " service " + ordinalString)
			continue
		}
	}

	return err
}

func (rs *ScalingState) configureExternalNetworkAccess() error {

	var err error = nil
	var isOpenshift bool = false

	if isOpenshift, err = env.DetectOpenshift(); err != nil {
		log.Error(err, "Failed to get env, will try kubernetes")
	}
	if isOpenshift {
		log.Info("Environment is OpenShift")
		err = rs.configureRoutes()
	} else {
		log.Info("Environment is not OpenShift, creating ingress")
		err = rs.configureIngress()
	}

	return err
}

func (rs *ScalingState) configureIngress() error {

	var err error = nil

	if _, err = ingresses.RetrieveIngress(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client); err != nil {
		// err means not found, so create routes
		if _, err = ingresses.CreateNewIngress(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme); err == nil {
		}
	}

	return err
}

func (rs *ScalingState) configureRoutes() error {

	var err error = nil
	var passthroughTLS bool

	for i := 0; i < int(rs.parentFSM.customResource.Spec.DeploymentPlan.Size); i++ {
		passthroughTLS = false
		targetPortName := "console-jolokia" + "-" + strconv.Itoa(i)
		targetServiceName := rs.parentFSM.customResource.Name + "-service-" + targetPortName
		log.Info("Checking route for " + targetPortName)

		consoleJolokiaRoute := routes.NewRouteDefinitionForCR(rs.parentFSM.customResource, selectors.LabelBuilder.Labels(), targetServiceName, targetPortName, passthroughTLS)
		if err = routes.RetrieveRoute(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, consoleJolokiaRoute); err != nil {
			routes.DeleteRoute(rs.parentFSM.customResource, rs.parentFSM.r.client, consoleJolokiaRoute)
		}
		if err = routes.CreateNewRoute(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, consoleJolokiaRoute); err == nil {
		}

		targetPortName = "all-protocol" + "-" + strconv.Itoa(i)
		targetServiceName = rs.parentFSM.customResource.Name + "-service-" + targetPortName
		log.Info("Checking route for " + targetPortName)
		passthroughTLS = true
		allProtocolRoute := routes.NewRouteDefinitionForCR(rs.parentFSM.customResource, selectors.LabelBuilder.Labels(), targetServiceName, targetPortName, passthroughTLS)
		if err = routes.RetrieveRoute(rs.parentFSM.customResource, rs.parentFSM.namespacedName, rs.parentFSM.r.client, allProtocolRoute); err != nil {
			routes.DeleteRoute(rs.parentFSM.customResource, rs.parentFSM.r.client, allProtocolRoute)
		}
		if err = routes.CreateNewRoute(rs.parentFSM.customResource, rs.parentFSM.r.client, rs.parentFSM.r.scheme, allProtocolRoute); err == nil {
		}
	}

	return err
}
