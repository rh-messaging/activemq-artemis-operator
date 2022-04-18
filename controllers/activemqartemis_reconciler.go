package controllers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/adler32"
	osruntime "runtime"
	"sort"

	"github.com/RHsyseng/operator-utils/pkg/olm"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	"github.com/RHsyseng/operator-utils/pkg/resource/read"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/containers"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/ingresses"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/persistentvolumeclaims"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/routes"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/serviceports"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/channels"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/config"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/volumes"

	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	netv1 "k8s.io/api/networking/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"

	"strconv"
	"strings"

	"os"
)

const (
	statefulSetNotUpdated            = 0
	statefulSetSizeUpdated           = 1 << 0
	statefulSetClusterConfigUpdated  = 1 << 1
	statefulSetImageUpdated          = 1 << 2
	statefulSetPersistentUpdated     = 1 << 3
	statefulSetAioUpdated            = 1 << 4
	statefulSetCommonConfigUpdated   = 1 << 5
	statefulSetRequireLoginUpdated   = 1 << 6
	statefulSetEnvVarInSecretUpdated = 1 << 7
	statefulSetAcceptorsUpdated      = 1 << 8
	statefulSetConnectorsUpdated     = 1 << 9
	statefulSetConsoleUpdated        = 1 << 10
	statefulSetInitImageUpdated      = 1 << 11

	livenessProbeGraceTime = 30
	TCPLivenessPort        = 8161
)

var defaultMessageMigration bool = true

var lastStatusMap map[types.NamespacedName]olm.DeploymentStatus = make(map[types.NamespacedName]olm.DeploymentStatus)

// the helper script looks for "/amq/scripts/post-config.sh"
// and run it if exists.
var initHelperScript = "/opt/amq-broker/script/default.sh"
var brokerConfigRoot = "/amq/init/config"
var configCmd = "/opt/amq/bin/launch.sh"

//default ApplyRule for address-settings
var defApplyRule string = "merge_all"
var yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.LatestVersion]

type ActiveMQArtemisReconcilerImpl struct {
	statefulSetUpdates uint32
}

type ValueInfo struct {
	Value    string
	AutoGen  bool
	Internal bool //if true put this value to the internal secret
}

type ActiveMQArtemisIReconciler interface {
	Process(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, firstTime bool) uint32
	ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client rtclient.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool)
	ProcessCredentials(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessDeploymentPlan(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32
	ProcessAcceptorsAndConnectors(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessConsole(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet)
	ProcessResources(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8
	ProcessAddressSettings(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) bool
}

func (reconciler *ActiveMQArtemisReconcilerImpl) Process(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, firstTime bool) (uint32, uint8, *appsv1.StatefulSet) {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")
	log.Info("Reconciler Processing...", "Operator version", version.Version, "ActiveMQArtemis release", fsm.customResource.Spec.Version, "firstTime:", firstTime)
	log.Info("Reconciler Processing...", "CRD.Name", fsm.customResource.Name, "CRD ver", fsm.customResource.ObjectMeta.ResourceVersion, "CRD Gen", fsm.customResource.ObjectMeta.Generation)

	reconciler.CurrentDeployedResources(fsm, client)

	// currentStateful Set is a clone of what exists if already deployed
	// what follows should transform the resources using the crd
	// if the transformation results in some change, process resources will respect that
	// comparisons should not be necessary, leave that to process resources
	currentStatefulSet, firstTime := reconciler.ProcessStatefulSet(fsm, client, log, firstTime)

	statefulSetUpdates := reconciler.ProcessDeploymentPlan(fsm, client, scheme, currentStatefulSet, firstTime)

	statefulSetUpdates |= reconciler.ProcessCredentials(fsm, client, scheme, currentStatefulSet)

	statefulSetUpdates |= reconciler.ProcessAcceptorsAndConnectors(fsm, client, scheme, currentStatefulSet)

	statefulSetUpdates |= reconciler.ProcessConsole(fsm, client, scheme, currentStatefulSet)

	// mods to env var values sourced from secrets are not detected by process resources
	// track updates in trigger env var that has a total checksum
	trackSecretCheckSumInEnvVar(fsm.requestedResources, currentStatefulSet.Spec.Template.Spec.Containers)

	fsm.requestedResources = append(fsm.requestedResources, currentStatefulSet)

	// this should apply any deltas/updates
	stepsComplete := reconciler.ProcessResources(fsm, client, scheme)

	log.Info("Reconciler Processing... complete", "CRD ver:", fsm.customResource.ObjectMeta.ResourceVersion, "CRD Gen:", fsm.customResource.ObjectMeta.Generation)

	return statefulSetUpdates, stepsComplete, currentStatefulSet
}

func trackSecretCheckSumInEnvVar(requestedResources []rtclient.Object, container []corev1.Container) {

	// find desired secrets and checksum their 'sorted' values
	digest := adler32.New()
	for _, obj := range requestedResources {
		if secret, ok := obj.(*corev1.Secret); ok {
			// ignore secret for persistence of cr
			if strings.HasSuffix(secret.Name, "-secret") {
				// note use of StringData to match MakeSecret for the initial create case
				if len(secret.StringData) > 0 {
					for _, k := range sortedKeys(secret.StringData) {
						digest.Write([]byte(secret.StringData[k]))
					}
				} else {
					for _, k := range sortedKeysStringKeyByteValue(secret.Data) {
						digest.Write(secret.Data[k])
					}
				}
			}
		}
	}
	environments.TrackSecretCheckSumInRollCount(hex.EncodeToString(digest.Sum(nil)), container)
}

func cloneOfDeployed(kind reflect.Type, name string, fsm *ActiveMQArtemisFSM) rtclient.Object {
	for _, obj := range fsm.deployed[kind] {
		if obj.GetName() == name {
			return obj.DeepCopyObject().(rtclient.Object)
		}
	}
	return nil
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessStatefulSet(fsm *ActiveMQArtemisFSM, client rtclient.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool) {

	ssNamespacedName := fsm.GetStatefulSetNamespacedName()

	var currentStatefulSet *appsv1.StatefulSet
	obj := cloneOfDeployed(reflect.TypeOf(appsv1.StatefulSet{}), ssNamespacedName.Name, fsm)
	if obj != nil {
		currentStatefulSet = obj.(*appsv1.StatefulSet)
	}

	if currentStatefulSet == nil {
		log.Info("StatefulSet: " + ssNamespacedName.Name + " not found, will create")
		firstTime = true
	}

	log.Info("Recreating desired statefulset")
	currentStatefulSet = NewStatefulSetForCR(fsm, currentStatefulSet)

	labels := fsm.namers.LabelBuilder.Labels()
	headlessServiceDefinition := svc.NewHeadlessServiceForCR2(client, fsm.GetHeadlessServiceName(), ssNamespacedName.Namespace, serviceports.GetDefaultPorts(), labels)
	if isClustered(fsm.customResource) {
		pingServiceDefinition := svc.NewPingServiceDefinitionForCR2(client, fsm.GetPingServiceName(), ssNamespacedName.Namespace, labels, labels)
		fsm.requestedResources = append(fsm.requestedResources, pingServiceDefinition)
	}
	fsm.requestedResources = append(fsm.requestedResources, headlessServiceDefinition)

	return currentStatefulSet, firstTime
}

func isClustered(customResource *brokerv1beta1.ActiveMQArtemis) bool {
	if customResource.Spec.DeploymentPlan.Clustered != nil {
		return *customResource.Spec.DeploymentPlan.Clustered
	}
	return true
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessCredentials(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")
	log.V(1).Info("ProcessCredentials")

	envVars := make(map[string]ValueInfo)

	adminUser := ValueInfo{
		Value:   "",
		AutoGen: false,
	}
	adminPassword := ValueInfo{
		Value:   "",
		AutoGen: false,
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

	return sourceEnvVarFromSecret2(fsm, currentStatefulSet, &envVars, secretName, client, scheme)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessDeploymentPlan(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32 {

	deploymentPlan := &fsm.customResource.Spec.DeploymentPlan

	clog.Info("Processing deployment plan", "plan", deploymentPlan, "broker cr", fsm.customResource.Name)
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
		if persistentSyncCausedUpdateOn(fsm, deploymentPlan, currentStatefulSet) {
			reconciler.statefulSetUpdates |= statefulSetPersistentUpdated
		}
	}

	if updatedEnvVar := environments.BoolSyncCausedUpdateOn(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_REQUIRE_LOGIN", deploymentPlan.RequireLogin); updatedEnvVar != nil {
		environments.Update(currentStatefulSet.Spec.Template.Spec.Containers, updatedEnvVar)
		reconciler.statefulSetUpdates |= statefulSetRequireLoginUpdated
	}

	if clusterSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		clog.Info("Clustered is false")
		reconciler.statefulSetUpdates |= statefulSetClusterConfigUpdated
	}

	clog.Info("Now sync Message migration", "for cr", fsm.customResource.Name)

	syncMessageMigration(fsm, client, scheme)

	return reconciler.statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessAddressSettings(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) bool {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")
	log.Info("Process addresssettings")

	if len(customResource.Spec.AddressSettings.AddressSetting) == 0 {
		return false
	}

	//we need to compare old with new and update if they are different.
	return compareAddressSettings(&prevCustomResource.Spec.AddressSettings, &customResource.Spec.AddressSettings)
}

func checkHasChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	return checkLivenessProbeChanged(customResource, prevCustomResource) ||
		checkReadinessProbeChanged(customResource, prevCustomResource) ||
		checkTolerationsChanged(customResource, prevCustomResource) ||
		checkLabelsChanged(customResource, prevCustomResource) ||
		checkNodeSelectorsChanged(customResource, prevCustomResource) ||
		checkAffinityChanged(customResource, prevCustomResource)
}
func checkLivenessProbeChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.LivenessProbe, customResource.Spec.DeploymentPlan.LivenessProbe)
}

func checkReadinessProbeChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.ReadinessProbe, customResource.Spec.DeploymentPlan.ReadinessProbe)
}

func checkTolerationsChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	if prevCustomResource.Spec.DeploymentPlan.Tolerations == nil && customResource.Spec.DeploymentPlan.Tolerations == nil {
		return false
	}
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.Tolerations, customResource.Spec.DeploymentPlan.Tolerations)
}

func checkLabelsChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	if len(prevCustomResource.Spec.DeploymentPlan.Labels) != len(customResource.Spec.DeploymentPlan.Labels) {
		return true
	}
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.Labels, customResource.Spec.DeploymentPlan.Labels)
}

func checkNodeSelectorsChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	if len(prevCustomResource.Spec.DeploymentPlan.NodeSelector) != len(customResource.Spec.DeploymentPlan.NodeSelector) {
		return true
	}
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.NodeSelector, customResource.Spec.DeploymentPlan.NodeSelector)
}

func checkAffinityChanged(customResource *brokerv1beta1.ActiveMQArtemis, prevCustomResource *brokerv1beta1.ActiveMQArtemis) bool {
	return !reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.Affinity.PodAffinity, customResource.Spec.DeploymentPlan.Affinity.PodAffinity) ||
		!reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.Affinity.PodAntiAffinity, customResource.Spec.DeploymentPlan.Affinity.PodAntiAffinity) ||
		!reflect.DeepEqual(prevCustomResource.Spec.DeploymentPlan.Affinity.NodeAffinity, customResource.Spec.DeploymentPlan.Affinity.NodeAffinity)
}

//returns true if currentAddressSettings need update
func compareAddressSettings(currentAddressSettings *brokerv1beta1.AddressSettingsType, newAddressSettings *brokerv1beta1.AddressSettingsType) bool {

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
	if len((*currentAddressSettings).AddressSetting) != len((*newAddressSettings).AddressSetting) || !config.IsEqualV1Beta1((*currentAddressSettings).AddressSetting, (*newAddressSettings).AddressSetting) {
		return true
	}
	return false
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessAcceptorsAndConnectors(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

	var retVal uint32 = statefulSetNotUpdated

	acceptorEntry := generateAcceptorsString(fsm, client)
	connectorEntry := generateConnectorsString(fsm, client)

	configureAcceptorsExposure(fsm, client, scheme)
	configureConnectorsExposure(fsm, client, scheme)

	envVars := make(map[string]ValueInfo)

	envVars["AMQ_ACCEPTORS"] = ValueInfo{
		Value:   acceptorEntry,
		AutoGen: true,
	}

	envVars["AMQ_CONNECTORS"] = ValueInfo{
		Value:   connectorEntry,
		AutoGen: true,
	}

	secretName := fsm.GetNettySecretName()
	retVal = sourceEnvVarFromSecret2(fsm, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessConsole(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32 {

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
	sslFlags = generateConsoleSSLFlags(fsm, client, secretName)
	envVars := make(map[string]ValueInfo)
	envVars[envVarName] = ValueInfo{
		Value:    sslFlags,
		AutoGen:  true,
		Internal: true,
	}

	retVal = sourceEnvVarFromSecret2(fsm, currentStatefulSet, &envVars, secretName, client, scheme)

	return retVal
}

func syncMessageMigration(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme) {

	var err error = nil
	var retrieveError error = nil

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}

	ssNames := make(map[string]string)
	ssNames["CRNAMESPACE"] = fsm.customResource.Namespace
	ssNames["CRNAME"] = fsm.customResource.Name
	ssNames["CLUSTERUSER"] = environments.GLOBAL_AMQ_CLUSTER_USER
	ssNames["CLUSTERPASS"] = environments.GLOBAL_AMQ_CLUSTER_PASSWORD
	ssNames["HEADLESSSVCNAMEVALUE"] = fsm.GetHeadlessServiceName()
	ssNames["PINGSVCNAMEVALUE"] = fsm.GetPingServiceName()
	ssNames["SERVICE_ACCOUNT"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["SERVICE_ACCOUNT_NAME"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["AMQ_CREDENTIALS_SECRET_NAME"] = fsm.GetCredentialsSecretName()

	scaledown := &brokerv1beta1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      fsm.namers.LabelBuilder.Labels(),
			Name:        fsm.customResource.Name,
			Namespace:   fsm.customResource.Namespace,
			Annotations: ssNames,
		},
		Spec: brokerv1beta1.ActiveMQArtemisScaledownSpec{
			LocalOnly: isLocalOnly(),
			Resources: fsm.customResource.Spec.DeploymentPlan.Resources,
		},
		Status: brokerv1beta1.ActiveMQArtemisScaledownStatus{},
	}

	if nil == fsm.customResource.Spec.DeploymentPlan.MessageMigration {
		fsm.customResource.Spec.DeploymentPlan.MessageMigration = &defaultMessageMigration
	}

	clustered := isClustered(fsm.customResource)

	if *fsm.customResource.Spec.DeploymentPlan.MessageMigration && clustered {
		if !fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
			clog.Info("Won't set up scaledown for non persistent deployment")
			return
		}
		clog.Info("we need scaledown for this cr", "crName", fsm.customResource.Name, "scheme", scheme)
		if err = resources.Retrieve(namespacedName, client, scaledown); err != nil {
			// err means not found so create
			clog.Info("Creating builtin drainer CR ", "scaledown", scaledown)
			if retrieveError = resources.Create(fsm.customResource, namespacedName, client, scheme, scaledown); retrieveError == nil {
				clog.Info("drainer created successfully", "drainer", scaledown)
			} else {
				clog.Error(retrieveError, "we have error retrieving drainer", "drainer", scaledown, "scheme", scheme)
			}
		}
	} else {
		if err = resources.Retrieve(namespacedName, client, scaledown); err == nil {
			ReleaseController(fsm.customResource.Name)
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

func sourceEnvVarFromSecret2(fsm *ActiveMQArtemisFSM, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]ValueInfo, secretName string, client rtclient.Client, scheme *runtime.Scheme) uint32 {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	var retVal uint32 = statefulSetNotUpdated

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: currentStatefulSet.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := make(map[string]string)
	internalStringDataMap := make(map[string]string)

	for k, v := range *envVars {
		if v.Internal {
			internalStringDataMap[k] = v.Value
		} else {
			stringDataMap[k] = v.Value
		}
	}

	secretDefinition := secrets.NewSecret(namespacedName, secretName, stringDataMap, fsm.namers.LabelBuilder.Labels())

	if err := resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("Did not find secret " + secretName)
		}
	} else {

		// update
		for k := range *envVars {
			elem, ok := secretDefinition.Data[k]
			if 0 != strings.Compare(string(elem), (*envVars)[k].Value) || !ok {
				log.V(1).Info("key value not equals or does not exist", "key", k, "exists", ok)
				if !(*envVars)[k].AutoGen || string(elem) == "" {
					secretDefinition.Data[k] = []byte((*envVars)[k].Value)
				}
			}
		}
	}

	// ensure processResources sees it
	fsm.requestedResources = append(fsm.requestedResources, secretDefinition)

	internalSecretName := secretName + "-internal"
	if len(internalStringDataMap) > 0 {

		internalSecretDefinition := secrets.NewSecret(namespacedName, internalSecretName, internalStringDataMap, fsm.namers.LabelBuilder.Labels())
		fsm.requestedResources = append(fsm.requestedResources, internalSecretDefinition)
	}

	log.Info("Populating env vars references from secret " + secretName)

	for envVarName, envVarValue := range *envVars {
		secretNameToUse := secretName
		if envVarValue.Internal {
			secretNameToUse = internalSecretName
		}
		envVarSource := &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretNameToUse,
				},
				Key:      envVarName,
				Optional: nil,
			},
		}

		envVarDefinition := &corev1.EnvVar{
			Name:      envVarName,
			Value:     "",
			ValueFrom: envVarSource,
		}
		if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, envVarName); nil == retrievedEnvVar {
			log.V(1).Info("sourceEnvVarFromSecret failed to retrieve " + envVarName + " creating")
			environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, envVarDefinition)
		}

		//custom init container
		if len(currentStatefulSet.Spec.Template.Spec.InitContainers) > 0 {
			log.Info("we have custom init-containers")
			if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarName); nil == retrievedEnvVar {
				environments.Create(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarDefinition)
			}
		}
	}

	for _, container := range currentStatefulSet.Spec.Template.Spec.Containers {
		sortEnvVars(container.Env)
	}

	for _, container := range currentStatefulSet.Spec.Template.Spec.InitContainers {
		sortEnvVars(container.Env)
	}

	return retVal
}

func generateAcceptorsString(fsm *ActiveMQArtemisFSM, client rtclient.Client) string {

	// TODO: Optimize for the single broker configuration
	ensureCOREOn61616Exists := true // as clustered is no longer an option but true by default

	acceptorEntry := ""
	defaultArgs := "tcpSendBufferSize=1048576;tcpReceiveBufferSize=1048576;useEpoll=true;amqpCredits=1000;amqpMinCredits=300"

	var portIncrement int32 = 10
	var currentPortIncrement int32 = 0
	var port61616InUse bool = false
	var i uint32 = 0
	for _, acceptor := range fsm.customResource.Spec.Acceptors {
		if 0 == acceptor.Port {
			acceptor.Port = 61626 + currentPortIncrement
			currentPortIncrement += portIncrement
			fsm.customResource.Spec.Acceptors[i].Port = acceptor.Port
		}
		if "" == acceptor.Protocols ||
			"all" == strings.ToLower(acceptor.Protocols) {
			acceptor.Protocols = "AMQP,CORE,HORNETQ,MQTT,OPENWIRE,STOMP"
		}
		bindAddress := "ACCEPTOR_IP"
		if acceptor.BindToAllInterfaces != nil && *acceptor.BindToAllInterfaces {
			bindAddress = "0.0.0.0"
		}
		acceptorEntry = acceptorEntry + "<acceptor name=\"" + acceptor.Name + "\">"
		acceptorEntry = acceptorEntry + "tcp:" + "\\/\\/" + bindAddress + ":"
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
			secretName := fsm.customResource.Name + "-" + acceptor.Name + "-secret"
			if "" != acceptor.SSLSecret {
				secretName = acceptor.SSLSecret
			}
			acceptorEntry = acceptorEntry + ";" + generateAcceptorConnectorSSLArguments(fsm, client, secretName)
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

func generateConnectorsString(fsm *ActiveMQArtemisFSM, client rtclient.Client) string {

	connectorEntry := ""
	connectors := fsm.customResource.Spec.Connectors
	for _, connector := range connectors {
		if connector.Type == "" {
			connector.Type = "tcp"
		}
		connectorEntry = connectorEntry + "<connector name=\"" + connector.Name + "\">"
		connectorEntry = connectorEntry + strings.ToLower(connector.Type) + ":\\/\\/" + strings.ToLower(connector.Host) + ":"
		connectorEntry = connectorEntry + fmt.Sprintf("%d", connector.Port)

		if connector.SSLEnabled {
			secretName := fsm.customResource.Name + "-" + connector.Name + "-secret"
			if "" != connector.SSLSecret {
				secretName = connector.SSLSecret
			}
			connectorEntry = connectorEntry + ";" + generateAcceptorConnectorSSLArguments(fsm, client, secretName)
			sslOptionalArguments := generateConnectorSSLOptionalArguments(connector)
			if "" != sslOptionalArguments {
				connectorEntry = connectorEntry + ";" + sslOptionalArguments
			}
		}
		connectorEntry = connectorEntry + "<\\/connector>"
	}

	return connectorEntry
}

func configureAcceptorsExposure(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := fsm.namers.LabelBuilder.Labels()
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
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, acceptor.Name+"-"+ordinalString, acceptor.Port, serviceRoutelabels, fsm.namers.LabelBuilder.Labels())

			fsm.requestedResources = append(fsm.requestedResources, serviceDefinition)
			targetPortName := acceptor.Name + "-" + ordinalString
			targetServiceName := fsm.customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, acceptor.SSLEnabled)
			if acceptor.Expose {
				fsm.requestedResources = append(fsm.requestedResources, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConnectorsExposure(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false

	originalLabels := fsm.namers.LabelBuilder.Labels()
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
			serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, connector.Name+"-"+ordinalString, connector.Port, serviceRoutelabels, fsm.namers.LabelBuilder.Labels())

			fsm.requestedResources = append(fsm.requestedResources, serviceDefinition)
			targetPortName := connector.Name + "-" + ordinalString
			targetServiceName := fsm.customResource.Name + "-" + targetPortName + "-svc"
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, connector.SSLEnabled)

			if connector.Expose {
				fsm.requestedResources = append(fsm.requestedResources, routeDefinition)
			}
		}
	}

	return causedUpdate, err
}

func configureConsoleExposure(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme) (bool, error) {

	var i int32 = 0
	var err error = nil
	ordinalString := ""
	causedUpdate := false
	console := fsm.customResource.Spec.Console

	originalLabels := fsm.namers.LabelBuilder.Labels()
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

		serviceDefinition := svc.NewServiceDefinitionForCR(namespacedName, targetPortName, portNumber, serviceRoutelabels, fsm.namers.LabelBuilder.Labels())

		serviceNamespacedName := types.NamespacedName{
			Name:      serviceDefinition.Name,
			Namespace: fsm.customResource.Namespace,
		}
		if console.Expose {
			fsm.requestedResources = append(fsm.requestedResources, serviceDefinition)
			//causedUpdate, err = resources.Enable(customResource, client, scheme, serviceNamespacedName, serviceDefinition)
		} else {
			causedUpdate, err = resources.Disable(fsm.customResource, client, scheme, serviceNamespacedName, serviceDefinition)
		}
		var err error = nil
		isOpenshift := false

		if isOpenshift, err = environments.DetectOpenshift(); err != nil {
			clog.Error(err, "Failed to get env, will try kubernetes")
		}
		if isOpenshift {
			clog.Info("Environment is OpenShift")
			clog.Info("Checking routeDefinition for " + targetPortName)
			routeDefinition := routes.NewRouteDefinitionForCR(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled)
			if console.Expose {
				fsm.requestedResources = append(fsm.requestedResources, routeDefinition)
			}
		} else {
			clog.Info("Environment is not OpenShift, creating ingress")
			ingressDefinition := ingresses.NewIngressForCRWithSSL(namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled)
			if console.Expose {
				fsm.requestedResources = append(fsm.requestedResources, ingressDefinition)
			}
		}
	}

	return causedUpdate, err
}

func generateConsoleSSLFlags(fsm *ActiveMQArtemisFSM, client rtclient.Client, secretName string) string {

	sslFlags := ""
	secretNamespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: fsm.customResource.Namespace,
	}
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	stringDataMap := map[string]string{}
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, fsm.namers.LabelBuilder.Labels())

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
	if fsm.customResource.Spec.Console.UseClientAuth {
		sslFlags = sslFlags + " " + "--use-client-auth"
	}

	return sslFlags
}

func generateAcceptorConnectorSSLArguments(fsm *ActiveMQArtemisFSM, client rtclient.Client, secretName string) string {

	sslArguments := "sslEnabled=true"
	secretNamespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: fsm.customResource.Namespace,
	}
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	stringDataMap := map[string]string{}
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, fsm.namers.LabelBuilder.Labels())

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

func generateAcceptorSSLOptionalArguments(acceptor brokerv1beta1.AcceptorType) string {

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

	if "" != acceptor.KeyStoreProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "keyStoreProvider=" + acceptor.KeyStoreProvider
	}

	if "" != acceptor.TrustStoreType {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreType=" + acceptor.TrustStoreType
	}

	if "" != acceptor.TrustStoreProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreProvider=" + acceptor.TrustStoreProvider
	}

	return sslOptionalArguments
}

func generateConnectorSSLOptionalArguments(connector brokerv1beta1.ConnectorType) string {

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

	if "" != connector.KeyStoreProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "keyStoreProvider=" + connector.KeyStoreProvider
	}

	if "" != connector.TrustStoreType {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreType=" + connector.TrustStoreType
	}

	if "" != connector.TrustStoreProvider {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreProvider=" + connector.TrustStoreProvider
	}

	return sslOptionalArguments
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []corev1.EnvVar, i int) []corev1.EnvVar {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

func aioSyncCausedUpdateOn(deploymentPlan *brokerv1beta1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

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

func persistentSyncCausedUpdateOn(fsm *ActiveMQArtemisFSM, deploymentPlan *brokerv1beta1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

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
				if v.Value != fsm.namers.GLOBAL_DATA_PATH {
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
				fsm.namers.GLOBAL_DATA_PATH,
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

func imageSyncCausedUpdateOn(customResource *brokerv1beta1.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet) bool {

	// Log where we are and what we're doing
	reqLogger := ctrl.Log.WithName(customResource.Name)
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
func initImageSyncCausedUpdateOn(customResource *brokerv1beta1.ActiveMQArtemis, currentStatefulSet *appsv1.StatefulSet) bool {

	reqLogger := clog.WithName(customResource.Name)
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

func clusterSyncCausedUpdateOn(deploymentPlan *brokerv1beta1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

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
					clog.Info("Setting clustered env to false", "envs", container.Env)
				}
			}
		}
	}
	return !isClustered
}

func (reconciler *ActiveMQArtemisReconcilerImpl) CurrentDeployedResources(fsm *ActiveMQArtemisFSM, client rtclient.Client) {
	reqLogger := clog.WithValues("ActiveMQArtemis Name", fsm.customResource.Name)
	reqLogger.Info("currentDeployedResources")

	var err error
	fsm.deployed, err = getDeployedResources(fsm.customResource, client)
	if err != nil {
		reqLogger.Error(err, "error getting deployed resources")
	}

	// track persisted cr secret
	for _, secret := range fsm.deployed[reflect.TypeOf(corev1.Secret{})] {
		if strings.HasPrefix(secret.GetName(), "secret-broker-") {
			// track this as it is managed by the controller state machine, not by reconcile
			fsm.requestedResources = append(fsm.requestedResources, secret)
		}
	}

	for t, objs := range fsm.deployed {
		for _, obj := range objs {
			reqLogger.Info("Deployed ", "Type", t, "Name", obj.GetName())
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessResources(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme) uint8 {

	reqLogger := clog.WithValues("ActiveMQArtemis Name", fsm.customResource.Name)
	reqLogger.Info("Processing resources")

	var err error = nil
	var createError error = nil
	var hasUpdates bool
	var stepsComplete uint8 = 0

	added := false
	updated := false
	removed := false

	for index := range fsm.requestedResources {
		fsm.requestedResources[index].SetNamespace(fsm.customResource.Namespace)
	}

	reqLogger.Info("Processing resources", "num requested", len(fsm.requestedResources))

	requested := compare.NewMapBuilder().Add(fsm.requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()

	comparator.Comparator.SetComparator(reflect.TypeOf(appsv1.StatefulSet{}), func(deployed, requested rtclient.Object) bool {
		ss1 := deployed.(*appsv1.StatefulSet)
		ss2 := requested.(*appsv1.StatefulSet)
		isEqual := equality.Semantic.DeepEqual(ss1.Spec, ss2.Spec)
		reqLogger.V(1).Info("Compare SS.Spec", "depoyed", ss1.Spec, "requested", ss2.Spec)

		if !isEqual {
			reqLogger.V(1).Info("Unequal", "depoyed", ss1.Spec, "requested", ss2.Spec)

		}
		return isEqual
	})

	deltas := comparator.Compare(fsm.deployed, requested)
	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	for resourceType, delta := range deltas {
		reqLogger.Info("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			added, stepsComplete = reconciler.createResource(fsm, client, scheme, resourceToAdd, resourceType, added, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			updated, stepsComplete = reconciler.updateResource(fsm.customResource, client, scheme, resourceToUpdate, resourceType, updated, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			removed, stepsComplete = reconciler.deleteResource(fsm.customResource, client, scheme, resourceToRemove, resourceType, removed, reqLogger, namespacedName, err, createError, stepsComplete)
		}

		hasUpdates = hasUpdates || added || updated || removed
	}

	//empty the collected objects
	fsm.requestedResources = nil

	return stepsComplete
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createResource(fsm *ActiveMQArtemisFSM, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, added bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, createError error, stepsComplete uint8) (bool, uint8) {

	reqLogger.V(1).Info("Adding delta resources, i.e. creating ", "name ", requested.GetName(), "of kind ", kind)
	namespacedName.Name = requested.GetName()
	err = reconciler.createRequestedResource(fsm.customResource, client, scheme, namespacedName, requested, reqLogger, createError, kind)
	if nil == err {
		added = true
	}
	return added, stepsComplete
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, updated bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, updateError error, stepsComplete uint8) (bool, uint8) {

	reqLogger.V(1).Info("Updating delta resources, i.e. updating ", "name ", requested.GetName(), "of kind ", kind)
	namespacedName.Name = requested.GetName()

	err = reconciler.updateRequestedResource(customResource, client, scheme, namespacedName, requested, reqLogger, updateError, kind)
	if nil == err {
		updated = true
	}
	return updated, stepsComplete
}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, deleted bool, reqLogger logr.Logger, namespacedName types.NamespacedName, err error, deleteError error, stepsComplete uint8) (bool, uint8) {

	reqLogger.V(1).Info("Deleting delta resources, i.e. removing ", "name ", requested.GetName(), "of kind ", kind)
	namespacedName.Name = requested.GetName()

	err = reconciler.deleteRequestedResource(customResource, client, scheme, namespacedName, requested, reqLogger, deleteError, kind)
	if nil == err {
		deleted = true
	}
	return deleted, stepsComplete
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested rtclient.Object, reqLogger logr.Logger, createError error, kind reflect.Type) error {

	if createError = resources.Create(customResource, namespacedName, client, scheme, requested); createError == nil {
		reqLogger.Info("Created ", "kind ", kind, "named ", namespacedName.Name)
	}
	return createError
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested rtclient.Object, reqLogger logr.Logger, updateError error, kind reflect.Type) error {

	if updateError = resources.Update(namespacedName, client, requested); updateError == nil {
		reqLogger.Info("updated", "kind ", kind, "named ", namespacedName.Name)
	} else {
		reqLogger.Error(updateError, "updated Failed", "kind ", kind, "named ", namespacedName.Name)
	}
	return updateError
}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, namespacedName types.NamespacedName, requested rtclient.Object, reqLogger logr.Logger, deleteError error, kind reflect.Type) error {

	if deleteError = resources.Delete(namespacedName, client, requested); deleteError == nil {
		reqLogger.Info("deleted", "kind", kind, " named ", namespacedName.Name)
	} else {
		reqLogger.Error(deleteError, "delete Failed", "kind", kind, " named ", namespacedName.Name)
	}
	return deleteError
}

func getDeployedResources(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (map[reflect.Type][]rtclient.Object, error) {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	reader := read.New(client).WithNamespace(instance.Namespace).WithOwnerObject(instance)
	var resourceMap map[reflect.Type][]rtclient.Object
	var err error
	if isOpenshift, _ := environments.DetectOpenshift(); isOpenshift {
		resourceMap, err = reader.ListAll(
			&corev1.PersistentVolumeClaimList{},
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&routev1.RouteList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
		)
	} else {
		resourceMap, err = reader.ListAll(
			&corev1.PersistentVolumeClaimList{},
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&netv1.IngressList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
		)
	}
	if err != nil {
		log.Error(err, "Failed to list deployed objects.")
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
		persistentCRVlMnt := volumes.MakePersistentVolumeMount(fsm.customResource.Name, fsm.namers.GLOBAL_DATA_PATH)
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

func MakeContainerPorts(cr *brokerv1beta1.ActiveMQArtemis) []corev1.ContainerPort {

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

func NewPodTemplateSpecForCR(fsm *ActiveMQArtemisFSM, current *corev1.PodTemplateSpec) *corev1.PodTemplateSpec {

	reqLogger := ctrl.Log.WithName(fsm.customResource.Name)
	reqLogger.V(1).Info("NewPodTemplateSpecForCR", "Version", fsm.customResource.ObjectMeta.ResourceVersion, "Generation", fsm.customResource.ObjectMeta.Generation)

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}

	terminationGracePeriodSeconds := int64(60)

	labels := getPodLabels(fsm, fsm.customResource)
	//add any custom labels provided in CR before creating the pod template spec
	if fsm.customResource.Spec.DeploymentPlan.Labels != nil {
		for key, value := range fsm.customResource.Spec.DeploymentPlan.Labels {
			labels[key] = value
			reqLogger.V(1).Info("Adding Label", "key", key, "value", value)
		}
	}

	//pts := pods.MakePodTemplateSpec(current, namespacedName, fsm.namers.LabelBuilder.Labels())
	pts := pods.MakePodTemplateSpec(current, namespacedName, labels)
	podSpec := &pts.Spec

	// REVISIT: don't know when this is nil
	if podSpec == nil {
		podSpec = &corev1.PodSpec{}
	}

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
	container := containers.MakeContainer(podSpec, fsm.customResource.Name, imageName, MakeEnvVarArrayForCR(fsm))

	container.Resources = fsm.customResource.Spec.DeploymentPlan.Resources

	containerPorts := MakeContainerPorts(fsm.customResource)
	if len(containerPorts) > 0 {
		reqLogger.V(1).Info("Adding new ports to main", "len", len(containerPorts))
		container.Ports = containerPorts
	}
	reqLogger.V(1).Info("now ports added to container", "new len", len(container.Ports))

	reqLogger.Info("Checking out extraMounts", "extra config", fsm.customResource.Spec.DeploymentPlan.ExtraMounts)
	brokerPropertiesConfigMapName := addConfigMapForBrokerProperties(fsm)
	configMapsToCreate := []string{brokerPropertiesConfigMapName}
	configMapsToCreate = append(configMapsToCreate, fsm.customResource.Spec.DeploymentPlan.ExtraMounts.ConfigMaps...)

	extraVolumes, extraVolumeMounts := createExtraConfigmapsAndSecrets(container, configMapsToCreate, fsm.customResource.Spec.DeploymentPlan.ExtraMounts.Secrets)

	reqLogger.Info("Extra volumes", "volumes", extraVolumes)
	reqLogger.Info("Extra mounts", "mounts", extraVolumeMounts)
	container.VolumeMounts = MakeVolumeMounts(fsm)
	if len(extraVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, extraVolumeMounts...)
	}

	reqLogger.V(1).Info("Adding new mounts to container", "len", len(container.VolumeMounts))

	container.LivenessProbe = configureLivenessProbe(container, fsm.customResource.Spec.DeploymentPlan.LivenessProbe)
	container.ReadinessProbe = configureReadinessProbe(container, fsm.customResource.Spec.DeploymentPlan.ReadinessProbe)

	if len(fsm.customResource.Spec.DeploymentPlan.NodeSelector) > 0 {
		reqLogger.V(1).Info("Adding Node Selectors", "len", len(fsm.customResource.Spec.DeploymentPlan.NodeSelector))
		podSpec.NodeSelector = fsm.customResource.Spec.DeploymentPlan.NodeSelector
	}

	configureAffinity(podSpec, &fsm.customResource.Spec.DeploymentPlan.Affinity)

	if len(fsm.customResource.Spec.DeploymentPlan.Tolerations) > 0 {
		reqLogger.V(1).Info("Adding Tolerations", "len", len(fsm.customResource.Spec.DeploymentPlan.Tolerations))
		podSpec.Tolerations = fsm.customResource.Spec.DeploymentPlan.Tolerations
	}

	configurePodSecurityContext(podSpec, fsm.customResource.Spec.DeploymentPlan.PodSecurityContext)

	newContainersArray := []corev1.Container{}
	podSpec.Containers = append(newContainersArray, *container)
	brokerVolumes := MakeVolumes(fsm)
	if len(extraVolumes) > 0 {
		brokerVolumes = append(brokerVolumes, extraVolumes...)
	}
	if len(brokerVolumes) > 0 {
		podSpec.Volumes = brokerVolumes
	}
	podSpec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

	var cfgVolumeName string = "amq-cfg-dir"

	//tell container don't config
	envConfigBroker := corev1.EnvVar{
		Name:  "CONFIG_BROKER",
		Value: "false",
	}
	environments.Create(podSpec.Containers, &envConfigBroker)

	envBrokerCustomInstanceDir := corev1.EnvVar{
		Name:  "CONFIG_INSTANCE_DIR",
		Value: brokerConfigRoot,
	}
	environments.Create(podSpec.Containers, &envBrokerCustomInstanceDir)

	//add empty-dir volume and volumeMounts to main container
	volumeForCfg := volumes.MakeVolumeForCfg(cfgVolumeName)
	podSpec.Volumes = append(podSpec.Volumes, volumeForCfg)

	volumeMountForCfg := volumes.MakeVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMountForCfg)

	clog.Info("Creating init container for broker configuration")
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

	initContainer := containers.MakeInitContainer(podSpec, fsm.customResource.Name, initImageName, MakeEnvVarArrayForCR(fsm))
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
			clog.Error(err, "failed to marshal specials")
		}
		jsonSpecials := string(byteArray)

		envVarTuneFilePath := "TUNE_PATH"
		outputDir := initCfgRootDir + "/yacfg_etc"

		initCmd := "mkdir -p " + outputDir + "; echo \"" + configYaml.String() + "\" > " + outputDir +
			"/broker.yaml; cat " + outputDir + "/broker.yaml; yacfg --profile " + yacfgProfileName + "/" +
			yacfgProfileVersion + "/default_with_user_address_settings.yaml.jinja2  --tune " +
			outputDir + "/broker.yaml --extra-properties '" + jsonSpecials + "' --output " + outputDir

		clog.Info("==debug==, initCmd: " + initCmd)
		initCmds = append(initCmds, initCmd)

		//populate args of init container

		podSpec.InitContainers = []corev1.Container{
			*initContainer,
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
		environments.Create(podSpec.InitContainers, &applyRule)

		mergeBrokerAs := corev1.EnvVar{
			Name:  "MERGE_BROKER_AS",
			Value: "true",
		}
		environments.Create(podSpec.InitContainers, &mergeBrokerAs)

		//pass cfg file location and apply rule to init container via env vars
		tuneFile := corev1.EnvVar{
			Name:  envVarTuneFilePath,
			Value: outputDir,
		}
		environments.Create(podSpec.InitContainers, &tuneFile)

	} else {
		clog.Info("No addressetings")

		podSpec.InitContainers = []corev1.Container{
			*initContainer,
		}
	}
	//now make volumes mount available to init image
	clog.Info("making volume mounts")

	//setup volumeMounts from scratch
	podSpec.InitContainers[0].VolumeMounts = []corev1.VolumeMount{}
	volumeMountForCfgRoot := volumes.MakeVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, volumeMountForCfgRoot)

	volumeMountForCfg = volumes.MakeVolumeMountForCfg("tool-dir", initCfgRootDir)
	podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, volumeMountForCfg)

	//add empty-dir volume
	volumeForCfg = volumes.MakeVolumeForCfg("tool-dir")
	podSpec.Volumes = append(podSpec.Volumes, volumeForCfg)

	clog.Info("Total volumes ", "volumes", podSpec.Volumes)

	// this depends on init container passing --java-opts to artemis create via launch.sh *and* it
	// not getting munged on the way.
	// REVISIT: should an existing value be respected here, only when we expose JAVA_OPTS in the CR?
	javaOpts := corev1.EnvVar{
		Name:  "JAVA_OPTS",
		Value: "-Dbroker.properties=/amq/extra/configmaps/" + brokerPropertiesConfigMapName + "/broker.properties",
	}
	environments.Create(podSpec.InitContainers, &javaOpts)

	var initArgs []string = []string{"-c"}

	//provide a way to configuration after launch.sh
	var brokerHandlerCmds []string = []string{}
	clog.Info("Checking if there are any config handlers", "main cr", namespacedName)
	brokerConfigHandler := GetBrokerConfigHandler(namespacedName)
	if brokerConfigHandler != nil {
		clog.Info("there is a config handler")
		handlerCmds := brokerConfigHandler.Config(podSpec.InitContainers, initCfgRootDir+"/security", yacfgProfileVersion, yacfgProfileName)
		clog.Info("Getting back some init commands", "handlerCmds", handlerCmds)
		if len(handlerCmds) > 0 {
			clog.Info("appending to initCmd array...")
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

	clog.Info("The final init cmds to init ", "the cmd array", initArgs)

	podSpec.InitContainers[0].Args = initArgs

	if len(extraVolumeMounts) > 0 {
		podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, extraVolumeMounts...)
		clog.Info("Added some extra mounts to init", "total mounts: ", podSpec.InitContainers[0].VolumeMounts)
	}

	dontRun := corev1.EnvVar{
		Name:  "RUN_BROKER",
		Value: "false",
	}
	environments.Create(podSpec.InitContainers, &dontRun)

	envBrokerCustomInstanceDir = corev1.EnvVar{
		Name:  "CONFIG_INSTANCE_DIR",
		Value: brokerConfigRoot,
	}
	environments.Create(podSpec.InitContainers, &envBrokerCustomInstanceDir)

	// NOTE: PodSecurity contains a RunAsUser that will be overridden by that in the provided PodSecurityContext if any
	configPodSecurity(podSpec, &fsm.customResource.Spec.DeploymentPlan.PodSecurity)
	configurePodSecurityContext(podSpec, fsm.customResource.Spec.DeploymentPlan.PodSecurityContext)

	clog.Info("Final Init spec", "Detail", podSpec.InitContainers)

	pts.Spec = *podSpec

	return pts
}

func configureLivenessProbe(container *corev1.Container, probeFromCR *corev1.Probe) *corev1.Probe {
	var livenessProbe *corev1.Probe = container.LivenessProbe
	clog.V(1).Info("Configuring Liveness Probe", "existing", livenessProbe)

	if livenessProbe == nil {
		livenessProbe = &corev1.Probe{}
	}

	if probeFromCR != nil {
		//copy the probe
		clog.V(1).Info("Liveness Probe provided by user, must provide all optional values")
		livenessProbe.InitialDelaySeconds = probeFromCR.InitialDelaySeconds
		livenessProbe.TimeoutSeconds = probeFromCR.TimeoutSeconds
		livenessProbe.PeriodSeconds = probeFromCR.PeriodSeconds
		livenessProbe.TerminationGracePeriodSeconds = probeFromCR.TerminationGracePeriodSeconds
		livenessProbe.SuccessThreshold = probeFromCR.SuccessThreshold
		livenessProbe.FailureThreshold = probeFromCR.FailureThreshold

		// not complete in this case!
		if probeFromCR.Exec == nil && probeFromCR.HTTPGet == nil && probeFromCR.TCPSocket == nil {
			clog.V(1).Info("Adding default TCP check")
			livenessProbe.Handler = corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(TCPLivenessPort),
				},
			}
		} else {
			clog.V(1).Info("Using user provided Liveness Probe Exec " + probeFromCR.Exec.String())
			livenessProbe.Exec = probeFromCR.Exec
		}
	} else {
		clog.V(1).Info("Creating Default Liveness Probe")

		livenessProbe.InitialDelaySeconds = livenessProbeGraceTime
		livenessProbe.TimeoutSeconds = 5
		livenessProbe.Handler = corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(TCPLivenessPort),
			},
		}
	}

	return livenessProbe
}

func configureReadinessProbe(container *corev1.Container, probe *corev1.Probe) *corev1.Probe {

	var readinessProbe *corev1.Probe = container.ReadinessProbe
	if readinessProbe == nil {
		readinessProbe = &corev1.Probe{}
	}

	if probe != nil {
		//copy the probe
		readinessProbe.InitialDelaySeconds = probe.InitialDelaySeconds
		readinessProbe.TimeoutSeconds = probe.TimeoutSeconds
		readinessProbe.PeriodSeconds = probe.PeriodSeconds
		readinessProbe.TerminationGracePeriodSeconds = probe.TerminationGracePeriodSeconds
		readinessProbe.SuccessThreshold = probe.SuccessThreshold
		readinessProbe.FailureThreshold = probe.FailureThreshold
		if probe.Exec == nil && probe.HTTPGet == nil && probe.TCPSocket == nil {
			//add the default readiness check if none
			clog.V(1).Info("Using user provided readiness Probe")
			readinessProbe.Handler = corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/bash",
						"-c",
						"/opt/amq/bin/readinessProbe.sh",
					},
				},
			}
		} else {
			readinessProbe.Handler = probe.Handler
		}
	} else {
		clog.V(1).Info("Creating Default readiness Probe")
		readinessProbe.InitialDelaySeconds = livenessProbeGraceTime
		readinessProbe.TimeoutSeconds = 5
		readinessProbe.Handler = corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/bash",
					"-c",
					"/opt/amq/bin/readinessProbe.sh",
				},
			},
		}
	}

	return readinessProbe
}

func addConfigMapForBrokerProperties(fsm *ActiveMQArtemisFSM) string {

	// fetch and do idempotent transform based on CR
	configMapName := types.NamespacedName{
		Namespace: fsm.customResource.Namespace,
		Name:      fsm.customResource.Name + "-props-" + HexShaHashOfMap(fsm.customResource.Spec.BrokerProperties),
	}
	var desired *corev1.ConfigMap = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "k8s.io.api.core.v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         configMapName.Name,
			GenerateName: "",
			Namespace:    configMapName.Namespace,
		},
	}
	// REVISIT: get from deployed
	err := resources.Retrieve(configMapName, fsm.r.Client, desired)
	if err != nil {
		// new instance
		desired = &corev1.ConfigMap{

			ObjectMeta: metav1.ObjectMeta{
				Name:                       configMapName.Name,
				GenerateName:               "",
				Namespace:                  configMapName.Namespace,
				SelfLink:                   "",
				UID:                        "",
				ResourceVersion:            "",
				Generation:                 0,
				CreationTimestamp:          metav1.Time{},
				DeletionTimestamp:          &metav1.Time{},
				DeletionGracePeriodSeconds: new(int64),
				Labels:                     map[string]string{},
				Annotations:                map[string]string{},
				OwnerReferences:            []metav1.OwnerReference{},
				Finalizers:                 []string{},
				ClusterName:                "",
				ManagedFields:              []metav1.ManagedFieldsEntry{},
			},
			Immutable:  common.NewTrue(),
			BinaryData: map[string][]byte{},
		}
	}

	desired.Data = brokerPropertiesData(fsm.customResource.Spec.BrokerProperties)

	clog.V(1).Info("Requesting configMap for broker propertiesp", "name", configMapName.Name, "requests len", len(fsm.requestedResources))
	fsm.requestedResources = append(fsm.requestedResources, desired)

	clog.V(1).Info("Requesting mount for broker properties config map")
	return configMapName.Name
}

func HexShaHashOfMap(props []string) string {

	// sort the keys for consistency
	digest := adler32.New()
	for _, k := range props {
		digest.Write([]byte(k))
	}

	return hex.EncodeToString(digest.Sum(nil))
}

func brokerPropertiesData(props []string) map[string]string {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "# generated by crd")
	fmt.Fprintln(buf, "#")

	// sort the keys for consistency
	for _, k := range props {
		fmt.Fprintf(buf, "%s\n", k)
	}

	return map[string]string{"broker.properties": buf.String()}
}

func configureAffinity(podSpec *corev1.PodSpec, affinity *corev1.Affinity) {
	if affinity != nil {
		podSpec.Affinity = &corev1.Affinity{}
		if affinity.PodAffinity != nil {
			clog.V(1).Info("Adding Pod Affinity")
			podSpec.Affinity.PodAffinity = affinity.PodAffinity
		}
		if affinity.PodAntiAffinity != nil {
			clog.V(1).Info("Adding Pod AntiAffinity")
			podSpec.Affinity.PodAntiAffinity = affinity.PodAntiAffinity
		}
		if affinity.NodeAffinity != nil {
			clog.V(1).Info("Adding Node Affinity")
			podSpec.Affinity.NodeAffinity = affinity.NodeAffinity
		}
	}
}

func configurePodSecurityContext(podSpec *corev1.PodSpec, podSecurityContext *corev1.PodSecurityContext) {
	clog.V(1).Info("Configuring PodSecurityContext")

	if nil != podSecurityContext {
		clog.V(5).Info("Incoming podSecurityContext is NOT nil, assigning")
		podSpec.SecurityContext = podSecurityContext
	} else {
		clog.V(5).Info("Incoming podSecurityContext is nil, creating with default values")
		podSpec.SecurityContext = &corev1.PodSecurityContext{}
	}
}

func sortedKeys(props map[string]string) []string {
	sortedKeys := make([]string, 0, len(props))
	for k := range props {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}

// generic version?
func sortedKeysStringKeyByteValue(props map[string][]byte) []string {
	sortedKeys := make([]string, 0, len(props))
	for k := range props {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}

func configPodSecurity(podSpec *corev1.PodSpec, podSecurity *brokerv1beta1.PodSecurityType) {
	if podSecurity.ServiceAccountName != nil {
		clog.Info("Pod serviceAccountName specified", "existing", podSpec.ServiceAccountName, "new", *podSecurity.ServiceAccountName)
		podSpec.ServiceAccountName = *podSecurity.ServiceAccountName
	}
	if podSecurity.RunAsUser != nil {
		clog.Info("Pod runAsUser specified", "runAsUser", *podSecurity.RunAsUser)
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

func determineImageToUse(customResource *brokerv1beta1.ActiveMQArtemis, imageTypeName string) string {

	imageName := ""
	compactVersionToUse := determineCompactVersionToUse(customResource)

	genericRelatedImageEnvVarName := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_" + imageTypeName + "_" + compactVersionToUse
	// Default case of x86_64/amd64 covered here
	archSpecificRelatedImageEnvVarName := genericRelatedImageEnvVarName
	if "s390x" == osruntime.GOARCH || "ppc64le" == osruntime.GOARCH {
		archSpecificRelatedImageEnvVarName = genericRelatedImageEnvVarName + "_" + osruntime.GOARCH
	}
	clog.V(1).Info("DetermineImageToUse GOARCH specific image env var is " + archSpecificRelatedImageEnvVarName)
	imageName = os.Getenv(archSpecificRelatedImageEnvVarName)
	clog.V(1).Info("DetermineImageToUse imageName is " + imageName)

	return imageName
}

func getPodLabels(fsm *ActiveMQArtemisFSM, customResource *brokerv1beta1.ActiveMQArtemis) map[string]string {
	labels := fsm.namers.LabelBuilder.Labels()
	for k, v := range labels {
		labels[k] = v
	}

	compactVersion := determineCompactVersionToUse(customResource)
	fullVersion := version.FullVersionFromCompactVersion[compactVersion]

	additionalLabels := pods.GetAdditionalLabels(fullVersion)

	if additionalLabels != nil {
		clog.Info("Adding additional Labels", "broker version", fullVersion, "labels", additionalLabels)
		for k, v := range additionalLabels {
			labels[k] = v
		}
	}

	return labels
}

func determineCompactVersionToUse(customResource *brokerv1beta1.ActiveMQArtemis) string {

	specifiedVersion := customResource.Spec.Version
	compactVersionToUse := version.CompactLatestVersion
	//yacfgProfileVersion

	// See if we need to lookup what version to use
	for {
		// If there's no version specified just use the default above
		if 0 == len(specifiedVersion) {
			clog.V(1).Info("DetermineImageToUse specifiedVersion was empty")
			break
		}
		clog.V(1).Info("DetermineImageToUse specifiedVersion was " + specifiedVersion)

		// There is a version specified by the user...
		// Are upgrades enabled?
		if false == customResource.Spec.Upgrades.Enabled {
			clog.V(1).Info("DetermineImageToUse upgrades are disabled")
			break
		}
		clog.V(1).Info("DetermineImageToUse upgrades are enabled")

		// We have a specified version and upgrades are enabled in general
		// Is the version specified on "the list"
		compactSpecifiedVersion := version.CompactVersionFromVersion[specifiedVersion]
		if 0 == len(compactSpecifiedVersion) {
			clog.V(1).Info("DetermineImageToUse failed to find the compact form of the specified version " + specifiedVersion)
			break
		}
		clog.V(1).Info("DetermineImageToUse found the compact form " + compactSpecifiedVersion + " of specifiedVersion")

		// We found the compact form in our list, is it a minor bump?
		if version.LastMinorVersion == specifiedVersion &&
			!customResource.Spec.Upgrades.Minor {
			clog.V(1).Info("DetermineImageToUse requested minor version upgrade but minor upgrades NOT enabled")
			break
		}

		clog.V(1).Info("DetermineImageToUse all checks ok using user specified version " + specifiedVersion)
		compactVersionToUse = compactSpecifiedVersion
		break
	}

	return compactVersionToUse
}

func createExtraConfigmapsAndSecrets(brokerContainer *corev1.Container, configMaps []string, secrets []string) ([]corev1.Volume, []corev1.VolumeMount) {

	var extraVolumes []corev1.Volume
	var extraVolumeMounts []corev1.VolumeMount

	cfgMapPathBase := "/amq/extra/configmaps/"
	secretPathBase := "/amq/extra/secrets/"

	if len(configMaps) > 0 {
		for _, cfgmap := range configMaps {
			if cfgmap == "" {
				clog.Info("No ConfigMap name specified, ignore", "configMap", cfgmap)
				continue
			}
			cfgmapPath := cfgMapPathBase + cfgmap
			clog.Info("Resolved configMap path", "path", cfgmapPath)
			//now we have a config map. First create a volume
			cfgmapVol := volumes.MakeVolumeForConfigMap(cfgmap)
			cfgmapVolumeMount := volumes.MakeVolumeMountForCfg2(cfgmapVol.Name, cfgmapPath, true)
			extraVolumes = append(extraVolumes, cfgmapVol)
			extraVolumeMounts = append(extraVolumeMounts, cfgmapVolumeMount)
		}
	}

	if len(secrets) > 0 {
		for _, secret := range secrets {
			if secret == "" {
				clog.Info("No Secret name specified, ignore", "Secret", secret)
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

func NewStatefulSetForCR(fsm *ActiveMQArtemisFSM, currentStateFullSet *appsv1.StatefulSet) *appsv1.StatefulSet {

	reqLogger := ctrl.Log.WithName(fsm.customResource.Name)
	reqLogger.V(1).Info("NewStatefulSetForCR", "current", currentStateFullSet)

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}
	currentStateFullSet = ss.MakeStatefulSet2(currentStateFullSet, fsm.GetStatefulSetName(), fsm.GetHeadlessServiceName(), namespacedName, fsm.customResource.Annotations, fsm.namers.LabelBuilder.Labels(), fsm.customResource.Spec.DeploymentPlan.Size)

	podTemplateSpec := *NewPodTemplateSpecForCR(fsm, &currentStateFullSet.Spec.Template)
	if fsm.customResource.Spec.DeploymentPlan.PersistenceEnabled {
		currentStateFullSet.Spec.VolumeClaimTemplates = *NewPersistentVolumeClaimArrayForCR(fsm, 1)
	}
	currentStateFullSet.Spec.Template = podTemplateSpec

	return currentStateFullSet
}

func NewPersistentVolumeClaimArrayForCR(fsm *ActiveMQArtemisFSM, arrayLength int) *[]corev1.PersistentVolumeClaim {

	var pvc *corev1.PersistentVolumeClaim = nil
	capacity := "2Gi"
	pvcArray := make([]corev1.PersistentVolumeClaim, 0, arrayLength)
	storageClassName := ""

	namespacedName := types.NamespacedName{
		Name:      fsm.customResource.Name,
		Namespace: fsm.customResource.Namespace,
	}

	if "" != fsm.customResource.Spec.DeploymentPlan.Storage.Size {
		capacity = fsm.customResource.Spec.DeploymentPlan.Storage.Size
	}

	if "" != fsm.customResource.Spec.DeploymentPlan.Storage.StorageClassName {
		storageClassName = fsm.customResource.Spec.DeploymentPlan.Storage.StorageClassName
	}

	for i := 0; i < arrayLength; i++ {
		pvc = persistentvolumeclaims.NewPersistentVolumeClaimWithCapacityAndStorageClassName(namespacedName, capacity, fsm.namers.LabelBuilder.Labels(), storageClassName)
		pvcArray = append(pvcArray, *pvc)
	}

	return &pvcArray
}

func UpdatePodStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName) error {

	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)
	reqLogger.V(1).Info("Updating status for pods")

	podStatus := GetPodStatus(cr, client, namespacedName)

	reqLogger.V(1).Info("PodStatus are to be updated.............................", "info:", podStatus)
	reqLogger.V(1).Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.V(1).Info("Stopped Count........................", "info:", len(podStatus.Stopped))
	reqLogger.V(1).Info("Starting Count........................", "info:", len(podStatus.Starting))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		cr.Status.PodStatus = podStatus

		err := resources.UpdateStatus(namespacedName, client, cr)
		if err != nil {
			reqLogger.Error(err, "Failed to update pods status")
			return err
		}
		reqLogger.Info("Pods status updated")
		return nil
	}

	return nil
}

func GetPodStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {

	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.V(1).Info("Getting status for pods")

	var status olm.DeploymentStatus
	var lastStatus olm.DeploymentStatus

	if lastStatus, lastStatusExist := lastStatusMap[namespacedName]; !lastStatusExist {
		ctrl.Log.Info("Creating lastStatus for new CR", "name", namespacedName)
		lastStatus = olm.DeploymentStatus{}
		lastStatusMap[namespacedName] = lastStatus
	}

	ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(namespacedName.Name), Namespace: namespacedName.Namespace}
	sfsFound := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), ssNamespacedName, sfsFound)
	if err == nil {
		status = getSingleStatefulSetStatus(sfsFound)
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
	lastStatusMap[namespacedName] = status

	return status
}

func getSingleStatefulSetStatus(ss *appsv1.StatefulSet) olm.DeploymentStatus {
	var ready, starting, stopped []string
	var requestedCount = int32(0)
	if ss.Spec.Replicas != nil {
		requestedCount = *ss.Spec.Replicas
	}

	targetCount := ss.Status.Replicas
	readyCount := ss.Status.ReadyReplicas

	if requestedCount == 0 || targetCount == 0 {
		stopped = append(stopped, ss.Name)
	} else {
		for i := int32(0); i < targetCount; i++ {
			instanceName := fmt.Sprintf("%s-%d", ss.Name, i)
			if i < readyCount {
				ready = append(ready, instanceName)
			} else {
				starting = append(starting, instanceName)
			}
		}
	}
	return olm.DeploymentStatus{
		Stopped:  stopped,
		Starting: starting,
		Ready:    ready,
	}
}

func MakeEnvVarArrayForCR(fsm *ActiveMQArtemisFSM) []corev1.EnvVar {

	reqLogger := clog.WithName(fsm.customResource.Name)
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

	sortEnvVars(envVar)

	return envVar
}

func sortEnvVars(envVar []corev1.EnvVar) {
	// sort for easy reconcile
	sort.SliceStable(envVar, func(i, j int) bool { return envVar[i].Name < envVar[j].Name })
}
