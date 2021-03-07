package v2alpha3activemqartemis

import (
	"context"
	"fmt"

	"github.com/RHsyseng/operator-utils/pkg/olm"
	"github.com/RHsyseng/operator-utils/pkg/resource"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	"github.com/RHsyseng/operator-utils/pkg/resource/read"
	activemqartemisscaledown "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha1/activemqartemisscaledown"
	v2alpha2activemqartemisaddress "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha2/activemqartemisaddress"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/containers"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/ingresses"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/persistentvolumeclaims"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/routes"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/serviceports"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/config"
	cr2jinja2v2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	//	brokerv2alpha2 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/volumes"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"

	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strconv"
	"strings"

	"os"
)

const (
	statefulSetNotUpdated           = 0
	statefulSetSizeUpdated          = 1 << 0
	statefulSetClusterConfigUpdated = 1 << 1
	statefulSetImageUpdated         = 1 << 2
	statefulSetPersistentUpdated    = 1 << 3
	statefulSetAioUpdated           = 1 << 4
	statefulSetCommonConfigUpdated  = 1 << 5
	statefulSetRequireLoginUpdated  = 1 << 6
	//statefulSetRoleUpdated          = 1 << 7
	statefulSetAcceptorsUpdated  = 1 << 8
	statefulSetConnectorsUpdated = 1 << 9
	statefulSetConsoleUpdated    = 1 << 10
)

var defaultMessageMigration bool = true
var requestedResources []resource.KubernetesResource
var lastStatus olm.DeploymentStatus

//default ApplyRule for address-settings
var defApplyRule string = "merge_all"

type ActiveMQArtemisReconciler struct {
	statefulSetUpdates uint32
}

type ActiveMQArtemisIReconciler interface {
	Process(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, firstTime bool) uint32
	ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client client.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool)
	ProcessCredentials(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessDeploymentPlan(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32
	ProcessAcceptorsAndConnectors(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessConsole(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet)
	ProcessResources(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8
	ProcessAddressSettings(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client) bool
}

func (reconciler *ActiveMQArtemisReconciler) Process(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, firstTime bool) (uint32, uint8) {

	var log = logf.Log.WithName("controller_v2alpha3activemqartemis")
	log.Info("Reconciler Processing...", "Operator version", version.Version, "ActiveMQArtemis release", fsm.customResource.Spec.Version)

	currentStatefulSet, firstTime := reconciler.ProcessStatefulSet(fsm, client, log, firstTime)
	statefulSetUpdates := reconciler.ProcessDeploymentPlan(fsm.customResource, client, scheme, currentStatefulSet, firstTime)
	statefulSetUpdates |= reconciler.ProcessCredentials(fsm.customResource, client, scheme, currentStatefulSet)
	statefulSetUpdates |= reconciler.ProcessAcceptorsAndConnectors(fsm.customResource, client, scheme, currentStatefulSet)
	statefulSetUpdates |= reconciler.ProcessConsole(fsm.customResource, client, scheme, currentStatefulSet)

	requestedResources = append(requestedResources, currentStatefulSet)
	stepsComplete := reconciler.ProcessResources(fsm.customResource, client, scheme, currentStatefulSet)

	if statefulSetUpdates > 0 {
		ssNamespacedName := types.NamespacedName{Name: ss.NameBuilder.Name(), Namespace: fsm.customResource.Namespace}
		if err := resources.Update(ssNamespacedName, client, currentStatefulSet); err != nil {
			log.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
		}
	}

	return statefulSetUpdates, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client client.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool) {

	ssNamespacedName := types.NamespacedName{
		Name:      ss.NameBuilder.Name(),
		Namespace: fsm.customResource.Namespace,
	}
	currentStatefulSet, err := ss.RetrieveStatefulSet(ss.NameBuilder.Name(), ssNamespacedName, client)
	if errors.IsNotFound(err) {
		log.Info("StatefulSet: " + ssNamespacedName.Name + " not found, will create")
		currentStatefulSet = NewStatefulSetForCR(fsm.customResource)
		firstTime = true
	} else {
		log.Info("StatefulSet: " + currentStatefulSet.Name + " found")
		//update statefulset with customer resource
		if reconciler.ProcessAddressSettings(fsm.customResource, fsm.prevCustomResource, client) {
			log.Info("There are new address settings change in the cr, creating a new pod template to update")
			*fsm.prevCustomResource = *fsm.customResource
			currentStatefulSet.Spec.Template = NewPodTemplateSpecForCR(fsm.customResource)
		}
	}

	headlessServiceDefinition := svc.NewHeadlessServiceForCR(ssNamespacedName, serviceports.GetDefaultPorts())
	labels := selectors.LabelBuilder.Labels()
	pingServiceDefinition := svc.NewPingServiceDefinitionForCR(ssNamespacedName, labels, labels)
	requestedResources = append(requestedResources, headlessServiceDefinition)
	requestedResources = append(requestedResources, pingServiceDefinition)

	return currentStatefulSet, firstTime
}

func (reconciler *ActiveMQArtemisReconciler) ProcessCredentials(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var log = logf.Log.WithName("controller_v2alpha3activemqartemis")
	log.V(1).Info("ProcessCredentials")

	adminUser := ""
	adminPassword := ""
	// TODO: Remove singular admin level user and password in favour of at least guest and admin access
	secretName := secrets.CredentialsNameBuilder.Name()
	envVarName1 := "AMQ_USER"
	for {
		adminUser = customResource.Spec.AdminUser
		if "" != adminUser {
			break
		}

		if amqUserEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_USER"); nil != amqUserEnvVar {
			adminUser = amqUserEnvVar.Value
		}
		if "" != adminUser {
			break
		}

		adminUser = environments.Defaults.AMQ_USER
		break
	} // do once

	envVarName2 := "AMQ_PASSWORD"
	for {
		adminPassword = customResource.Spec.AdminPassword
		if "" != adminPassword {
			break
		}

		if amqPasswordEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_PASSWORD"); nil != amqPasswordEnvVar {
			adminPassword = amqPasswordEnvVar.Value
		}
		if "" != adminPassword {
			break
		}

		adminPassword = environments.Defaults.AMQ_PASSWORD
		break
	} // do once

	envVars := make(map[string]string)
	envVars[envVarName1] = adminUser
	envVars[envVarName2] = adminPassword
	envVars["AMQ_CLUSTER_USER"] = environments.GLOBAL_AMQ_CLUSTER_USER
	envVars["AMQ_CLUSTER_PASSWORD"] = environments.GLOBAL_AMQ_CLUSTER_PASSWORD
	statefulSetUpdates := sourceEnvVarFromSecret(customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) ProcessDeploymentPlan(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32 {

	deploymentPlan := &customResource.Spec.DeploymentPlan

	// Ensure the StatefulSet size is the same as the spec
	if *currentStatefulSet.Spec.Replicas != deploymentPlan.Size {
		currentStatefulSet.Spec.Replicas = &deploymentPlan.Size
		reconciler.statefulSetUpdates |= statefulSetSizeUpdated
	}

	if imageSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetImageUpdated
	}

	if aioSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetAioUpdated
	}

	if firstTime {
		if persistentSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
			reconciler.statefulSetUpdates |= statefulSetPersistentUpdated
		}
	}

	if updatedEnvVar := environments.BoolSyncCausedUpdateOn(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_REQUIRE_LOGIN", deploymentPlan.RequireLogin); updatedEnvVar != nil {
		environments.Update(currentStatefulSet.Spec.Template.Spec.Containers, updatedEnvVar)
		reconciler.statefulSetUpdates |= statefulSetRequireLoginUpdated
	}

	syncMessageMigration(customResource, client, scheme)

	return reconciler.statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) ProcessAddressSettings(customResource *brokerv2alpha3.ActiveMQArtemis, prevCustomResource *brokerv2alpha3.ActiveMQArtemis, client client.Client) bool {

	var log = logf.Log.WithName("controller_v2alpha2activemqartemis")
	log.Info("Process addresssettings")

	if len(customResource.Spec.AddressSettings.AddressSetting) == 0 {
		return false
	}

	//we need to compare old with new and update if they are different.
	return compareAddressSettings(&prevCustomResource.Spec.AddressSettings, &customResource.Spec.AddressSettings)
}

//returns true if currentAddressSettings need update
func compareAddressSettings(currentAddressSettings *brokerv2alpha3.AddressSettingsType, newAddressSettings *brokerv2alpha3.AddressSettingsType) bool {

	if len((*currentAddressSettings).AddressSetting) != len((*newAddressSettings).AddressSetting) || *(*currentAddressSettings).ApplyRule != *(*newAddressSettings).ApplyRule || !config.IsEqual((*currentAddressSettings).AddressSetting, (*newAddressSettings).AddressSetting) {
		return true
	}
	return false
}

func (reconciler *ActiveMQArtemisReconciler) ProcessAcceptorsAndConnectors(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var retVal uint32 = statefulSetNotUpdated

	acceptorEntry := generateAcceptorsString(customResource, client)
	connectorEntry := generateConnectorsString(customResource, client)

	configureAcceptorsExposure(customResource, client, scheme)
	configureConnectorsExposure(customResource, client, scheme)

	envVars := map[string]string{
		"AMQ_ACCEPTORS":  acceptorEntry,
		"AMQ_CONNECTORS": connectorEntry,
	}
	secretName := secrets.NettyNameBuilder.Name()
	retVal = sourceEnvVarFromSecret(customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func (reconciler *ActiveMQArtemisReconciler) ProcessConsole(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var retVal uint32 = statefulSetNotUpdated

	configureConsoleExposure(customResource, client, scheme)
	if !customResource.Spec.Console.SSLEnabled {
		return retVal
	}

	sslFlags := ""
	envVarName := "AMQ_CONSOLE_ARGS"
	secretName := secrets.ConsoleNameBuilder.Name()
	if "" != customResource.Spec.Console.SSLSecret {
		secretName = customResource.Spec.Console.SSLSecret
	}
	sslFlags = generateConsoleSSLFlags(customResource, client, secretName)
	envVars := make(map[string]string)
	envVars[envVarName] = sslFlags
	retVal = sourceEnvVarFromSecret(customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func syncMessageMigration(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) {

	var err error = nil
	var retrieveError error = nil

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	scaledown := &brokerv2alpha1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    selectors.LabelBuilder.Labels(),
			Name:      customResource.Name,
			Namespace: customResource.Namespace,
		},
		Spec: brokerv2alpha1.ActiveMQArtemisScaledownSpec{
			LocalOnly: true,
		},
		Status: brokerv2alpha1.ActiveMQArtemisScaledownStatus{},
	}

	if nil == customResource.Spec.DeploymentPlan.MessageMigration {
		customResource.Spec.DeploymentPlan.MessageMigration = &defaultMessageMigration
	}

	if *customResource.Spec.DeploymentPlan.MessageMigration {
		if err = resources.Retrieve(namespacedName, client, scaledown); err != nil {
			// err means not found so create
			if retrieveError = resources.Create(customResource, namespacedName, client, scheme, scaledown); retrieveError == nil {
			}
		}
	} else {
		if err = resources.Retrieve(namespacedName, client, scaledown); err == nil {
			close(activemqartemisscaledown.StopCh)
			// err means not found so delete
			if retrieveError = resources.Delete(namespacedName, client, scaledown); retrieveError == nil {
			}
		}
	}
}

func sourceEnvVarFromSecret(customResource *brokerv2alpha3.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]string, secretName string, client client.Client, scheme *runtime.Scheme) uint32 {

	var log = logf.Log.WithName("controller_v2alpha3activemqartemis")

	var err error = nil
	var retVal uint32 = statefulSetNotUpdated

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: currentStatefulSet.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := make(map[string]string)
	for k := range *envVars {
		stringDataMap[k] = (*envVars)[k]
	}
	secretDefinition := secrets.NewSecret(namespacedName, secretName, stringDataMap)
	if err = resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("Did not find secret " + secretName)
			requestedResources = append(requestedResources, secretDefinition)
		}
	} else { // err == nil so it already exists
		// Exists now
		// Check the contents against what we just got above
		log.V(1).Info("Found secret " + secretName)

		var needUpdate bool = false
		for k := range *envVars {
			elem, ok := secretDefinition.Data[k]
			if 0 != strings.Compare(string(elem), (*envVars)[k]) || !ok {
				log.V(1).Info("Secret exists but not equals, or not ok", "ok?", ok)
				secretDefinition.Data[k] = []byte((*envVars)[k])
				needUpdate = true
			}
		}

		if needUpdate {
			log.V(1).Info("Secret " + secretName + " needs update")

			// These updates alone do not trigger a rolling update due to env var update as it's from a secret
			err = resources.Update(namespacedName, client, secretDefinition)

			// Force the rolling update to occur
			environments.IncrementTriggeredRollCount(currentStatefulSet.Spec.Template.Spec.Containers)

			//so far it doesn't matter what the value is as long as it's greater than zero
			retVal = statefulSetAcceptorsUpdated
		}
	}

	log.Info("Populating env vars from secret " + secretName)
	for envVarName := range *envVars {
		acceptorsEnvVarSource := &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key:      envVarName,
				Optional: nil,
			},
		}

		envVarDefinition := &corev1.EnvVar{
			Name:      envVarName,
			Value:     "",
			ValueFrom: acceptorsEnvVarSource,
		}
		if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, envVarName); nil == retrievedEnvVar {
			log.V(1).Info("sourceEnvVarFromSecret failed to retrieve " + envVarName + " creating")
			environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, envVarDefinition)
			retVal = statefulSetAcceptorsUpdated
		} else {
			log.V(1).Info("sourceEnvVarFromSecret retrieved " + envVarName)
		}
	}

	return retVal
}

func generateAcceptorsString(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client) string {

	// TODO: Optimize for the single broker configuration
	ensureCOREOn61616Exists := true // as clustered is no longer an option but true by default

	acceptorEntry := ""
	defaultArgs := "tcpSendBufferSize=1048576;tcpReceiveBufferSize=1048576;useEpoll=true;amqpCredits=1000;amqpMinCredits=300"

	var portIncrement int32 = 10
	var currentPortIncrement int32 = 0
	var port61616InUse bool = false
	for _, acceptor := range customResource.Spec.Acceptors {
		if 0 == acceptor.Port {
			acceptor.Port = 61626 + currentPortIncrement
			currentPortIncrement += portIncrement
		}
		if "" == acceptor.Protocols ||
			"all" == strings.ToLower(acceptor.Protocols) {
			acceptor.Protocols = "AMQP,CORE,HORNETQ,MQTT,OPENWIRE,STOMP"
		}
		acceptorEntry = acceptorEntry + "<acceptor name=\"" + acceptor.Name + "\">"
		acceptorEntry = acceptorEntry + "tcp:" + "\\/\\/" + "ACCEPTOR_IP:"
		acceptorEntry = acceptorEntry + fmt.Sprintf("%d", acceptor.Port)
		acceptorEntry = acceptorEntry + "?protocols=" + strings.ToUpper(acceptor.Protocols)
		// TODO: Evaluate more dynamic messageMigration
		if 61616 == acceptor.Port {
			port61616InUse = true
		}
		if ensureCOREOn61616Exists &&
			(61616 == acceptor.Port) &&
			!strings.Contains(acceptor.Protocols, "CORE") {
			acceptorEntry = acceptorEntry + ",CORE"
		}
		if acceptor.SSLEnabled {
			secretName := customResource.Name + "-" + acceptor.Name + "-secret"
			if "" != acceptor.SSLSecret {
				secretName = acceptor.SSLSecret
			}
			acceptorEntry = acceptorEntry + ";" + generateAcceptorConnectorSSLArguments(customResource, client, secretName)
			sslOptionalArguments := generateAcceptorSSLOptionalArguments(acceptor)
			if "" != sslOptionalArguments {
				acceptorEntry = acceptorEntry + ";" + sslOptionalArguments
			}
		}
		if "" != acceptor.AnycastPrefix {
			safeAnycastPrefix := strings.Replace(acceptor.AnycastPrefix, "/", "\\/", -1)
			acceptorEntry = acceptorEntry + ";" + "anycastPrefix=" + safeAnycastPrefix
		}
		if "" != acceptor.MulticastPrefix {
			safeMulticastPrefix := strings.Replace(acceptor.MulticastPrefix, "/", "\\/", -1)
			acceptorEntry = acceptorEntry + ";" + "multicastPrefix=" + safeMulticastPrefix
		}
		if acceptor.ConnectionsAllowed > 0 {
			acceptorEntry = acceptorEntry + ";" + "connectionsAllowed=" + fmt.Sprintf("%d", acceptor.ConnectionsAllowed)
		}
		if acceptor.AMQPMinLargeMessageSize > 0 {
			acceptorEntry = acceptorEntry + ";" + "amqpMinLargeMessageSize=" + fmt.Sprintf("%d", acceptor.AMQPMinLargeMessageSize)
		}
		acceptorEntry = acceptorEntry + ";" + defaultArgs

		acceptorEntry = acceptorEntry + "<\\/acceptor>"
	}
	// TODO: Evaluate more dynamic messageMigration
	if ensureCOREOn61616Exists && !port61616InUse {
		acceptorEntry = acceptorEntry + "<acceptor name=\"" + "scaleDown" + "\">"
		acceptorEntry = acceptorEntry + "tcp:" + "\\/\\/" + "ACCEPTOR_IP:"
		acceptorEntry = acceptorEntry + fmt.Sprintf("%d", 61616)
		acceptorEntry = acceptorEntry + "?protocols=" + "CORE"
		acceptorEntry = acceptorEntry + ";" + defaultArgs
		// TODO: SSL
		acceptorEntry = acceptorEntry + "<\\/acceptor>"
	}

	return acceptorEntry
}

func generateConnectorsString(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client) string {

	connectorEntry := ""
	connectors := customResource.Spec.Connectors
	for _, connector := range connectors {
		if connector.Type == "" {
			connector.Type = "tcp"
		}
		connectorEntry = connectorEntry + "<connector name=\"" + connector.Name + "\">"
		connectorEntry = connectorEntry + strings.ToLower(connector.Type) + ":\\/\\/" + strings.ToLower(connector.Host) + ":"
		connectorEntry = connectorEntry + fmt.Sprintf("%d", connector.Port)

		if connector.SSLEnabled {
			secretName := customResource.Name + "-" + connector.Name + "-secret"
			if "" != connector.SSLSecret {
				secretName = connector.SSLSecret
			}
			connectorEntry = connectorEntry + ";" + generateAcceptorConnectorSSLArguments(customResource, client, secretName)
			sslOptionalArguments := generateConnectorSSLOptionalArguments(connector)
			if "" != sslOptionalArguments {
				connectorEntry = connectorEntry + ";" + sslOptionalArguments
			}
		}
		connectorEntry = connectorEntry + "<\\/connector>"
	}

	return connectorEntry
}

func configureAcceptorsExposure(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	for ; i < customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = statefulsets.NameBuilder.Name() + "-" + ordinalString

		for _, acceptor := range customResource.Spec.Acceptors {
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, acceptor.Name+"-"+ordinalString, acceptor.Port, serviceRoutelabels)
			serviceNamespacedName := types.NamespacedName{
				Name:      serviceDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if acceptor.Expose {
				requestedResources = append(requestedResources, serviceDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			}
			targetPortName := acceptor.Name + "-" + ordinalString
			targetServiceName := customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, acceptor.SSLEnabled)
			routeNamespacedName := types.NamespacedName{
				Name:      routeDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if acceptor.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConnectorsExposure(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	for ; i < customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = statefulsets.NameBuilder.Name() + "-" + ordinalString

		for _, connector := range customResource.Spec.Connectors {
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, connector.Name+"-"+ordinalString, connector.Port, serviceRoutelabels)

			serviceNamespacedName := types.NamespacedName{
				Name:      serviceDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if connector.Expose {
				requestedResources = append(requestedResources, serviceDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			}
			targetPortName := connector.Name + "-" + ordinalString
			targetServiceName := customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, connector.SSLEnabled)

			routeNamespacedName := types.NamespacedName{
				Name:      routeDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if connector.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConsoleExposure(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false
	console := customResource.Spec.Console

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	for ; i < customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = statefulsets.NameBuilder.Name() + "-" + ordinalString

		portNumber := int32(8161)
		targetPortName := "wconsj" + "-" + ordinalString
		targetServiceName := customResource.Name + "-" + targetPortName + "-svc"

		serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, targetPortName, portNumber, serviceRoutelabels)

		serviceNamespacedName := types.NamespacedName{
			Name:      serviceDefinition.Name,
			Namespace: customResource.Namespace,
		}
		if console.Expose {
			requestedResources = append(requestedResources, serviceDefinition)
			//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
		} else {
			causedUpdate, err = resources.Disable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
		}
		var err error = nil
		isOpenshift := false

		if isOpenshift, err = environments.DetectOpenshift(); err != nil {
			log.Error(err, "Failed to get env, will try kubernetes")
		}
		if isOpenshift {
			log.Info("Environment is OpenShift")
			log.Info("Checking routeDefinition for " + targetPortName)
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled)
			routeNamespacedName := types.NamespacedName{
				Name:      routeDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if console.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		} else {
			log.Info("Environment is not OpenShift, creating ingress")
			ingressDefinition := ingresses.NewIngressForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName)
			ingressNamespacedName := types.NamespacedName{
				Name:      ingressDefinition.Name,
				Namespace: customResource.Namespace,
			}
			if console.Expose {
				requestedResources = append(requestedResources, ingressDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, ingressNamespacedName, ingressDefinition)
			} else {
				causedUpdate, err = resources.Disable(customResource, client, scheme, ingressNamespacedName, ingressDefinition)
			}
		}
	}

	return causedUpdate, err
}

func generateConsoleSSLFlags(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, secretName string) string {

	sslFlags := ""
	secretNamespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: customResource.Namespace,
	}
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	stringDataMap := map[string]string{}
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap)

	keyStorePassword := "password"
	keyStorePath := "/etc/" + secretName + "-volume/broker.ks"
	trustStorePassword := "password"
	trustStorePath := "/etc/" + secretName + "-volume/client.ts"
	if err := resources.Retrieve(secretNamespacedName, client, userPasswordSecret); err == nil {
		if "" != string(userPasswordSecret.Data["keyStorePassword"]) {
			keyStorePassword = string(userPasswordSecret.Data["keyStorePassword"])
		}
		if "" != string(userPasswordSecret.Data["keyStorePath"]) {
			keyStorePath = string(userPasswordSecret.Data["keyStorePath"])
		}
		if "" != string(userPasswordSecret.Data["trustStorePassword"]) {
			trustStorePassword = string(userPasswordSecret.Data["trustStorePassword"])
		}
		if "" != string(userPasswordSecret.Data["trustStorePath"]) {
			trustStorePath = string(userPasswordSecret.Data["trustStorePath"])
		}
	}

	sslFlags = sslFlags + " " + "--ssl-key" + " " + keyStorePath
	sslFlags = sslFlags + " " + "--ssl-key-password" + " " + keyStorePassword
	sslFlags = sslFlags + " " + "--ssl-trust" + " " + trustStorePath
	sslFlags = sslFlags + " " + "--ssl-trust-password" + " " + trustStorePassword
	if customResource.Spec.Console.UseClientAuth {
		sslFlags = sslFlags + " " + "--use-client-auth"
	}

	return sslFlags
}

func generateAcceptorConnectorSSLArguments(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, secretName string) string {

	sslArguments := "sslEnabled=true"
	secretNamespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: customResource.Namespace,
	}
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	stringDataMap := map[string]string{}
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap)

	keyStorePassword := "password"
	keyStorePath := "\\/etc\\/" + secretName + "-volume\\/broker.ks"
	trustStorePassword := "password"
	trustStorePath := "\\/etc\\/" + secretName + "-volume\\/client.ts"
	if err := resources.Retrieve(secretNamespacedName, client, userPasswordSecret); err == nil {
		if "" != string(userPasswordSecret.Data["keyStorePassword"]) {
			//noinspection GoUnresolvedReference
			keyStorePassword = strings.ReplaceAll(string(userPasswordSecret.Data["keyStorePassword"]), "/", "\\/")
		}
		if "" != string(userPasswordSecret.Data["keyStorePath"]) {
			//noinspection GoUnresolvedReference
			keyStorePath = strings.ReplaceAll(string(userPasswordSecret.Data["keyStorePath"]), "/", "\\/")
		}
		if "" != string(userPasswordSecret.Data["trustStorePassword"]) {
			//noinspection GoUnresolvedReference
			trustStorePassword = strings.ReplaceAll(string(userPasswordSecret.Data["trustStorePassword"]), "/", "\\/")
		}
		if "" != string(userPasswordSecret.Data["trustStorePath"]) {
			//noinspection GoUnresolvedReference
			trustStorePath = strings.ReplaceAll(string(userPasswordSecret.Data["trustStorePath"]), "/", "\\/")
		}
	}
	sslArguments = sslArguments + ";" + "keyStorePath=" + keyStorePath
	sslArguments = sslArguments + ";" + "keyStorePassword=" + keyStorePassword
	sslArguments = sslArguments + ";" + "trustStorePath=" + trustStorePath
	sslArguments = sslArguments + ";" + "trustStorePassword=" + trustStorePassword

	return sslArguments
}

func generateAcceptorSSLOptionalArguments(acceptor brokerv2alpha3.AcceptorType) string {

	sslOptionalArguments := ""

	if "" != acceptor.EnabledCipherSuites {
		sslOptionalArguments = sslOptionalArguments + "enabledCipherSuites=" + acceptor.EnabledCipherSuites
	}
	if "" != acceptor.EnabledProtocols {
		sslOptionalArguments = sslOptionalArguments + ";" + "enabledProtocols=" + acceptor.EnabledProtocols
	}
	if acceptor.NeedClientAuth {
		sslOptionalArguments = sslOptionalArguments + ";" + "needClientAuth=true"
	}
	if acceptor.WantClientAuth {
		sslOptionalArguments = sslOptionalArguments + ";" + "wantClientAuth=true"
	}
	if acceptor.VerifyHost {
		sslOptionalArguments = sslOptionalArguments + ";" + "verifyHost=true"
	}
	if "" != acceptor.SSLProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "sslProvider=" + acceptor.SSLProvider
	}
	if "" != acceptor.SNIHost {
		sslOptionalArguments = sslOptionalArguments + ";" + "sniHost=" + acceptor.SNIHost
	}

	return sslOptionalArguments
}

func generateConnectorSSLOptionalArguments(connector brokerv2alpha3.ConnectorType) string {

	sslOptionalArguments := ""

	if "" != connector.EnabledCipherSuites {
		sslOptionalArguments = sslOptionalArguments + "enabledCipherSuites=" + connector.EnabledCipherSuites
	}
	if "" != connector.EnabledProtocols {
		sslOptionalArguments = sslOptionalArguments + ";" + "enabledProtocols=" + connector.EnabledProtocols
	}
	if connector.NeedClientAuth {
		sslOptionalArguments = sslOptionalArguments + ";" + "needClientAuth=true"
	}
	if connector.WantClientAuth {
		sslOptionalArguments = sslOptionalArguments + ";" + "wantClientAuth=true"
	}
	if connector.VerifyHost {
		sslOptionalArguments = sslOptionalArguments + ";" + "verifyHost=true"
	}
	if "" != connector.SSLProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "sslProvider=" + connector.SSLProvider
	}
	if "" != connector.SNIHost {
		sslOptionalArguments = sslOptionalArguments + ";" + "sniHost=" + connector.SNIHost
	}

	return sslOptionalArguments
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []corev1.EnvVar, i int) []corev1.EnvVar {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

func aioSyncCausedUpdateOn(deploymentPlan *brokerv2alpha3.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	foundAio := false
	foundNio := false
	var extraArgs string = ""
	extraArgsNeedsUpdate := false

	// Find the existing values
	for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
		if v.Name == "AMQ_JOURNAL_TYPE" {
			if strings.Index(v.Value, "aio") > -1 {
				foundAio = true
			}
			if strings.Index(v.Value, "nio") > -1 {
				foundNio = true
			}
			extraArgs = v.Value
			break
		}
	}

	if "aio" == strings.ToLower(deploymentPlan.JournalType) && foundNio {
		extraArgs = strings.Replace(extraArgs, "nio", "aio", 1)
		extraArgsNeedsUpdate = true
	}

	if !("aio" == strings.ToLower(deploymentPlan.JournalType)) && foundAio {
		extraArgs = strings.Replace(extraArgs, "aio", "nio", 1)
		extraArgsNeedsUpdate = true
	}

	if !foundAio && !foundNio {
		extraArgs = "--" + strings.ToLower(deploymentPlan.JournalType)
		extraArgsNeedsUpdate = true
	}

	if extraArgsNeedsUpdate {
		newExtraArgsValue := corev1.EnvVar{
			"AMQ_JOURNAL_TYPE",
			extraArgs,
			nil,
		}
		environments.Update(currentStatefulSet.Spec.Template.Spec.Containers, &newExtraArgsValue)
	}

	return extraArgsNeedsUpdate
}

func persistentSyncCausedUpdateOn(deploymentPlan *brokerv2alpha3.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	foundDataDir := false
	foundDataDirLogging := false

	dataDirNeedsUpdate := false
	dataDirLoggingNeedsUpdate := false

	statefulSetUpdated := false

	// TODO: Remove yuck
	// ensure password and username are valid if can't via openapi validation?
	if deploymentPlan.PersistenceEnabled {

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_DATA_DIR" {
				foundDataDir = true
				if v.Value != volumes.GLOBAL_DATA_PATH {
					dataDirNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_DATA_DIR_LOGGING" {
				foundDataDirLogging = true
				if v.Value != "true" {
					dataDirLoggingNeedsUpdate = true
				}
			}
		}

		if !foundDataDir || dataDirNeedsUpdate {
			newDataDirValue := corev1.EnvVar{
				"AMQ_DATA_DIR",
				volumes.GLOBAL_DATA_PATH,
				nil,
			}
			envVarArray = append(envVarArray, newDataDirValue)
			statefulSetUpdated = true
		}

		if !foundDataDirLogging || dataDirLoggingNeedsUpdate {
			newDataDirLoggingValue := corev1.EnvVar{
				"AMQ_DATA_DIR_LOGGING",
				"true",
				nil,
			}
			envVarArray = append(envVarArray, newDataDirLoggingValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
				for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
					for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
						if ("AMQ_DATA_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && dataDirNeedsUpdate) ||
							("AMQ_DATA_DIR_LOGGING" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && dataDirLoggingNeedsUpdate) {
							currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
						}
					}
				}

				containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
				for i := 0; i < containerArrayLen; i++ {
					for j := 0; j < envVarArrayLen; j++ {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
					}
				}
			}
		}
	} else {

		for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
			for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
				if "AMQ_DATA_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_DATA_DIR_LOGGING" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					statefulSetUpdated = true
				}
			}
		}
	}

	return statefulSetUpdated
}

func imageSyncCausedUpdateOn(deploymentPlan *brokerv2alpha3.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	// At implementation time only one container
	if strings.Compare(currentStatefulSet.Spec.Template.Spec.Containers[0].Image, deploymentPlan.Image) != 0 {
		containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
		for i := 0; i < containerArrayLen; i++ {
			currentStatefulSet.Spec.Template.Spec.Containers[i].Image = deploymentPlan.Image
		}
		return true
	}

	return false
}

func (reconciler *ActiveMQArtemisReconciler) ProcessResources(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8 {

	reqLogger := log.WithValues("ActiveMQArtemis Name", customResource.Name)
	reqLogger.Info("Processing resources")

	var err error = nil
	var createError error = nil
	var deployed map[reflect.Type][]resource.KubernetesResource
	var hasUpdates bool
	var stepsComplete uint8 = 0

	added := false
	updated := false
	removed := false

	for index := range requestedResources {
		requestedResources[index].SetNamespace(customResource.Namespace)
	}

	err = reconciler.checkUpgradeVersions(customResource, err, reqLogger)
	deployed, err = getDeployedResources(customResource, client)
	if err != nil {
		reqLogger.Error(err, "error getting deployed resources", "returned", stepsComplete)
		return stepsComplete
	}

	requested := compare.NewMapBuilder().Add(requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()
	deltas := comparator.Compare(deployed, requested)
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	for resourceType, delta := range deltas {
		reqLogger.Info("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			added, stepsComplete = reconciler.createResource(customResource, client, scheme, resourceToAdd, added, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			updated, stepsComplete = reconciler.updateResource(customResource, client, scheme, resourceToUpdate, updated, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			removed, stepsComplete = reconciler.deleteResource(customResource, client, scheme, resourceToRemove, removed, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		hasUpdates = hasUpdates || added || updated || removed
	}

	//empty the collected objects
	requestedResources = nil

	return stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) createResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, added bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, createError error, stepsComplete uint8) (bool, uint8) {

	kind := requested.GetName()
	added = true
	reqLogger.V(1).Info("Adding delta resources, i.e. creating ", "for kind ", kind)
	reqLogger.V(1).Info("last namespacedName.Name was " + namespacedName.Name)
	namespacedName.Name = kind
	reqLogger.V(1).Info("this namespacedName.Name IS " + namespacedName.Name)
	err, createError = reconciler.createRequestedResource(customResource, client, scheme, namespacedName, requested, reqLogger, createError, kind)
	if nil == createError && nil != err {
		switch kind {
		case ss.NameBuilder.Name():
			stepsComplete |= CreatedStatefulSet
			ss.GLOBAL_CRNAME = customResource.Name
		case svc.HeadlessNameBuilder.Name():
			stepsComplete |= CreatedHeadlessService
		case svc.PingNameBuilder.Name():
			stepsComplete |= CreatedPingService
		case secrets.CredentialsNameBuilder.Name():
			stepsComplete |= CreatedCredentialsSecret
		case secrets.NettyNameBuilder.Name():
			stepsComplete |= CreatedNettySecret
		default:
		}
	} else if nil != createError {
		reqLogger.Info("Failed to create resource " + kind + " named " + namespacedName.Name)
	}

	return added, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) updateResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, updated bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, updateError error, stepsComplete uint8) (bool, uint8) {

	kind := requested.GetName()
	updated = true
	reqLogger.V(1).Info("Updating delta resources, i.e. updating ", "for kind ", kind)
	reqLogger.V(1).Info("last namespacedName.Name was " + namespacedName.Name)
	namespacedName.Name = kind
	reqLogger.V(1).Info("this namespacedName.Name IS " + namespacedName.Name)

	err, updateError = reconciler.updateRequestedResource(customResource, client, scheme, namespacedName, requested, reqLogger, updateError, kind)
	if nil == updateError && nil != err {
		//switch kind {
		//case ss.NameBuilder.Name():
		//	//stepsComplete |= CreatedStatefulSet
		//	ss.GLOBAL_CRNAME = customResource.Name
		//case svc.HeadlessNameBuilder.Name():
		//	//stepsComplete |= CreatedHeadlessService
		//case svc.PingNameBuilder.Name():
		//	//stepsComplete |= CreatedPingService
		//case secrets.CredentialsNameBuilder.Name():
		//	//stepsComplete |= CreatedCredentialsSecret
		//case secrets.NettyNameBuilder.Name():
		//	//stepsComplete |= CreatedNettySecret
		//default:
		//}
		reqLogger.V(1).Info("updateResource updated " + kind)
	} else if nil != updateError {
		reqLogger.Info("updateResource Failed to update resource " + kind)
	}

	return updated, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) deleteResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, deleted bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, deleteError error, stepsComplete uint8) (bool, uint8) {

	kind := requested.GetName()
	deleted = true
	reqLogger.V(1).Info("Deleting delta resources, i.e. removing ", "for kind ", kind)
	reqLogger.V(1).Info("last namespacedName.Name was " + namespacedName.Name)
	namespacedName.Name = kind
	reqLogger.V(1).Info("this namespacedName.Name IS " + namespacedName.Name)

	err, deleteError = reconciler.deleteRequestedResource(customResource, client, scheme, namespacedName, requested, reqLogger, deleteError, kind)
	if nil == deleteError && nil != err {
		//switch kind {
		//case ss.NameBuilder.Name():
		//	//stepsComplete |= CreatedStatefulSet
		//	ss.GLOBAL_CRNAME = customResource.Name
		//case svc.HeadlessNameBuilder.Name():
		//	//stepsComplete |= CreatedHeadlessService
		//case svc.PingNameBuilder.Name():
		//	//stepsComplete |= CreatedPingService
		//case secrets.CredentialsNameBuilder.Name():
		//	//stepsComplete |= CreatedCredentialsSecret
		//case secrets.NettyNameBuilder.Name():
		//	//stepsComplete |= CreatedNettySecret
		//default:
		//}
		reqLogger.V(1).Info("deleteResource deleted " + kind)
	} else if nil != deleteError {
		reqLogger.Info("deleteResource Failed to delete resource " + kind)
	}

	return deleted, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) checkUpgradeVersions(customResource *brokerv2alpha3.ActiveMQArtemis, err error, reqLogger logr.Logger) error {
	_, _, err = checkProductUpgrade(customResource)
	//if err != nil {
	//	log.Info("checkProductUpgrade failed")
	//} else {
	//	hasUpdates = true
	//}
	specifiedMinorVersion := getMinorImageVersion(customResource.Spec.Version)
	if customResource.Spec.Upgrades.Enabled && customResource.Spec.Upgrades.Minor {
		imageName, imageTag, imageContext := GetImage(customResource.Spec.DeploymentPlan.Image)
		reqLogger.V(1).Info("Current imageName " + imageName)
		reqLogger.V(1).Info("Current imageTag " + imageTag)
		reqLogger.V(1).Info("Current imageContext " + imageContext)

		imageTagNoDash := strings.Replace(imageTag, "-", ".", -1)
		imageVersionSplitFromTag := strings.Split(imageTagNoDash, ".")
		var currentMinorVersion = ""
		if 3 == len(imageVersionSplitFromTag) {
			currentMinorVersion = imageVersionSplitFromTag[0] + imageVersionSplitFromTag[1]
		}
		reqLogger.V(1).Info("Current minor version " + currentMinorVersion)

		if specifiedMinorVersion != currentMinorVersion {
			// reset current annotations and update CR use to specified product version
			customResource.SetAnnotations(map[string]string{
				brokerv2alpha3.SchemeGroupVersion.Group: FullVersionFromMinorVersion[specifiedMinorVersion]})
			customResource.Spec.Version = FullVersionFromMinorVersion[specifiedMinorVersion]
			upgradeVersionEnvBrokerImage := os.Getenv("BROKER_IMAGE_" + CompactFullVersionFromMinorVersion[specifiedMinorVersion])
			if "" != upgradeVersionEnvBrokerImage {
				customResource.Spec.DeploymentPlan.Image = upgradeVersionEnvBrokerImage
			}

			imageName, imageTag, imageContext = GetImage(customResource.Spec.DeploymentPlan.Image)
			reqLogger.V(1).Info("Updated imageName " + imageName)
			reqLogger.V(1).Info("Updated imageTag " + imageTag)
			reqLogger.V(1).Info("Updated imageContext " + imageContext)
		}
	}
	return err
}

func (reconciler *ActiveMQArtemisReconciler) createRequestedResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, createError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("createResource Failed to Retrieve " + namespacedName.Name)
		if createError = resources.Create(customResource, namespacedName, client, scheme, requested); createError == nil {
			reqLogger.Info("Created kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, createError
}

func (reconciler *ActiveMQArtemisReconciler) updateRequestedResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, updateError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("updateResource Failed to Retrieve " + namespacedName.Name)
		if updateError = resources.Update(namespacedName, client, requested); updateError == nil {
			reqLogger.Info("updated kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, updateError
}

func (reconciler *ActiveMQArtemisReconciler) deleteRequestedResource(customResource *brokerv2alpha3.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, deleteError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("deleteResource Failed to Retrieve " + namespacedName.Name)
		if deleteError = resources.Delete(namespacedName, client, requested); deleteError == nil {
			reqLogger.Info("deleted kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, deleteError
}

func getDeployedResources(instance *brokerv2alpha3.ActiveMQArtemis, client client.Client) (map[reflect.Type][]resource.KubernetesResource, error) {

	var log = logf.Log.WithName("controller_v2alpha3activemqartemis")

	reader := read.New(client).WithNamespace(instance.Namespace).WithOwnerObject(instance)
	var resourceMap map[reflect.Type][]resource.KubernetesResource
	var err error
	if isOpenshift, _ := environments.DetectOpenshift(); isOpenshift {
		resourceMap, err = reader.ListAll(
			&corev1.PersistentVolumeClaimList{},
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&routev1.RouteList{},
			&corev1.SecretList{},
		)
	} else {
		resourceMap, err = reader.ListAll(
			&corev1.PersistentVolumeClaimList{},
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&extv1b1.IngressList{},
			&corev1.SecretList{},
		)
	}
	if err != nil {
		log.Error(err, "Failed to list deployed objects. ")
		return nil, err
	}

	return resourceMap, nil
}

func MakeVolumes(cr *brokerv2alpha3.ActiveMQArtemis) []corev1.Volume {

	volumeDefinitions := []corev1.Volume{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		basicCRVolume := volumes.MakePersistentVolume(cr.Name)
		volumeDefinitions = append(volumeDefinitions, basicCRVolume...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range cr.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		secretName := cr.Name + "-" + acceptor.Name + "-secret"
		if "" != acceptor.SSLSecret {
			secretName = acceptor.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range cr.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		secretName := cr.Name + "-" + connector.Name + "-secret"
		if "" != connector.SSLSecret {
			secretName = connector.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	if cr.Spec.Console.SSLEnabled {
		secretName := secrets.ConsoleNameBuilder.Name()
		if "" != cr.Spec.Console.SSLSecret {
			secretName = cr.Spec.Console.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	return volumeDefinitions
}

func MakeVolumeMounts(cr *brokerv2alpha3.ActiveMQArtemis) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		persistentCRVlMnt := volumes.MakePersistentVolumeMount(cr.Name)
		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range cr.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		volumeMountName := cr.Name + "-" + acceptor.Name + "-secret-volume"
		if "" != acceptor.SSLSecret {
			volumeMountName = acceptor.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range cr.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		volumeMountName := cr.Name + "-" + connector.Name + "-secret-volume"
		if "" != connector.SSLSecret {
			volumeMountName = connector.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	if cr.Spec.Console.SSLEnabled {
		volumeMountName := secrets.ConsoleNameBuilder.Name() + "-volume"
		if "" != cr.Spec.Console.SSLSecret {
			volumeMountName = cr.Spec.Console.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

func NewPodTemplateSpecForCR(customResource *brokerv2alpha3.ActiveMQArtemis) corev1.PodTemplateSpec {

	// Log where we are and what we're doing
	reqLogger := log.WithName(customResource.Name)
	//reqLogger.Info("Creating new pod template spec for custom resource")
	reqLogger.V(1).Info("NewPodTemplateSpecForCR")

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	terminationGracePeriodSeconds := int64(60)

	pts := pods.MakePodTemplateSpec(namespacedName, selectors.LabelBuilder.Labels())
	Spec := corev1.PodSpec{}
	Containers := []corev1.Container{}
	container := containers.MakeContainer(customResource.Name, customResource.Spec.DeploymentPlan.Image, MakeEnvVarArrayForCR(customResource))
	container.Resources = customResource.Spec.DeploymentPlan.Resources

	volumeMounts := MakeVolumeMounts(customResource)
	if len(volumeMounts) > 0 {
		reqLogger.V(1).Info("Adding new mounts to main", "len", len(volumeMounts))
		container.VolumeMounts = volumeMounts
	}
	reqLogger.V(1).Info("now mounts added to container", "new len", len(container.VolumeMounts))

	Spec.Containers = append(Containers, container)
	brokerVolumes := MakeVolumes(customResource)
	if len(brokerVolumes) > 0 {
		Spec.Volumes = brokerVolumes
	}
	Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

	//address settings
	addressSettings := customResource.Spec.AddressSettings.AddressSetting
	if len(addressSettings) > 0 {
		//User has supplied addressSettings
		envVarTuneFilePath := "TUNE_PATH"
		outputDir := "/yacfg_etc"
		envVarApplyRule := "APPLY_RULE"
		envVarApplyRuleValue := customResource.Spec.AddressSettings.ApplyRule

		if envVarApplyRuleValue == nil {
			envVarApplyRuleValue = &defApplyRule
		}
		reqLogger.V(1).Info("Process addresssetting", "ApplyRule", *envVarApplyRuleValue)

		brokerYaml := cr2jinja2v2alpha3.MakeBrokerCfgOverrides(customResource, nil, nil)
		InitContainers := []corev1.Container{
			{
				Name:            "amq-broker-init",
				Image:           "quay.io/artemiscloud/activemq-artemis-broker-init:0.2.1",
				ImagePullPolicy: "Always",
				Command:         []string{"/bin/bash"},
				Args: []string{"-c",
					"echo \"" + brokerYaml + "\" > " + outputDir +
						"/broker.yaml; cat /yacfg_etc/broker.yaml; yacfg --profile artemis/2.15.0/default_with_user_address_settings.yaml.jinja2  --tune " +
						outputDir + "/broker.yaml --output " + outputDir},
				Resources: customResource.Spec.DeploymentPlan.Resources,
			},
		}

		Spec.InitContainers = InitContainers
		//create a volumeMount for both init-container and main container
		volumeMountForCfg := volumes.MakeVolumeMountForCfg("tool-dir", outputDir)
		Spec.Containers[0].VolumeMounts = append(Spec.Containers[0].VolumeMounts, volumeMountForCfg)
		InitContainers[0].VolumeMounts = append(InitContainers[0].VolumeMounts, volumeMountForCfg)

		//pass cfg file location and apply rule to main container via env vars
		tuneFile := corev1.EnvVar{
			Name:  envVarTuneFilePath,
			Value: outputDir,
		}
		environments.Create(Spec.Containers, &tuneFile)

		applyRule := corev1.EnvVar{
			Name:  envVarApplyRule,
			Value: *envVarApplyRuleValue,
		}
		environments.Create(Spec.Containers, &applyRule)

		//add empty-dir volume
		volumeForCfg := volumes.MakeVolumeForCfg("tool-dir")
		Spec.Volumes = append(Spec.Volumes, volumeForCfg)
	}
	pts.Spec = Spec

	return pts
}

func NewStatefulSetForCR(cr *brokerv2alpha3.ActiveMQArtemis) *appsv1.StatefulSet {

	// Log where we are and what we're doing
	reqLogger := log.WithName(cr.Name)
	reqLogger.V(1).Info("NewStatefulSetForCR")

	namespacedName := types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}
	ss, Spec := statefulsets.MakeStatefulSet(namespacedName, cr.Annotations, cr.Spec.DeploymentPlan.Size, NewPodTemplateSpecForCR(cr))

	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		Spec.VolumeClaimTemplates = *NewPersistentVolumeClaimArrayForCR(cr, 1)
	}
	ss.Spec = Spec

	return ss
}

func NewPersistentVolumeClaimArrayForCR(cr *brokerv2alpha3.ActiveMQArtemis, arrayLength int) *[]corev1.PersistentVolumeClaim {

	var pvc *corev1.PersistentVolumeClaim = nil
	capacity := "2Gi"
	pvcArray := make([]corev1.PersistentVolumeClaim, 0, arrayLength)

	namespacedName := types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}

	if "" != cr.Spec.DeploymentPlan.Storage.Size {
		capacity = cr.Spec.DeploymentPlan.Storage.Size
	}

	for i := 0; i < arrayLength; i++ {
		pvc = persistentvolumeclaims.NewPersistentVolumeClaimWithCapacity(namespacedName, capacity)
		pvcArray = append(pvcArray, *pvc)
	}

	return &pvcArray
}

// TODO: Test namespacedName to ensure it's the right namespacedName
func UpdatePodStatus(cr *brokerv2alpha3.ActiveMQArtemis, client client.Client, ssNamespacedName types.NamespacedName) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.V(1).Info("Updating status for pods")

	podStatus := GetPodStatus(cr, client, ssNamespacedName)

	reqLogger.V(1).Info("PodStatus are to be updated.............................", "info:", podStatus)
	reqLogger.V(1).Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.V(1).Info("Stopped Count........................", "info:", len(podStatus.Stopped))
	reqLogger.V(1).Info("Starting Count........................", "info:", len(podStatus.Starting))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		cr.Status.PodStatus = podStatus

		err := client.Status().Update(context.TODO(), cr)
		if err != nil {
			reqLogger.Error(err, "Failed to update pods status")
			return err
		}
		reqLogger.Info("Pods status updated")
		return nil
	}

	return nil
}

func GetPodStatus(cr *brokerv2alpha3.ActiveMQArtemis, client client.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {

	reqLogger := log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.V(1).Info("Getting status for pods")

	var status olm.DeploymentStatus

	sfsFound := &appsv1.StatefulSet{}

	err := client.Get(context.TODO(), namespacedName, sfsFound)
	if err == nil {
		status = olm.GetSingleStatefulSetStatus(*sfsFound)
	} else {
		dsFound := &appsv1.DaemonSet{}
		err = client.Get(context.TODO(), namespacedName, dsFound)
		if err == nil {
			status = olm.GetSingleDaemonSetStatus(*dsFound)
		}
	}

	// TODO: Remove global usage
	reqLogger.V(1).Info("lastStatus.Ready len is " + string(len(lastStatus.Ready)))
	reqLogger.V(1).Info("status.Ready len is " + string(len(status.Ready)))
	if len(status.Ready) > len(lastStatus.Ready) {
		// More pods ready, let the address controller know
		newPodCount := len(status.Ready) - len(lastStatus.Ready)
		for i := newPodCount - 1; i < len(status.Ready); i++ {
			v2alpha2activemqartemisaddress.C <- types.NamespacedName{namespacedName.Namespace, status.Ready[i]}
		}
	}
	lastStatus = status

	return status
}

func MakeEnvVarArrayForCR(cr *brokerv2alpha3.ActiveMQArtemis) []corev1.EnvVar {

	reqLogger := log.WithName(cr.Name)
	reqLogger.V(1).Info("Adding Env variable ")

	requireLogin := "false"
	if cr.Spec.DeploymentPlan.RequireLogin {
		requireLogin = "true"
	} else {
		requireLogin = "false"
	}

	journalType := "aio"
	if "aio" == strings.ToLower(cr.Spec.DeploymentPlan.JournalType) {
		journalType = "aio"
	} else {
		journalType = "nio"
	}

	envVar := []corev1.EnvVar{}
	envVarArrayForBasic := environments.AddEnvVarForBasic(requireLogin, journalType)
	envVar = append(envVar, envVarArrayForBasic...)
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		envVarArrayForPresistent := environments.AddEnvVarForPersistent(cr.Name)
		envVar = append(envVar, envVarArrayForPresistent...)
	}

	// TODO: Optimize for the single broker configuration
	envVarArrayForCluster := environments.AddEnvVarForCluster()
	envVar = append(envVar, envVarArrayForCluster...)

	return envVar
}
