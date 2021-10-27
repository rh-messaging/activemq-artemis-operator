package v2alpha5activemqartemis

import (
	"context"
	"encoding/json"
	"fmt"
	osruntime "runtime"

	"github.com/RHsyseng/operator-utils/pkg/olm"
	"github.com/RHsyseng/operator-utils/pkg/resource"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	"github.com/RHsyseng/operator-utils/pkg/resource/read"
	activemqartemisscaledown "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha1/activemqartemisscaledown"
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
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/channels"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/config"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha5"
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
	statefulSetInitImageUpdated  = 1 << 11
)

var defaultMessageMigration bool = true
var requestedResources []resource.KubernetesResource
var lastStatus olm.DeploymentStatus

// the helper script looks for "/amq/scripts/post-config.sh"
// and run it if exists.
var initHelperScript = "/opt/amq-broker/script/default.sh"
var brokerConfigRoot = "/amq/init/config"
var configCmd = "/opt/amq/bin/launch.sh"

//default ApplyRule for address-settings
var defApplyRule string = "merge_all"
var yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.LatestVersion]

type ActiveMQArtemisReconciler struct {
	statefulSetUpdates uint32
}

type ValueInfo struct {
	Value   string
	AutoGen bool
}

type ActiveMQArtemisIReconciler interface {
	Process(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, firstTime bool) uint32
	ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client client.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool)
	ProcessCredentials(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessDeploymentPlan(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32
	ProcessAcceptorsAndConnectors(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessConsole(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet)
	ProcessResources(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8
	ProcessAddressSettings(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client) bool
}

func (reconciler *ActiveMQArtemisReconciler) Process(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, firstTime bool) (uint32, uint8) {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")
	log.Info("Reconciler Processing...", "Operator version", version.Version, "ActiveMQArtemis release", fsm.customResource.Spec.Version)

	currentStatefulSet, firstTime := reconciler.ProcessStatefulSet(fsm, client, log, firstTime)
	statefulSetUpdates := reconciler.ProcessDeploymentPlan(fsm, client, scheme, currentStatefulSet, firstTime)
	statefulSetUpdates |= reconciler.ProcessCredentials(fsm, client, scheme, currentStatefulSet)
	statefulSetUpdates |= reconciler.ProcessAcceptorsAndConnectors(fsm, client, scheme, currentStatefulSet)
	statefulSetUpdates |= reconciler.ProcessConsole(fsm, client, scheme, currentStatefulSet)

	requestedResources = append(requestedResources, currentStatefulSet)
	stepsComplete := reconciler.ProcessResources(fsm, client, scheme, currentStatefulSet)

	if statefulSetUpdates > 0 {
		ssNamespacedName := fsm.GetStatefulSetNamespacedName()
		if err := resources.Update(ssNamespacedName, client, currentStatefulSet); err != nil {
			log.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
		}
	}

	return statefulSetUpdates, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client client.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool) {

	statefulsetRecreationRequired := false

	ssNamespacedName := fsm.GetStatefulSetNamespacedName()

	currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, client)
	if errors.IsNotFound(err) {
		log.Info("StatefulSet: " + ssNamespacedName.Name + " not found, will create")
		currentStatefulSet = NewStatefulSetForCR(fsm)
		firstTime = true
	} else if nil == err {
		// Found it
		log.Info("StatefulSet: " + ssNamespacedName.Name + " found")
		log.Info("Checking for statefulset and current operator compatibility")
		log.V(1).Info("Checking owner apiVersion")
		objectMetadata := currentStatefulSet.GetObjectMeta()
		log.V(1).Info(fmt.Sprintf("ObjectMetadata: %s", objectMetadata))
		ownerReferenceArray := objectMetadata.GetOwnerReferences()
		log.V(1).Info(fmt.Sprintf("ownerReferenceArray: %s", ownerReferenceArray))
		if 0 < len(ownerReferenceArray) {
			// got at least one owner
			log.V(1).Info("ownerReferenceArray has at least one owner")
			log.V(1).Info(fmt.Sprintf("ownerReference[0].APIVersion: %s", ownerReferenceArray[0].APIVersion))
			if "broker.amq.io/v2alpha5" != ownerReferenceArray[0].APIVersion {
				// nuke it and recreate
				log.V(1).Info(fmt.Sprintf("ownerReference[0].APIVersion: %s - removing in favour of upgraded v2alpha5", ownerReferenceArray[0].APIVersion))
				log.Info("Statefulset recreation required for current operator compatibility")
				statefulsetRecreationRequired = true
			}
		}
		log.V(1).Info("Checking statefulset for CONFIG_INSTANCE_DIR existence")
		configInstanceDirEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "CONFIG_INSTANCE_DIR")
		if nil == configInstanceDirEnvVar {
			log.Info("CONFIG_INSTANCE_DIR environment variable NOT found, ensuring existing statefulset operator compatibility")
			log.Info("Statefulset recreation required for current operator compatibility")
			statefulsetRecreationRequired = true
		}
		if !statefulsetRecreationRequired {
			enableMetricsPluginEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_ENABLE_METRICS_PLUGIN")
			enableMetricsPluginInCr := fsm.customResource.Spec.DeploymentPlan.EnableMetricsPlugin
			if enableMetricsPluginEnvVar == nil || enableMetricsPluginEnvVar.Value == "false" {
				if enableMetricsPluginInCr != nil && *enableMetricsPluginInCr == true {
					log.Info("Need recreate statefuleset to update enableMetricsPlugin to true")
					statefulsetRecreationRequired = true
				}
			} else {
				if enableMetricsPluginInCr == nil || *enableMetricsPluginInCr == false {
					log.Info("Need recreate statefuleset to update enableMetricsPlugin to false")
					statefulsetRecreationRequired = true
				}
			}
		}
		if statefulsetRecreationRequired {
			log.Info("Recreating existing statefulset")
			deleteErr := resources.Delete(ssNamespacedName, client, currentStatefulSet)
			if nil == deleteErr {
				log.Info(fmt.Sprintf("sucessfully deleted ownerReference[0].APIVersion: %s, recreating v2alpha5 version for use", ownerReferenceArray[0].APIVersion))
				currentStatefulSet = NewStatefulSetForCR(fsm)
				firstTime = true
			} else {
				log.Info("statefulset recreation failed!")
			}
		}

		newPodTemplateCreated := false
		//update statefulset with customer resource
		log.Info("Calling ProcessAddressSettings")
		if reconciler.ProcessAddressSettings(fsm.customResource, fsm.prevCustomResource, client) {
			log.Info("There are new address settings change in the cr, creating a new pod template to update")
			*fsm.prevCustomResource = *fsm.customResource
			currentStatefulSet.Spec.Template = NewPodTemplateSpecForCR(fsm)
			newPodTemplateCreated = true
		}

		podInvalid := fsm.GetPodInvalid()
		if podInvalid && !newPodTemplateCreated {
			log.Info("Updating the pod template for ss as is marked invalid")
			currentStatefulSet.Spec.Template = NewPodTemplateSpecForCR(fsm)
			fsm.SetPodInvalid(false)
			newPodTemplateCreated = true
		}
	}

	headlessServiceDefinition := svc.NewHeadlessServiceForCR2(fsm.GetHeadlessServiceName(), ssNamespacedName, serviceports.GetDefaultPorts())
	labels := selectors.LabelBuilder.Labels()
	if isClustered(fsm.customResource) {
		pingServiceDefinition := svc.NewPingServiceDefinitionForCR2(fsm.GetPingServiceName(), ssNamespacedName, labels, labels)
		requestedResources = append(requestedResources, pingServiceDefinition)
	}
	requestedResources = append(requestedResources, headlessServiceDefinition)

	return currentStatefulSet, firstTime
}

func isClustered(customResource *brokerv2alpha5.ActiveMQArtemis) bool {
	if customResource.Spec.DeploymentPlan.Clustered != nil {
		return *customResource.Spec.DeploymentPlan.Clustered
	}
	return true
}

func (reconciler *ActiveMQArtemisReconciler) ProcessCredentials(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")
	log.V(1).Info("ProcessCredentials")

	envVars := make(map[string]ValueInfo)

	adminUser := ValueInfo{
		"",
		false,
	}
	adminPassword := ValueInfo{
		"",
		false,
	}
	// TODO: Remove singular admin level user and password in favour of at least guest and admin access
	secretName := fsm.GetCredentialsSecretName()
	envVarName1 := "AMQ_USER"
	for {
		adminUser.Value = fsm.customResource.Spec.AdminUser
		if "" != adminUser.Value {
			break
		}

		if amqUserEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_USER"); nil != amqUserEnvVar {
			adminUser.Value = amqUserEnvVar.Value
		}
		if "" != adminUser.Value {
			break
		}

		adminUser.Value = environments.Defaults.AMQ_USER
		adminUser.AutoGen = true
		break
	} // do once
	envVars[envVarName1] = adminUser

	envVarName2 := "AMQ_PASSWORD"
	for {
		adminPassword.Value = fsm.customResource.Spec.AdminPassword
		if "" != adminPassword.Value {
			break
		}

		if amqPasswordEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_PASSWORD"); nil != amqPasswordEnvVar {
			adminPassword.Value = amqPasswordEnvVar.Value
		}
		if "" != adminPassword.Value {
			break
		}

		adminPassword.Value = environments.Defaults.AMQ_PASSWORD
		adminPassword.AutoGen = true
		break
	} // do once
	envVars[envVarName2] = adminPassword

	envVars["AMQ_CLUSTER_USER"] = ValueInfo{
		Value:   environments.GLOBAL_AMQ_CLUSTER_USER,
		AutoGen: true,
	}
	envVars["AMQ_CLUSTER_PASSWORD"] = ValueInfo{
		Value:   environments.GLOBAL_AMQ_CLUSTER_PASSWORD,
		AutoGen: true,
	}
	statefulSetUpdates := sourceEnvVarFromSecret2(fsm.customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) ProcessDeploymentPlan(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32 {

	deploymentPlan := &fsm.customResource.Spec.DeploymentPlan

	log.Info("Processing deployment plan", "plan", deploymentPlan, "broker cr", fsm.customResource.Name)
	// Ensure the StatefulSet size is the same as the spec
	if *currentStatefulSet.Spec.Replicas != deploymentPlan.Size {
		currentStatefulSet.Spec.Replicas = &deploymentPlan.Size
		reconciler.statefulSetUpdates |= statefulSetSizeUpdated
	}

	if initImageSyncCausedUpdateOn(fsm.customResource, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetInitImageUpdated
	}

	if imageSyncCausedUpdateOn(fsm.customResource, currentStatefulSet) {
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

	if clusterSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		log.Info("Clustered is false")
		reconciler.statefulSetUpdates |= statefulSetClusterConfigUpdated
	}

	log.Info("Now sync Message migration", "for cr", fsm.customResource.Name)

	syncMessageMigration(fsm.customResource, client, scheme)

	return reconciler.statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) ProcessAddressSettings(customResource *brokerv2alpha5.ActiveMQArtemis, prevCustomResource *brokerv2alpha5.ActiveMQArtemis, client client.Client) bool {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")
	log.Info("Process addresssettings")

	if len(customResource.Spec.AddressSettings.AddressSetting) == 0 {
		return false
	}

	//we need to compare old with new and update if they are different.
	return compareAddressSettings(&prevCustomResource.Spec.AddressSettings, &customResource.Spec.AddressSettings)
}

//returns true if currentAddressSettings need update
func compareAddressSettings(currentAddressSettings *brokerv2alpha5.AddressSettingsType, newAddressSettings *brokerv2alpha5.AddressSettingsType) bool {

	if (*currentAddressSettings).ApplyRule == nil {
		if (*newAddressSettings).ApplyRule != nil {
			return true
		}
	} else {
		if (*newAddressSettings).ApplyRule != nil {
			if *(*currentAddressSettings).ApplyRule != *(*newAddressSettings).ApplyRule {
				return true
			}
		} else {
			return true
		}
	}
	if len((*currentAddressSettings).AddressSetting) != len((*newAddressSettings).AddressSetting) || !config.IsEqualV2Alpha5((*currentAddressSettings).AddressSetting, (*newAddressSettings).AddressSetting) {
		return true
	}
	return false
}

func (reconciler *ActiveMQArtemisReconciler) ProcessAcceptorsAndConnectors(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var retVal uint32 = statefulSetNotUpdated

	acceptorEntry := generateAcceptorsString(fsm.customResource, client)
	connectorEntry := generateConnectorsString(fsm.customResource, client)

	configureAcceptorsExposure(fsm, client, scheme)
	configureConnectorsExposure(fsm, client, scheme)

	envVars := map[string]string{
		"AMQ_ACCEPTORS":  acceptorEntry,
		"AMQ_CONNECTORS": connectorEntry,
	}
	secretName := fsm.GetNettySecretName()
	retVal = sourceEnvVarFromSecret(fsm.customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func (reconciler *ActiveMQArtemisReconciler) ProcessConsole(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var retVal uint32 = statefulSetNotUpdated

	configureConsoleExposure(fsm, client, scheme)
	if !fsm.customResource.Spec.Console.SSLEnabled {
		return retVal
	}

	isOpenshift, _ := environments.DetectOpenshift()
	if !isOpenshift && fsm.customResource.Spec.Console.Expose {
		//if it is kubernetes the tls termination at ingress point
		//so the console shouldn't be secured.
		return retVal
	}

	sslFlags := ""
	envVarName := "AMQ_CONSOLE_ARGS"
	secretName := fsm.GetConsoleSecretName()
	if "" != fsm.customResource.Spec.Console.SSLSecret {
		secretName = fsm.customResource.Spec.Console.SSLSecret
	}
	sslFlags = generateConsoleSSLFlags(fsm.customResource, client, secretName)
	envVars := make(map[string]string)
	envVars[envVarName] = sslFlags
	retVal = sourceEnvVarFromSecret(fsm.customResource, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func syncMessageMigration(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme) {

	var err error = nil
	var retrieveError error = nil

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	fsm := namespacedNameToFSM[namespacedName]

	ssNames := make(map[string]string)
	ssNames["CRNAMESPACE"] = customResource.Namespace
	ssNames["CRNAME"] = customResource.Name
	ssNames["CLUSTERUSER"] = environments.GLOBAL_AMQ_CLUSTER_USER
	ssNames["CLUSTERPASS"] = environments.GLOBAL_AMQ_CLUSTER_PASSWORD
	ssNames["HEADLESSSVCNAMEVALUE"] = fsm.GetHeadlessServiceName()
	ssNames["PINGSVCNAMEVALUE"] = fsm.GetPingServiceName()
	ssNames["SERVICE_ACCOUNT"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["SERVICE_ACCOUNT_NAME"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["AMQ_CREDENTIALS_SECRET_NAME"] = fsm.GetCredentialsSecretName()

	scaledown := &brokerv2alpha1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      selectors.LabelBuilder.Labels(),
			Name:        customResource.Name,
			Namespace:   customResource.Namespace,
			Annotations: ssNames,
		},
		Spec: brokerv2alpha1.ActiveMQArtemisScaledownSpec{
			LocalOnly: isLocalOnly(),
		},
		Status: brokerv2alpha1.ActiveMQArtemisScaledownStatus{},
	}

	if nil == customResource.Spec.DeploymentPlan.MessageMigration {
		customResource.Spec.DeploymentPlan.MessageMigration = &defaultMessageMigration
	}

	clustered := isClustered(customResource)

	if *customResource.Spec.DeploymentPlan.MessageMigration && clustered {
		if !customResource.Spec.DeploymentPlan.PersistenceEnabled {
			log.Info("Won't set up scaledown for non persistent deployment")
			return
		}
		log.Info("we need scaledown for this cr", "crName", customResource.Name, "scheme", scheme)
		if err = resources.Retrieve(namespacedName, client, scaledown); err != nil {
			// err means not found so create
			log.Info("Creating builtin drainer CR ", "scaledown", scaledown)
			if retrieveError = resources.Create(customResource, namespacedName, client, scheme, scaledown); retrieveError == nil {
				log.Info("drainer created successfully", "drainer", scaledown)
			} else {
				log.Error(retrieveError, "we have error retrieving drainer", "drainer", scaledown, "scheme", scheme)
			}
		}
	} else {
		if err = resources.Retrieve(namespacedName, client, scaledown); err == nil {
			activemqartemisscaledown.ReleaseController(customResource.Name)
			// err means not found so delete
			if retrieveError = resources.Delete(namespacedName, client, scaledown); retrieveError == nil {
			}
		}
	}
}

func isLocalOnly() bool {
	oprNamespace := os.Getenv("OPERATOR_NAMESPACE")
	watchNamespace := os.Getenv("OPERATOR_WATCH_NAMESPACE")
	if oprNamespace == watchNamespace {
		return true
	}
	return false
}

func sourceEnvVarFromSecret(customResource *brokerv2alpha5.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]string, secretName string, client client.Client, scheme *runtime.Scheme) uint32 {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")

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
		//custom init container
		if len(currentStatefulSet.Spec.Template.Spec.InitContainers) > 0 {
			log.Info("we have custom init-containers")
			if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarName); nil == retrievedEnvVar {
				environments.Create(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarDefinition)
			} else {
				log.V(1).Info("sourceEnvVarFromSecret retrieved for init container" + envVarName)
			}
		}
	}

	return retVal
}

func sourceEnvVarFromSecret2(customResource *brokerv2alpha5.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]ValueInfo, secretName string, client client.Client, scheme *runtime.Scheme) uint32 {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")

	var err error = nil
	var retVal uint32 = statefulSetNotUpdated

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: currentStatefulSet.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := make(map[string]string)
	for k := range *envVars {
		stringDataMap[k] = (*envVars)[k].Value
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
			if 0 != strings.Compare(string(elem), (*envVars)[k].Value) || !ok {
				log.V(1).Info("Secret exists but not equals, or not ok", "ok?", ok)
				if !(*envVars)[k].AutoGen || string(elem) == "" {
					secretDefinition.Data[k] = []byte((*envVars)[k].Value)
					needUpdate = true
				}
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
		}
		//custom init container
		if len(currentStatefulSet.Spec.Template.Spec.InitContainers) > 0 {
			log.Info("we have custom init-containers")
			if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarName); nil == retrievedEnvVar {
				environments.Create(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarDefinition)
			}
		}
	}

	return retVal
}

func generateAcceptorsString(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client) string {

	// TODO: Optimize for the single broker configuration
	ensureCOREOn61616Exists := true // as clustered is no longer an option but true by default

	acceptorEntry := ""
	defaultArgs := "tcpSendBufferSize=1048576;tcpReceiveBufferSize=1048576;useEpoll=true;amqpCredits=1000;amqpMinCredits=300"

	var portIncrement int32 = 10
	var currentPortIncrement int32 = 0
	var port61616InUse bool = false
	var i uint32 = 0
	for _, acceptor := range customResource.Spec.Acceptors {
		if 0 == acceptor.Port {
			acceptor.Port = 61626 + currentPortIncrement
			currentPortIncrement += portIncrement
			customResource.Spec.Acceptors[i].Port = acceptor.Port
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
			!strings.Contains(strings.ToUpper(acceptor.Protocols), "CORE") {
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
		if acceptor.SupportAdvisory != nil {
			acceptorEntry = acceptorEntry + ";" + "supportAdvisory=" + strconv.FormatBool(*acceptor.SupportAdvisory)
		}
		if acceptor.SuppressInternalManagementObjects != nil {
			acceptorEntry = acceptorEntry + ";" + "suppressInternalManagementObjects=" + strconv.FormatBool(*acceptor.SuppressInternalManagementObjects)
		}
		acceptorEntry = acceptorEntry + ";" + defaultArgs

		acceptorEntry = acceptorEntry + "<\\/acceptor>"

		// Used for indexing the original acceptor port to update it if required
		i = i + 1
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

func generateConnectorsString(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client) string {

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

func configureAcceptorsExposure(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	for ; i < fsm.customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = fsm.GetStatefulSetName() + "-" + ordinalString

		for _, acceptor := range fsm.customResource.Spec.Acceptors {
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, acceptor.Name+"-"+ordinalString, acceptor.Port, serviceRoutelabels)
			serviceNamespacedName := types.NamespacedName{
				Name:      serviceDefinition.Name,
				Namespace: fsm.customResource.Namespace,
			}
			if acceptor.Expose {
				requestedResources = append(requestedResources, serviceDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			}
			targetPortName := acceptor.Name + "-" + ordinalString
			targetServiceName := fsm.customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, acceptor.SSLEnabled)
			routeNamespacedName := types.NamespacedName{
				Name:      routeDefinition.Name,
				Namespace: fsm.customResource.Namespace,
			}
			if acceptor.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConnectorsExposure(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	for ; i < fsm.customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = fsm.GetStatefulSetName() + "-" + ordinalString

		for _, connector := range fsm.customResource.Spec.Connectors {
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, connector.Name+"-"+ordinalString, connector.Port, serviceRoutelabels)

			serviceNamespacedName := types.NamespacedName{
				Name:      serviceDefinition.Name,
				Namespace: fsm.customResource.Namespace,
			}
			if connector.Expose {
				requestedResources = append(requestedResources, serviceDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, serviceNamespacedName, serviceDefinition)
			}
			targetPortName := connector.Name + "-" + ordinalString
			targetServiceName := fsm.customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, connector.SSLEnabled)

			routeNamespacedName := types.NamespacedName{
				Name:      routeDefinition.Name,
				Namespace: fsm.customResource.Namespace,
			}
			if connector.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConsoleExposure(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false
	console := fsm.customResource.Spec.Console

	originalLabels := selectors.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	for ; i < fsm.customResource.Spec.DeploymentPlan.Size; i++ {
		ordinalString = strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = fsm.GetStatefulSetName() + "-" + ordinalString

		portNumber := int32(8161)
		targetPortName := "wconsj" + "-" + ordinalString
		targetServiceName := fsm.customResource.Name + "-" + targetPortName + "-svc"

		serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, targetPortName, portNumber, serviceRoutelabels)

		serviceNamespacedName := types.NamespacedName{
			Name:      serviceDefinition.Name,
			Namespace: fsm.customResource.Namespace,
		}
		if console.Expose {
			requestedResources = append(requestedResources, serviceDefinition)
			//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
		} else {
			causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, serviceNamespacedName, serviceDefinition)
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
				Namespace: fsm.customResource.Namespace,
			}
			if console.Expose {
				requestedResources = append(requestedResources, routeDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, routeNamespacedName, routeDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, routeNamespacedName, routeDefinition)
			}
		} else {
			log.Info("Environment is not OpenShift, creating ingress")
			ingressDefinition := ingresses.NewIngressForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName)
			ingressNamespacedName := types.NamespacedName{
				Name:      ingressDefinition.Name,
				Namespace: fsm.customResource.Namespace,
			}
			if console.Expose {
				requestedResources = append(requestedResources, ingressDefinition)
				//causedUpdate, err = resources.Enable(customResource, client, scheme, ingressNamespacedName, ingressDefinition)
			} else {
				causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, ingressNamespacedName, ingressDefinition)
			}
		}
	}

	return causedUpdate, err
}

func generateConsoleSSLFlags(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, secretName string) string {

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

func generateAcceptorConnectorSSLArguments(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, secretName string) string {

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

func generateAcceptorSSLOptionalArguments(acceptor brokerv2alpha5.AcceptorType) string {

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

func generateConnectorSSLOptionalArguments(connector brokerv2alpha5.ConnectorType) string {

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

func aioSyncCausedUpdateOn(deploymentPlan *brokerv2alpha5.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

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

func persistentSyncCausedUpdateOn(deploymentPlan *brokerv2alpha5.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

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

func imageSyncCausedUpdateOn(customResource *brokerv2alpha5.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet) bool {

	// Log where we are and what we're doing
	reqLogger := log.WithName(customResource.Name)
	reqLogger.V(1).Info("imageSyncCausedUpdateOn")

	imageName := ""
	if "placeholder" == customResource.Spec.DeploymentPlan.Image ||
		0 == len(customResource.Spec.DeploymentPlan.Image) {
		reqLogger.Info("Determining the updated kubernetes image to use due to placeholder setting")
		imageName = determineImageToUse(customResource, "Kubernetes")
	} else {
		reqLogger.Info("Using the user provided kubernetes image " + customResource.Spec.DeploymentPlan.Image)
		imageName = customResource.Spec.DeploymentPlan.Image
	}
	if strings.Compare(currentStatefulSet.Spec.Template.Spec.Containers[0].Image, imageName) != 0 {
		containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
		for i := 0; i < containerArrayLen; i++ {
			currentStatefulSet.Spec.Template.Spec.Containers[i].Image = imageName
		}
		return true
	}

	return false
}

// TODO: Eliminate duplication between this and the original imageSyncCausedUpdateOn
func initImageSyncCausedUpdateOn(customResource *brokerv2alpha5.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet) bool {

	// Log where we are and what we're doing
	reqLogger := log.WithName(customResource.Name)
	reqLogger.V(1).Info("initImageSyncCausedUpdateOn")

	initImageName := ""
	if "placeholder" == customResource.Spec.DeploymentPlan.InitImage ||
		0 == len(customResource.Spec.DeploymentPlan.InitImage) {
		reqLogger.Info("Determining the updated init image to use due to placeholder setting")
		initImageName = determineImageToUse(customResource, "Init")
	} else {
		reqLogger.Info("Using the user provided init image " + customResource.Spec.DeploymentPlan.InitImage)
		initImageName = customResource.Spec.DeploymentPlan.InitImage
	}
	if strings.Compare(currentStatefulSet.Spec.Template.Spec.InitContainers[0].Image, initImageName) != 0 {
		containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.InitContainers)
		for i := 0; i < containerArrayLen; i++ {
			currentStatefulSet.Spec.Template.Spec.InitContainers[i].Image = initImageName
		}
		return true
	}

	return false
}

func clusterSyncCausedUpdateOn(deploymentPlan *brokerv2alpha5.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	isClustered := true
	if deploymentPlan.Clustered != nil {
		isClustered = *deploymentPlan.Clustered
	}

	//we are only interested in non-cluster case
	//as elsewhere in the code it's been treated as clustered.
	if !isClustered {

		brokerContainers := []*corev1.Container{
			&currentStatefulSet.Spec.Template.Spec.InitContainers[0],
			&currentStatefulSet.Spec.Template.Spec.Containers[0],
		}

		for _, container := range brokerContainers {
			for j, envVar := range container.Env {
				if "AMQ_CLUSTERED" == envVar.Name {
					container.Env[j].Value = "false"
					log.Info("Setting clustered env to false", "envs", container.Env)
				}
			}
		}
	}
	return !isClustered
}

func (reconciler *ActiveMQArtemisReconciler) ProcessResources(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8 {

	reqLogger := log.WithValues("ActiveMQArtemis Name", fsm.customResource.Name)
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
		requestedResources[index].SetNamespace(fsm.customResource.Namespace)
	}

	deployed, err = getDeployedResources(fsm.customResource, client)
	if err != nil {
		reqLogger.Error(err, "error getting deployed resources", "returned", stepsComplete)
		return stepsComplete
	}

	requested := compare.NewMapBuilder().Add(requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()
	deltas := comparator.Compare(deployed, requested)
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	for resourceType, delta := range deltas {
		reqLogger.Info("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			added, stepsComplete = reconciler.createResource(fsm, client, scheme, resourceToAdd, added, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			updated, stepsComplete = reconciler.updateResource(fsm.customResource, client, scheme, resourceToUpdate, updated, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			removed, stepsComplete = reconciler.deleteResource(fsm.customResource, client, scheme, resourceToRemove, removed, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		hasUpdates = hasUpdates || added || updated || removed
	}

	//empty the collected objects
	requestedResources = nil

	return stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) createResource(fsm *ActiveMQArtemisFSM, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, added bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, createError error, stepsComplete uint8) (bool, uint8) {

	kind := requested.GetName()
	added = true
	reqLogger.V(1).Info("Adding delta resources, i.e. creating ", "for kind ", kind)
	reqLogger.V(1).Info("last namespacedName.Name was " + namespacedName.Name)
	namespacedName.Name = kind
	reqLogger.V(1).Info("this namespacedName.Name IS " + namespacedName.Name)
	err, createError = reconciler.createRequestedResource(fsm.customResource, client, scheme, namespacedName, requested, reqLogger, createError, kind)
	if nil == createError && nil != err {
		switch kind {
		case fsm.GetStatefulSetName():
			stepsComplete |= CreatedStatefulSet
		case fsm.GetHeadlessServiceName():
			stepsComplete |= CreatedHeadlessService
		case fsm.GetPingServiceName():
			stepsComplete |= CreatedPingService
		case fsm.GetCredentialsSecretName():
			stepsComplete |= CreatedCredentialsSecret
		case fsm.GetNettySecretName():
			stepsComplete |= CreatedNettySecret
		default:
		}
	} else if nil != createError {
		reqLogger.Error(createError, "Failed to create resource "+kind+" named "+namespacedName.Name)
	}

	return added, stepsComplete
}

func (reconciler *ActiveMQArtemisReconciler) updateResource(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, updated bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, updateError error, stepsComplete uint8) (bool, uint8) {

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

func (reconciler *ActiveMQArtemisReconciler) deleteResource(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, requested resource.KubernetesResource, deleted bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, deleteError error, stepsComplete uint8) (bool, uint8) {

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

func (reconciler *ActiveMQArtemisReconciler) createRequestedResource(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, createError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("createResource Failed to Retrieve " + namespacedName.Name)
		if createError = resources.Create(customResource, namespacedName, client, scheme, requested); createError == nil {
			reqLogger.Info("Created kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, createError
}

func (reconciler *ActiveMQArtemisReconciler) updateRequestedResource(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, updateError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("updateResource Failed to Retrieve " + namespacedName.Name)
		if updateError = resources.Update(namespacedName, client, requested); updateError == nil {
			reqLogger.Info("updated kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, updateError
}

func (reconciler *ActiveMQArtemisReconciler) deleteRequestedResource(customResource *brokerv2alpha5.ActiveMQArtemis, client client.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested resource.KubernetesResource, reqLogger logr.Logger, deleteError error, kind string) (error, error) {

	var err error = nil

	if err = resources.Retrieve(namespacedName, client, requested); err != nil {
		reqLogger.Info("deleteResource Failed to Retrieve " + namespacedName.Name)
		if deleteError = resources.Delete(namespacedName, client, requested); deleteError == nil {
			reqLogger.Info("deleted kind " + kind + " named " + namespacedName.Name)
		}
	}

	return err, deleteError
}

func getDeployedResources(instance *brokerv2alpha5.ActiveMQArtemis, client client.Client) (map[reflect.Type][]resource.KubernetesResource, error) {

	var log = logf.Log.WithName("controller_v2alpha5activemqartemis")

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
		log.Error(err, "Failed to list deployed objects. ", err)
		return nil, err
	}

	return resourceMap, nil
}

func MakeVolumes(fsm *ActiveMQArtemisFSM) []corev1.Volume {

	volumeDefinitions := []corev1.Volume{}
	if fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
		basicCRVolume := volumes.MakePersistentVolume(fsm.customResource.Name)
		volumeDefinitions = append(volumeDefinitions, basicCRVolume...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range fsm.customResource.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		secretName := fsm.customResource.Name + "-" + acceptor.Name + "-secret"
		if "" != acceptor.SSLSecret {
			secretName = acceptor.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range fsm.customResource.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		secretName := fsm.customResource.Name + "-" + connector.Name + "-secret"
		if "" != connector.SSLSecret {
			secretName = connector.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	if fsm.customResource.Spec.Console.SSLEnabled {
		secretName := fsm.GetConsoleSecretName()
		if "" != fsm.customResource.Spec.Console.SSLSecret {
			secretName = fsm.customResource.Spec.Console.SSLSecret
		}
		volume := volumes.MakeVolume(secretName)
		volumeDefinitions = append(volumeDefinitions, volume)
	}

	return volumeDefinitions
}

func MakeVolumeMounts(fsm *ActiveMQArtemisFSM) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	if fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
		persistentCRVlMnt := volumes.MakePersistentVolumeMount(fsm.customResource.Name)
		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
	}

	// Scan acceptors for any with sslEnabled
	for _, acceptor := range fsm.customResource.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		volumeMountName := fsm.customResource.Name + "-" + acceptor.Name + "-secret-volume"
		if "" != acceptor.SSLSecret {
			volumeMountName = acceptor.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range fsm.customResource.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		volumeMountName := fsm.customResource.Name + "-" + connector.Name + "-secret-volume"
		if "" != connector.SSLSecret {
			volumeMountName = connector.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	if fsm.customResource.Spec.Console.SSLEnabled {
		volumeMountName := fsm.GetConsoleSecretName() + "-volume"
		if "" != fsm.customResource.Spec.Console.SSLSecret {
			volumeMountName = fsm.customResource.Spec.Console.SSLSecret + "-volume"
		}
		volumeMount := volumes.MakeVolumeMount(volumeMountName)
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

func MakeContainerPorts(cr *brokerv2alpha5.ActiveMQArtemis) []corev1.ContainerPort {

	containerPorts := []corev1.ContainerPort{}
	if cr.Spec.DeploymentPlan.JolokiaAgentEnabled {
		jolokiaContainerPort := corev1.ContainerPort{

			Name:          "jolokia",
			ContainerPort: 8778,
			Protocol:      "TCP",
		}
		containerPorts = append(containerPorts, jolokiaContainerPort)
	}

	return containerPorts
}

func NewPodTemplateSpecForCR(fsm *ActiveMQArtemisFSM) corev1.PodTemplateSpec {

	// Log where we are and what we're doing
	reqLogger := log.WithName(fsm.customResource.Name)
	reqLogger.V(1).Info("NewPodTemplateSpecForCR")

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}

	terminationGracePeriodSeconds := int64(60)

	pts := pods.MakePodTemplateSpec(namespacedName, selectors.LabelBuilder.Labels())
	Spec := corev1.PodSpec{}
	Containers := []corev1.Container{}

	imageName := ""
	if "placeholder" == fsm.customResource.Spec.DeploymentPlan.Image ||
		0 == len(fsm.customResource.Spec.DeploymentPlan.Image) {
		reqLogger.Info("Determining the kubernetes image to use due to placeholder setting")
		imageName = determineImageToUse(fsm.customResource, "Kubernetes")
	} else {
		reqLogger.Info("Using the user provided kubernetes image " + fsm.customResource.Spec.DeploymentPlan.Image)
		imageName = fsm.customResource.Spec.DeploymentPlan.Image
	}
	reqLogger.V(1).Info("NewPodTemplateSpecForCR determined image to use " + imageName)
	container := containers.MakeContainer(fsm.customResource.Name, imageName, MakeEnvVarArrayForCR(fsm))

	container.Resources = fsm.customResource.Spec.DeploymentPlan.Resources

	containerPorts := MakeContainerPorts(fsm.customResource)
	if len(containerPorts) > 0 {
		reqLogger.V(1).Info("Adding new ports to main", "len", len(containerPorts))
		container.Ports = containerPorts
	}
	reqLogger.V(1).Info("now ports added to container", "new len", len(container.Ports))

	reqLogger.Info("Checking out extraMounts", "extra config", fsm.customResource.Spec.DeploymentPlan.ExtraMounts)
	extraVolumes, extraVolumeMounts := createExtraConfigmapsAndSecrets(&container, &fsm.customResource.Spec.DeploymentPlan.ExtraMounts)

	reqLogger.Info("Extra volumes", "volumes", extraVolumes)
	reqLogger.Info("Extra mounts", "mounts", extraVolumeMounts)
	volumeMounts := MakeVolumeMounts(fsm)
	if len(extraVolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, extraVolumeMounts...)
	}
	if len(volumeMounts) > 0 {
		reqLogger.V(1).Info("Adding new mounts to main", "len", len(volumeMounts))
		container.VolumeMounts = volumeMounts
	}
	reqLogger.V(1).Info("now mounts added to container", "new len", len(container.VolumeMounts))

	if fsm.customResource.Spec.DeploymentPlan.LivenessProbe.TimeoutSeconds != nil {
		container.LivenessProbe.TimeoutSeconds = *fsm.customResource.Spec.DeploymentPlan.LivenessProbe.TimeoutSeconds
		reqLogger.V(1).Info("Setting liveness timeoutSeconds", "value", container.LivenessProbe.TimeoutSeconds)
	}
	reqLogger.V(1).Info("Checking out readiness", "crvalue", fsm.customResource.Spec.DeploymentPlan.ReadinessProbe.TimeoutSeconds)
	if fsm.customResource.Spec.DeploymentPlan.ReadinessProbe.TimeoutSeconds != nil {
		container.ReadinessProbe.TimeoutSeconds = *fsm.customResource.Spec.DeploymentPlan.ReadinessProbe.TimeoutSeconds
		reqLogger.V(1).Info("Setting readiness timeoutSeconds", "value", container.ReadinessProbe.TimeoutSeconds)
	}

	Spec.Containers = append(Containers, container)
	brokerVolumes := MakeVolumes(fsm)
	if len(extraVolumes) > 0 {
		brokerVolumes = append(brokerVolumes, extraVolumes...)
	}
	if len(brokerVolumes) > 0 {
		Spec.Volumes = brokerVolumes
	}
	Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

	var cfgVolumeName string = "amq-cfg-dir"

	//tell container don't config
	envConfigBroker := corev1.EnvVar{
		Name:  "CONFIG_BROKER",
		Value: "false",
	}
	environments.Create(Spec.Containers, &envConfigBroker)

	envBrokerCustomInstanceDir := corev1.EnvVar{
		Name:  "CONFIG_INSTANCE_DIR",
		Value: brokerConfigRoot,
	}
	environments.Create(Spec.Containers, &envBrokerCustomInstanceDir)

	//add empty-dir volume and volumeMounts to main container
	volumeForCfg := volumes.MakeVolumeForCfg(cfgVolumeName)
	Spec.Volumes = append(Spec.Volumes, volumeForCfg)

	volumeMountForCfg := volumes.MakeVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	Spec.Containers[0].VolumeMounts = append(Spec.Containers[0].VolumeMounts, volumeMountForCfg)

	log.Info("Creating init container for broker configuration")
	initContainer := containers.MakeInitContainer("", "", MakeEnvVarArrayForCR(fsm))

	initImageName := ""
	if "placeholder" == fsm.customResource.Spec.DeploymentPlan.InitImage ||
		0 == len(fsm.customResource.Spec.DeploymentPlan.InitImage) {
		reqLogger.Info("Determining the init image to use due to placeholder setting")
		initImageName = determineImageToUse(fsm.customResource, "Init")
	} else {
		reqLogger.Info("Using the user provided init image " + fsm.customResource.Spec.DeploymentPlan.InitImage)
		initImageName = fsm.customResource.Spec.DeploymentPlan.InitImage
	}
	reqLogger.V(1).Info("NewPodTemplateSpecForCR determined initImage to use " + initImageName)

	initContainer.Name = fsm.customResource.Name + "-container-init"
	initContainer.Image = initImageName
	initContainer.Command = []string{"/bin/bash"}
	initContainer.Resources = fsm.customResource.Spec.DeploymentPlan.Resources

	var initCmds []string
	var initCfgRootDir = "/init_cfg_root"

	compactVersionToUse := determineCompactVersionToUse(fsm.customResource)
	yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.FullVersionFromCompactVersion[compactVersionToUse]]
	yacfgProfileName := version.YacfgProfileName

	//address settings
	addressSettings := fsm.customResource.Spec.AddressSettings.AddressSetting
	if len(addressSettings) > 0 {
		reqLogger.Info("processing address-settings")

		var configYaml strings.Builder
		var configSpecials map[string]string = make(map[string]string)

		var hasAddressSettings bool = len(addressSettings) > 0

		if hasAddressSettings {
			reqLogger.Info("We have custom address-settings")

			brokerYaml, specials := cr2jinja2.MakeBrokerCfgOverrides(fsm.customResource, nil, nil)

			configYaml.WriteString(brokerYaml)

			for k, v := range specials {
				configSpecials[k] = v
			}
		}

		byteArray, err := json.Marshal(configSpecials)
		if err != nil {
			log.Error(err, "failed to marshal specials")
		}
		jsonSpecials := string(byteArray)

		envVarTuneFilePath := "TUNE_PATH"
		outputDir := initCfgRootDir + "/yacfg_etc"

		compactVersionToUse := determineCompactVersionToUse(fsm.customResource)
		yacfgProfileVersion = version.FullVersionFromCompactVersion[compactVersionToUse]
		yacfgProfileName := version.YacfgProfileName

		initCmd := "mkdir -p " + outputDir + "; echo \"" + configYaml.String() + "\" > " + outputDir +
			"/broker.yaml; cat " + outputDir + "/broker.yaml; yacfg --profile " + yacfgProfileName + "/" +
			yacfgProfileVersion + "/default_with_user_address_settings.yaml.jinja2  --tune " +
			outputDir + "/broker.yaml --extra-properties '" + jsonSpecials + "' --output " + outputDir

		log.Info("==debug==, initCmd: " + initCmd)
		initCmds = append(initCmds, initCmd)

		//populate args of init container

		Spec.InitContainers = []corev1.Container{
			initContainer,
		}

		//expose env for address-settings
		envVarApplyRule := "APPLY_RULE"
		envVarApplyRuleValue := fsm.customResource.Spec.AddressSettings.ApplyRule

		if envVarApplyRuleValue == nil {
			envVarApplyRuleValue = &defApplyRule
		}
		reqLogger.V(1).Info("Process addresssetting", "ApplyRule", *envVarApplyRuleValue)

		applyRule := corev1.EnvVar{
			Name:  envVarApplyRule,
			Value: *envVarApplyRuleValue,
		}
		environments.Create(Spec.InitContainers, &applyRule)

		mergeBrokerAs := corev1.EnvVar{
			Name:  "MERGE_BROKER_AS",
			Value: "true",
		}
		environments.Create(Spec.InitContainers, &mergeBrokerAs)

		//pass cfg file location and apply rule to init container via env vars
		tuneFile := corev1.EnvVar{
			Name:  envVarTuneFilePath,
			Value: outputDir,
		}
		environments.Create(Spec.InitContainers, &tuneFile)

	} else {
		log.Info("No addressetings")

		Spec.InitContainers = []corev1.Container{
			initContainer,
		}
	}
	//now make volumes mount available to init image
	log.Info("making volume mounts")

	//setup volumeMounts
	volumeMountForCfgRoot := volumes.MakeVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	Spec.InitContainers[0].VolumeMounts = append(Spec.InitContainers[0].VolumeMounts, volumeMountForCfgRoot)

	volumeMountForCfg = volumes.MakeVolumeMountForCfg("tool-dir", initCfgRootDir)
	Spec.InitContainers[0].VolumeMounts = append(Spec.InitContainers[0].VolumeMounts, volumeMountForCfg)

	//add empty-dir volume
	volumeForCfg = volumes.MakeVolumeForCfg("tool-dir")
	Spec.Volumes = append(Spec.Volumes, volumeForCfg)

	log.Info("Total volumes ", "volumes", Spec.Volumes)

	var initArgs []string = []string{"-c"}

	//provide a way to configuration after launch.sh
	var brokerHandlerCmds []string = []string{}
	log.Info("Checking if there are any config handlers", "main cr", namespacedName)
	brokerConfigHandler := GetBrokerConfigHandler(namespacedName)
	if brokerConfigHandler != nil {
		log.Info("there is a config handler")
		handlerCmds := brokerConfigHandler.Config(Spec.InitContainers, initCfgRootDir+"/security", yacfgProfileVersion, yacfgProfileName)
		log.Info("Getting back some init commands", "handlerCmds", handlerCmds)
		if len(handlerCmds) > 0 {
			log.Info("appending to initCmd array...")
			brokerHandlerCmds = append(brokerHandlerCmds, handlerCmds...)
		}
	}

	var strBuilder strings.Builder

	isFirst := true
	initCmds = append(initCmds, configCmd)
	initCmds = append(initCmds, brokerHandlerCmds...)
	initCmds = append(initCmds, initHelperScript)

	for _, icmd := range initCmds {
		if isFirst {
			isFirst = false
		} else {
			strBuilder.WriteString(" && ")
		}
		strBuilder.WriteString(icmd)
	}
	initArgs = append(initArgs, strBuilder.String())

	log.Info("The final init cmds to init ", "the cmd array", initArgs)

	Spec.InitContainers[0].Args = initArgs

	if len(extraVolumeMounts) > 0 {
		Spec.InitContainers[0].VolumeMounts = append(Spec.InitContainers[0].VolumeMounts, extraVolumeMounts...)
		log.Info("Added some extra mounts to init", "total mounts: ", Spec.InitContainers[0].VolumeMounts)
	}

	dontRun := corev1.EnvVar{
		Name:  "RUN_BROKER",
		Value: "false",
	}
	environments.Create(Spec.InitContainers, &dontRun)

	envBrokerCustomInstanceDir = corev1.EnvVar{
		Name:  "CONFIG_INSTANCE_DIR",
		Value: brokerConfigRoot,
	}
	environments.Create(Spec.InitContainers, &envBrokerCustomInstanceDir)

	configPodSecurity(&Spec, &fsm.customResource.Spec.DeploymentPlan.PodSecurity)

	log.Info("Final Init spec", "Detail", Spec.InitContainers)

	pts.Spec = Spec

	return pts
}

func configPodSecurity(podSpec *corev1.PodSpec, podSecurity *brokerv2alpha5.PodSecurityType) {
	if podSecurity.ServiceAccountName != nil {
		log.Info("Pod serviceAccountName specified", "existing", podSpec.ServiceAccountName, "new", *podSecurity.ServiceAccountName)
		podSpec.ServiceAccountName = *podSecurity.ServiceAccountName
	}
	if podSecurity.RunAsUser != nil {
		log.Info("Pod runAsUser specified", "runAsUser", *podSecurity.RunAsUser)
		if podSpec.SecurityContext == nil {
			secCtxt := corev1.PodSecurityContext{
				RunAsUser: podSecurity.RunAsUser,
			}
			podSpec.SecurityContext = &secCtxt
		} else {
			podSpec.SecurityContext.RunAsUser = podSecurity.RunAsUser
		}
	}
}

func determineImageToUse(customResource *brokerv2alpha5.ActiveMQArtemis, imageTypeName string) string {

	imageName := ""
	compactVersionToUse := determineCompactVersionToUse(customResource)

	genericRelatedImageEnvVarName := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_" + imageTypeName + "_" + compactVersionToUse
	// Default case of x86_64/amd64 covered here
	archSpecificRelatedImageEnvVarName := genericRelatedImageEnvVarName
	if "s390x" == osruntime.GOARCH || "ppc64le" == osruntime.GOARCH {
		archSpecificRelatedImageEnvVarName = genericRelatedImageEnvVarName + "_" + osruntime.GOARCH
	}
	log.V(1).Info("DetermineImageToUse GOARCH specific image env var is " + archSpecificRelatedImageEnvVarName)
	imageName = os.Getenv(archSpecificRelatedImageEnvVarName)
	log.V(1).Info("DetermineImageToUse imageName is " + imageName)

	return imageName
}

func determineCompactVersionToUse(customResource *brokerv2alpha5.ActiveMQArtemis) string {

	specifiedVersion := customResource.Spec.Version
	compactVersionToUse := version.CompactLatestVersion
	//yacfgProfileVersion

	// See if we need to lookup what version to use
	for {
		// If there's no version specified just use the default above
		if 0 == len(specifiedVersion) {
			log.V(1).Info("DetermineImageToUse specifiedVersion was empty")
			break
		}
		log.V(1).Info("DetermineImageToUse specifiedVersion was " + specifiedVersion)

		// There is a version specified by the user...
		// Are upgrades enabled?
		if false == customResource.Spec.Upgrades.Enabled {
			log.V(1).Info("DetermineImageToUse upgrades are disabled")
			break
		}
		log.V(1).Info("DetermineImageToUse upgrades are enabled")

		// We have a specified version and upgrades are enabled in general
		// Is the version specified on "the list"
		compactSpecifiedVersion := version.CompactVersionFromVersion[specifiedVersion]
		if 0 == len(compactSpecifiedVersion) {
			log.V(1).Info("DetermineImageToUse failed to find the compact form of the specified version " + specifiedVersion)
			break
		}
		log.V(1).Info("DetermineImageToUse found the compact form " + compactSpecifiedVersion + " of specifiedVersion")

		// We found the compact form in our list, is it a minor bump?
		if version.LastMinorVersion == specifiedVersion &&
			!customResource.Spec.Upgrades.Minor {
			log.V(1).Info("DetermineImageToUse requested minor version upgrade but minor upgrades NOT enabled")
			break
		}

		log.V(1).Info("DetermineImageToUse all checks ok using user specified version " + specifiedVersion)
		compactVersionToUse = compactSpecifiedVersion
		break
	}

	return compactVersionToUse
}

func createExtraConfigmapsAndSecrets(brokerContainer *corev1.Container, extraMounts *brokerv2alpha5.ExtraMountsType) ([]corev1.Volume, []corev1.VolumeMount) {

	var extraVolumes []corev1.Volume
	var extraVolumeMounts []corev1.VolumeMount

	cfgMapPathBase := "/amq/extra/configmaps/"
	secretPathBase := "/amq/extra/secrets/"

	if len(extraMounts.ConfigMaps) > 0 {
		for _, cfgmap := range extraMounts.ConfigMaps {
			if cfgmap == "" {
				log.Info("No ConfigMap name specified, ignore", "configMap", cfgmap)
				continue
			}
			cfgmapPath := cfgMapPathBase + cfgmap
			log.Info("Resolved configMap path", "path", cfgmapPath)
			//now we have a config map. First create a volume
			cfgmapVol := volumes.MakeVolumeForConfigMap(cfgmap)
			cfgmapVolumeMount := volumes.MakeVolumeMountForCfg2(cfgmapVol.Name, cfgmapPath, true)
			extraVolumes = append(extraVolumes, cfgmapVol)
			extraVolumeMounts = append(extraVolumeMounts, cfgmapVolumeMount)
		}
	}

	if len(extraMounts.Secrets) > 0 {
		for _, secret := range extraMounts.Secrets {
			if secret == "" {
				log.Info("No Secret name specified, ignore", "Secret", secret)
				continue
			}
			secretPath := secretPathBase + secret
			//now we have a secret. First create a volume
			secretVol := volumes.MakeVolumeForSecret(secret)
			secretVolumeMount := volumes.MakeVolumeMountForCfg2(secretVol.Name, secretPath, true)
			extraVolumes = append(extraVolumes, secretVol)
			extraVolumeMounts = append(extraVolumeMounts, secretVolumeMount)
		}
	}

	return extraVolumes, extraVolumeMounts
}

func NewStatefulSetForCR(fsm *ActiveMQArtemisFSM) *appsv1.StatefulSet {

	// Log where we are and what we're doing
	reqLogger := log.WithName(fsm.customResource.Name)
	reqLogger.V(1).Info("NewStatefulSetForCR")

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	ss, Spec := statefulsets.MakeStatefulSet2(fsm.GetStatefulSetName(), fsm.GetHeadlessServiceName(), namespacedName, fsm.customResource.Annotations, fsm.customResource.Spec.DeploymentPlan.Size, NewPodTemplateSpecForCR(fsm))

	if fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
		Spec.VolumeClaimTemplates = *NewPersistentVolumeClaimArrayForCR(fsm.customResource, 1)
	}
	ss.Spec = Spec

	return ss
}

func NewPersistentVolumeClaimArrayForCR(cr *brokerv2alpha5.ActiveMQArtemis, arrayLength int) *[]corev1.PersistentVolumeClaim {

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
func UpdatePodStatus(cr *brokerv2alpha5.ActiveMQArtemis, client client.Client, ssNamespacedName types.NamespacedName) error {

	reqLogger := log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.V(1).Info("Updating status for pods")

	podStatus := GetPodStatus(cr, client, ssNamespacedName)

	reqLogger.V(1).Info("PodStatus are to be updated.............................", "info:", podStatus)
	reqLogger.V(1).Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.V(1).Info("Stopped Count........................", "info:", len(podStatus.Stopped))
	reqLogger.V(1).Info("Starting Count........................", "info:", len(podStatus.Starting))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		cr.Status.PodStatus = podStatus

		err := resources.Update(ssNamespacedName, client, cr)
		if err != nil {
			reqLogger.Error(err, "Failed to update pods status")
			return err
		}
		reqLogger.Info("Pods status updated")
		return nil
	}

	return nil
}

func GetPodStatus(cr *brokerv2alpha5.ActiveMQArtemis, client client.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {

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
	reqLogger.V(1).Info("lastStatus.Ready len is " + fmt.Sprint(len(lastStatus.Ready)))
	reqLogger.V(1).Info("status.Ready len is " + fmt.Sprint(len(status.Ready)))
	if len(status.Ready) > len(lastStatus.Ready) {
		// More pods ready, let the address controller know
		newPodCount := len(status.Ready) - len(lastStatus.Ready)
		for i := newPodCount - 1; i < len(status.Ready); i++ {
			channels.AddressListeningCh <- types.NamespacedName{namespacedName.Namespace, status.Ready[i]}
		}
	}
	lastStatus = status

	return status
}

func MakeEnvVarArrayForCR(fsm *ActiveMQArtemisFSM) []corev1.EnvVar {

	reqLogger := log.WithName(fsm.customResource.Name)
	reqLogger.V(1).Info("Adding Env variable ")

	requireLogin := "false"
	if fsm.customResource.Spec.DeploymentPlan.RequireLogin {
		requireLogin = "true"
	} else {
		requireLogin = "false"
	}

	journalType := "aio"
	if "aio" == strings.ToLower(fsm.customResource.Spec.DeploymentPlan.JournalType) {
		journalType = "aio"
	} else {
		journalType = "nio"
	}

	jolokiaAgentEnabled := "false"
	if fsm.customResource.Spec.DeploymentPlan.JolokiaAgentEnabled {
		jolokiaAgentEnabled = "true"
	} else {
		jolokiaAgentEnabled = "false"
	}

	managementRBACEnabled := "false"
	if fsm.customResource.Spec.DeploymentPlan.ManagementRBACEnabled {
		managementRBACEnabled = "true"
	} else {
		managementRBACEnabled = "false"
	}

	metricsPluginEnabled := "false"
	if fsm.customResource.Spec.DeploymentPlan.EnableMetricsPlugin != nil {
		metricsPluginEnabled = strconv.FormatBool(*fsm.customResource.Spec.DeploymentPlan.EnableMetricsPlugin)
	}

	envVar := []corev1.EnvVar{}
	envVarArrayForBasic := environments.AddEnvVarForBasic2(requireLogin, journalType, fsm.GetPingServiceName())
	envVar = append(envVar, envVarArrayForBasic...)
	if fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
		envVarArrayForPresistent := environments.AddEnvVarForPersistent(fsm.customResource.Name)
		envVar = append(envVar, envVarArrayForPresistent...)
	}

	// TODO: Optimize for the single broker configuration
	envVarArrayForCluster := environments.AddEnvVarForCluster()
	envVar = append(envVar, envVarArrayForCluster...)

	envVarArrayForJolokia := environments.AddEnvVarForJolokia(jolokiaAgentEnabled)
	envVar = append(envVar, envVarArrayForJolokia...)

	envVarArrayForManagement := environments.AddEnvVarForManagement(managementRBACEnabled)
	envVar = append(envVar, envVarArrayForManagement...)

	envVarArrayForMetricsPlugin := environments.AddEnvVarForMetricsPlugin(metricsPluginEnabled)
	envVar = append(envVar, envVarArrayForMetricsPlugin...)

	return envVar
}
