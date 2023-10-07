package controllers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/adler32"
	osruntime "runtime"
	"sort"
	"unicode"

	"github.com/blang/semver/v4"

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
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/jolokia_client"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

	policyv1 "k8s.io/api/policy/v1"
)

const (
	ImageNamePrefix                  = "RELATED_IMAGE_ActiveMQ_Artemis_Broker_"
	BrokerImageKey                   = "Kubernetes"
	InitImageKey                     = "Init"
	defaultLivenessProbeInitialDelay = 5
	TCPLivenessPort                  = 8161
	jaasConfigSuffix                 = "-jaas-config"
	loggingConfigSuffix              = "-logging-config"

	cfgMapPathBase = "/amq/extra/configmaps/"
	secretPathBase = "/amq/extra/secrets/"

	OrdinalPrefix         = "broker-"
	OrdinalPrefixSep      = "."
	BrokerPropertiesName  = "broker.properties"
	JaasConfigKey         = "login.config"
	LoggingConfigKey      = "logging.properties"
	DefaultDeploymentSize = int32(1)
)

var defaultMessageMigration bool = true
var lastStatusMap map[types.NamespacedName]olm.DeploymentStatus = make(map[types.NamespacedName]olm.DeploymentStatus)

// the helper script looks for "/amq/scripts/post-config.sh"
// and run it if exists.
var initHelperScript = "/opt/amq-broker/script/default.sh"
var brokerConfigRoot = "/amq/init/config"
var configCmd = "/opt/amq/bin/launch.sh"

// default ApplyRule for address-settings
var defApplyRule string = "merge_all"
var yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.LatestVersion]

type ActiveMQArtemisReconcilerImpl struct {
	requestedResources []rtclient.Object
	deployed           map[reflect.Type][]rtclient.Object
}

type ValueInfo struct {
	Value    string
	AutoGen  bool
	Internal bool //if true put this value to the internal secret
}

type ActiveMQArtemisIReconciler interface {
	Process(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, firstTime bool) uint32
	ProcessStatefulSet(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, log logr.Logger, firstTime bool) (*appsv1.StatefulSet, bool)
	ProcessCredentials(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessDeploymentPlan(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet, firstTime bool) uint32
	ProcessAcceptorsAndConnectors(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessConsole(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet)
	ProcessResources(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) uint8
}

func (reconciler *ActiveMQArtemisReconcilerImpl) Process(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme) error {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")
	log.Info("Reconciler Processing...", "Operator version", version.Version, "ActiveMQArtemis release", customResource.Spec.Version)
	log.Info("Reconciler Processing...", "CRD.Name", customResource.Name, "CRD ver", customResource.ObjectMeta.ResourceVersion, "CRD Gen", customResource.ObjectMeta.Generation)

	reconciler.CurrentDeployedResources(customResource, client)

	// currentStateful Set is a clone of what exists if already deployed
	// what follows should transform the resources using the crd
	// if the transformation results in some change, process resources will respect that
	// comparisons should not be necessary, leave that to process resources
	desiredStatefulSet, err := reconciler.ProcessStatefulSet(customResource, namer, client, log)
	if err != nil {
		log.Error(err, "Error processing stafulset")
		return err
	}

	reconciler.ProcessDeploymentPlan(customResource, namer, client, scheme, desiredStatefulSet)

	reconciler.ProcessCredentials(customResource, namer, client, scheme, desiredStatefulSet)

	reconciler.ProcessAcceptorsAndConnectors(customResource, namer, client, scheme, desiredStatefulSet)

	reconciler.ProcessConsole(customResource, namer, client, scheme, desiredStatefulSet)

	// mods to env var values sourced from secrets are not detected by process resources
	// track updates in trigger env var that has a total checksum
	trackSecretCheckSumInEnvVar(reconciler.requestedResources, desiredStatefulSet.Spec.Template.Spec.Containers)

	reconciler.trackDesired(desiredStatefulSet)

	// this should apply any deltas/updates
	err = reconciler.ProcessResources(customResource, client, scheme)

	log.Info("Reconciler Processing... complete", "CRD ver:", customResource.ObjectMeta.ResourceVersion, "CRD Gen:", customResource.ObjectMeta.Generation)

	// we dont't requeue
	return err
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

func (reconciler *ActiveMQArtemisReconcilerImpl) cloneOfDeployed(kind reflect.Type, name string) rtclient.Object {
	obj := reconciler.getFromDeployed(kind, name)
	if obj != nil {
		return obj.DeepCopyObject().(rtclient.Object)
	}
	return nil
}

func (reconciler *ActiveMQArtemisReconcilerImpl) getFromDeployed(kind reflect.Type, name string) rtclient.Object {
	for _, obj := range reconciler.deployed[kind] {
		if obj.GetName() == name {
			return obj
		}
	}
	return nil
}

func (reconciler *ActiveMQArtemisReconcilerImpl) addToDeployed(kind reflect.Type, obj rtclient.Object) {
	reconciler.deployed[kind] = append(reconciler.deployed[kind], obj)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessStatefulSet(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, log logr.Logger) (*appsv1.StatefulSet, error) {

	reqLogger := ctrl.Log.WithName(customResource.Name)

	ssNamespacedName := types.NamespacedName{
		Namespace: customResource.Namespace,
		Name:      namer.SsNameBuilder.Name(),
	}

	var err error
	var currentStatefulSet *appsv1.StatefulSet
	obj := reconciler.cloneOfDeployed(reflect.TypeOf(appsv1.StatefulSet{}), ssNamespacedName.Name)
	if obj != nil {
		currentStatefulSet = obj.(*appsv1.StatefulSet)
	}

	log.V(3).Info("Reconciling desired statefulset", "name", ssNamespacedName, "current", currentStatefulSet)
	currentStatefulSet, err = reconciler.NewStatefulSetForCR(customResource, namer, currentStatefulSet, client)
	if err != nil {
		reqLogger.Error(err, "Error creating new stafulset")
		return nil, err
	}

	labels := namer.LabelBuilder.Labels()
	headlessServiceDefinition := svc.NewHeadlessServiceForCR2(client, namer.SvcHeadlessNameBuilder.Name(), ssNamespacedName.Namespace, serviceports.GetDefaultPorts(), labels)
	if isClustered(customResource) {
		pingServiceDefinition := svc.NewPingServiceDefinitionForCR2(client, namer.SvcPingNameBuilder.Name(), ssNamespacedName.Namespace, labels, labels)
		reconciler.trackDesired(pingServiceDefinition)
	}
	reconciler.trackDesired(headlessServiceDefinition)

	if customResource.Spec.DeploymentPlan.RevisionHistoryLimit != nil {
		currentStatefulSet.Spec.RevisionHistoryLimit = customResource.Spec.DeploymentPlan.RevisionHistoryLimit
	}
	return currentStatefulSet, nil
}

func isClustered(customResource *brokerv1beta1.ActiveMQArtemis) bool {
	if customResource.Spec.DeploymentPlan.Clustered != nil {
		return *customResource.Spec.DeploymentPlan.Clustered
	}
	return true
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessCredentials(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

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
	secretName := namer.SecretsCredentialsNameBuilder.Name()
	adminUser.Value = customResource.Spec.AdminUser
	if adminUser.Value == "" {
		if amqUserEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_USER"); nil != amqUserEnvVar {
			adminUser.Value = amqUserEnvVar.Value
		}
		if adminUser.Value == "" {
			adminUser.Value = environments.Defaults.AMQ_USER
			adminUser.AutoGen = true
		}
	}
	envVars["AMQ_USER"] = adminUser

	adminPassword.Value = customResource.Spec.AdminPassword
	if adminPassword.Value == "" {
		if amqPasswordEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_PASSWORD"); nil != amqPasswordEnvVar {
			adminPassword.Value = amqPasswordEnvVar.Value
		}
		if adminPassword.Value == "" {
			adminPassword.Value = environments.Defaults.AMQ_PASSWORD
			adminPassword.AutoGen = true
		}
	}
	envVars["AMQ_PASSWORD"] = adminPassword

	envVars["AMQ_CLUSTER_USER"] = ValueInfo{
		Value:   environments.GLOBAL_AMQ_CLUSTER_USER,
		AutoGen: true,
	}
	envVars["AMQ_CLUSTER_PASSWORD"] = ValueInfo{
		Value:   environments.GLOBAL_AMQ_CLUSTER_PASSWORD,
		AutoGen: true,
	}

	reconciler.sourceEnvVarFromSecret(customResource, namer, currentStatefulSet, &envVars, secretName, client, scheme)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessDeploymentPlan(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

	deploymentPlan := &customResource.Spec.DeploymentPlan

	clog.V(3).Info("Processing deployment plan", "plan", deploymentPlan, "broker cr", customResource.Name)
	// Ensure the StatefulSet size is the same as the spec
	replicas := getDeploymentSize(customResource)
	currentStatefulSet.Spec.Replicas = &replicas

	clog.V(3).Info("Now sync Message migration", "for cr", customResource.Name)
	syncMessageMigration(customResource, namer, client, scheme)

	if customResource.Spec.DeploymentPlan.PodDisruptionBudget != nil {
		reconciler.applyPodDisruptionBudget(customResource, client, currentStatefulSet)
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) applyPodDisruptionBudget(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, currentStatefulSet *appsv1.StatefulSet) {

	var desired *policyv1.PodDisruptionBudget
	obj := reconciler.cloneOfDeployed(reflect.TypeOf(policyv1.PodDisruptionBudget{}), customResource.Name+"-pdb")

	if obj != nil {
		desired = obj.(*policyv1.PodDisruptionBudget)
	} else {
		desired = &policyv1.PodDisruptionBudget{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "policy/v1",
				Kind:       "PodDisruptionBudget",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      customResource.Name + "-pdb",
				Namespace: customResource.Namespace,
			},
		}
	}
	desired.Spec = *customResource.Spec.DeploymentPlan.PodDisruptionBudget.DeepCopy()
	matchLabels := map[string]string{"ActiveMQArtemis": customResource.Name}

	desired.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}

	reconciler.trackDesired(desired)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessAcceptorsAndConnectors(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

	acceptorEntry := generateAcceptorsString(customResource, namer, client)
	connectorEntry := generateConnectorsString(customResource, namer, client)

	reconciler.configureAcceptorsExposure(customResource, namer, client, scheme)
	reconciler.configureConnectorsExposure(customResource, namer, client, scheme)

	envVars := make(map[string]ValueInfo)

	envVars["AMQ_ACCEPTORS"] = ValueInfo{
		Value:   acceptorEntry,
		AutoGen: false,
	}

	envVars["AMQ_CONNECTORS"] = ValueInfo{
		Value:   connectorEntry,
		AutoGen: false,
	}

	secretName := namer.SecretsNettyNameBuilder.Name()
	reconciler.sourceEnvVarFromSecret(customResource, namer, currentStatefulSet, &envVars, secretName, client, scheme)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessConsole(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

	reconciler.configureConsoleExposure(customResource, namer, client, scheme)
	if !customResource.Spec.Console.SSLEnabled {
		return
	}

	secretName := namer.SecretsConsoleNameBuilder.Name()

	envVars := map[string]ValueInfo{"AMQ_CONSOLE_ARGS": {
		Value:    generateConsoleSSLFlags(customResource, namer, client, secretName),
		AutoGen:  true,
		Internal: true,
	}}

	reconciler.sourceEnvVarFromSecret(customResource, namer, currentStatefulSet, &envVars, secretName, client, scheme)
}

func syncMessageMigration(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme) {

	var err error = nil
	var retrieveError error = nil

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	ssNames := make(map[string]string)
	ssNames["CRNAMESPACE"] = customResource.Namespace
	ssNames["CRNAME"] = customResource.Name
	ssNames["CLUSTERUSER"] = environments.GLOBAL_AMQ_CLUSTER_USER
	ssNames["CLUSTERPASS"] = environments.GLOBAL_AMQ_CLUSTER_PASSWORD
	ssNames["HEADLESSSVCNAMEVALUE"] = namer.SvcHeadlessNameBuilder.Name()
	ssNames["PINGSVCNAMEVALUE"] = namer.SvcPingNameBuilder.Name()
	ssNames["SERVICE_ACCOUNT"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["SERVICE_ACCOUNT_NAME"] = os.Getenv("SERVICE_ACCOUNT")
	ssNames["AMQ_CREDENTIALS_SECRET_NAME"] = namer.SecretsCredentialsNameBuilder.Name()

	scaledown := &brokerv1beta1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      namer.LabelBuilder.Labels(),
			Name:        customResource.Name,
			Namespace:   customResource.Namespace,
			Annotations: ssNames,
		},
		Spec: brokerv1beta1.ActiveMQArtemisScaledownSpec{
			LocalOnly: isLocalOnly(),
			Resources: customResource.Spec.DeploymentPlan.Resources,
		},
		Status: brokerv1beta1.ActiveMQArtemisScaledownStatus{},
	}

	if nil == customResource.Spec.DeploymentPlan.MessageMigration {
		customResource.Spec.DeploymentPlan.MessageMigration = &defaultMessageMigration
	}

	clustered := isClustered(customResource)

	if *customResource.Spec.DeploymentPlan.MessageMigration && clustered {
		if !customResource.Spec.DeploymentPlan.PersistenceEnabled {
			clog.V(2).Info("Won't set up scaledown for non persistent deployment")
			return
		}
		clog.V(3).Info("we need scaledown for this cr", "crName", customResource.Name, "scheme", scheme)
		if err = resources.Retrieve(namespacedName, client, scaledown); err != nil {
			// err means not found so create
			clog.V(2).Info("Creating builtin drainer CR ", "scaledown", scaledown)
			if retrieveError = resources.Create(customResource, client, scheme, scaledown); retrieveError == nil {
				clog.Info("drainer created successfully", "drainer", scaledown)
			} else {
				clog.Error(retrieveError, "we have error retrieving drainer", "drainer", scaledown, "scheme", scheme)
			}
		}
	} else {
		if err = resources.Retrieve(namespacedName, client, scaledown); err == nil {
			//	ReleaseController(customResource.Name)
			// err means not found so delete
			resources.Delete(client, scaledown)
		}
	}
}

func isLocalOnly() bool {
	oprNamespace := os.Getenv("OPERATOR_NAMESPACE")
	watchNamespace := os.Getenv("OPERATOR_WATCH_NAMESPACE")
	return oprNamespace == watchNamespace
}

func (reconciler *ActiveMQArtemisReconcilerImpl) sourceEnvVarFromSecret(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]ValueInfo, secretName string, client rtclient.Client, scheme *runtime.Scheme) {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis").WithName("sourceEnvVarFromSecret")

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

	secretDefinition := secrets.NewSecret(namespacedName, secretName, stringDataMap, namer.LabelBuilder.Labels())

	desired := false
	if err := resources.Retrieve(namespacedName, client, secretDefinition); err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(2).Info("Did not find secret", "key", namespacedName)
		} else {
			log.Error(err, "Error while retrieving secret", "key", namespacedName)
		}
		if len(stringDataMap) > 0 {
			// we need to create and track
			desired = true
		}
	} else {
		log.V(1).Info("updating from " + secretName)
		for k, envVar := range *envVars {
			if envVar.Internal {
				// goes in internal secret
				continue
			}
			elem, exists := secretDefinition.Data[k]
			if strings.Compare(string(elem), (*envVars)[k].Value) != 0 || !exists {
				log.V(2).Info("key value not equals or does not exist", "key", k, "exists", exists)
				if !(*envVars)[k].AutoGen || string(elem) == "" {
					secretDefinition.Data[k] = []byte((*envVars)[k].Value)
				}
			}
		}
		//if operator doesn't own it, don't track
		if len(secretDefinition.OwnerReferences) > 0 {
			for _, or := range secretDefinition.OwnerReferences {
				if or.Kind == "ActiveMQArtemis" && or.Name == customResource.Name {
					desired = true
				}
			}
		}
	}

	if desired {
		// ensure processResources sees it
		reconciler.trackDesired(secretDefinition)
	}

	internalSecretName := secretName + "-internal"
	if len(internalStringDataMap) > 0 {

		var internalSecretDefinition *corev1.Secret
		obj := reconciler.cloneOfDeployed(reflect.TypeOf(corev1.Secret{}), internalSecretName)
		if obj != nil {
			log.V(2).Info("updating from " + internalSecretName)
			internalSecretDefinition = obj.(*corev1.Secret)
		} else {
			internalSecretDefinition = secrets.NewSecret(namespacedName, internalSecretName, internalStringDataMap, namer.LabelBuilder.Labels())
			reconciler.adoptExistingSecretWithNoOwnerRefForUpdate(customResource, internalSecretDefinition, client)
		}

		// apply our desired data
		internalSecretDefinition.ObjectMeta.Labels = namer.LabelBuilder.Labels()
		internalSecretDefinition.StringData = internalStringDataMap

		reconciler.trackDesired(internalSecretDefinition)
	}

	log.V(2).Info("Populating env vars references, in order, from secret " + secretName)

	// sort the keys for consistency of reconcilitation as iteration source is a map
	sortedKeys := make([]string, 0, len(*envVars))
	for k := range *envVars {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, envVarName := range sortedKeys {
		envVarInfo := (*envVars)[envVarName]
		secretNameToUse := secretName
		if envVarInfo.Internal {
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
			log.V(3).Info("containers: failed to retrieve " + envVarName + " creating")
			environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, envVarDefinition)
		}

		//custom init container
		if len(currentStatefulSet.Spec.Template.Spec.InitContainers) > 0 {
			if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarName); nil == retrievedEnvVar {
				log.V(3).Info("init_containers: failed to retrieve " + envVarName + " creating")
				environments.Create(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarDefinition)
			}
		}
	}

}

func generateAcceptorsString(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client) string {

	// TODO: Optimize for the single broker configuration
	ensureCOREOn61616Exists := true // as clustered is no longer an option but true by default

	acceptorEntry := ""
	defaultArgs := "tcpSendBufferSize=1048576;tcpReceiveBufferSize=1048576;useEpoll=true;amqpCredits=1000;amqpMinCredits=300"

	var portIncrement int32 = 10
	var currentPortIncrement int32 = 0
	var port61616InUse bool = false
	var i uint32 = 0
	for _, acceptor := range customResource.Spec.Acceptors {
		if acceptor.Port == 0 {
			acceptor.Port = 61626 + currentPortIncrement
			currentPortIncrement += portIncrement
			customResource.Spec.Acceptors[i].Port = acceptor.Port
		}
		if acceptor.Protocols == "" ||
			strings.ToLower(acceptor.Protocols) == "all" {
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
		if acceptor.Port == 61616 {
			port61616InUse = true
		}
		if ensureCOREOn61616Exists &&
			(acceptor.Port == 61616) &&
			!strings.Contains(strings.ToUpper(acceptor.Protocols), "CORE") {
			acceptorEntry = acceptorEntry + ",CORE"
		}
		if acceptor.SSLEnabled {
			secretName := customResource.Name + "-" + acceptor.Name + "-secret"
			if acceptor.SSLSecret != "" {
				secretName = acceptor.SSLSecret
			}
			acceptorEntry = acceptorEntry + ";" + generateAcceptorConnectorSSLArguments(customResource, namer, client, secretName)
			sslOptionalArguments := generateAcceptorSSLOptionalArguments(acceptor)
			if sslOptionalArguments != "" {
				acceptorEntry = acceptorEntry + ";" + sslOptionalArguments
			}
		}
		if acceptor.AnycastPrefix != "" {
			safeAnycastPrefix := strings.Replace(acceptor.AnycastPrefix, "/", "\\/", -1)
			acceptorEntry = acceptorEntry + ";" + "anycastPrefix=" + safeAnycastPrefix
		}
		if acceptor.MulticastPrefix != "" {
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

func generateConnectorsString(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client) string {

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
			if connector.SSLSecret != "" {
				secretName = connector.SSLSecret
			}
			connectorEntry = connectorEntry + ";" + generateAcceptorConnectorSSLArguments(customResource, namer, client, secretName)
			sslOptionalArguments := generateConnectorSSLOptionalArguments(connector)
			if sslOptionalArguments != "" {
				connectorEntry = connectorEntry + ";" + sslOptionalArguments
			}
		}
		connectorEntry = connectorEntry + "<\\/connector>"
	}

	return connectorEntry
}

func (reconciler *ActiveMQArtemisReconcilerImpl) configureAcceptorsExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme) {
	originalLabels := namer.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	deploymentSize := getDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = namer.SsNameBuilder.Name() + "-" + ordinalString

		for _, acceptor := range customResource.Spec.Acceptors {
			serviceDefinition := svc.NewServiceDefinitionForCR("", client, namespacedName, acceptor.Name+"-"+ordinalString, acceptor.Port, serviceRoutelabels, namer.LabelBuilder.Labels())

			reconciler.checkExistingService(customResource, serviceDefinition, client)
			reconciler.trackDesired(serviceDefinition)

			if acceptor.Expose {
				exposureDefinition := reconciler.ExposureDefinitionForCR(customResource, namespacedName, serviceRoutelabels, acceptor.SSLEnabled, acceptor.IngressHost, ordinalString, acceptor.Name)
				reconciler.trackDesired(exposureDefinition)
			}
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ExposureDefinitionForCR(customResource *brokerv1beta1.ActiveMQArtemis, namespacedName types.NamespacedName, labels map[string]string, passthroughTLS bool, ingressHost string, ordinalString string, itemName string) rtclient.Object {

	targetPortName := itemName + "-" + ordinalString
	targetServiceName := customResource.Name + "-" + targetPortName + "-svc"

	if isOpenshift, err := environments.DetectOpenshift(); isOpenshift && err == nil {
		clog.Info("creating route for "+targetPortName, "service", targetServiceName)

		var existing *routev1.Route = nil
		obj := reconciler.cloneOfDeployed(reflect.TypeOf(routev1.Route{}), targetServiceName+"-rte")
		if obj != nil {
			existing = obj.(*routev1.Route)
		}
		brokerHost := formatIngressHost(customResource, ingressHost, ordinalString, itemName, "rte")
		return routes.NewRouteDefinitionForCR(existing, namespacedName, labels, targetServiceName, targetPortName, passthroughTLS, customResource.Spec.IngressDomain, brokerHost)
	} else {
		clog.Info("creating ingress for "+targetPortName, "service", targetServiceName)

		var existing *netv1.Ingress = nil
		obj := reconciler.cloneOfDeployed(reflect.TypeOf(netv1.Ingress{}), targetServiceName+"-ing")
		if obj != nil {
			existing = obj.(*netv1.Ingress)
		}
		brokerHost := formatIngressHost(customResource, ingressHost, ordinalString, itemName, "ing")
		return ingresses.NewIngressForCRWithSSL(existing, namespacedName, labels, targetServiceName, targetPortName, passthroughTLS, customResource.Spec.IngressDomain, brokerHost)
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) trackDesired(desired rtclient.Object) {
	reconciler.requestedResources = append(reconciler.requestedResources, desired)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) configureConnectorsExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme) {
	originalLabels := namer.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	deploymentSize := getDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = namer.SsNameBuilder.Name() + "-" + ordinalString

		for _, connector := range customResource.Spec.Connectors {
			serviceDefinition := svc.NewServiceDefinitionForCR("", client, namespacedName, connector.Name+"-"+ordinalString, connector.Port, serviceRoutelabels, namer.LabelBuilder.Labels())
			reconciler.checkExistingService(customResource, serviceDefinition, client)
			reconciler.trackDesired(serviceDefinition)

			if connector.Expose {

				exposureDefinition := reconciler.ExposureDefinitionForCR(customResource, namespacedName, serviceRoutelabels, connector.SSLEnabled, connector.IngressHost, ordinalString, connector.Name)

				reconciler.trackDesired(exposureDefinition)
			}
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) configureConsoleExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, scheme *runtime.Scheme) {
	console := customResource.Spec.Console
	consoleName := customResource.Spec.Console.Name

	if consoleName == "" {
		consoleName = "wconsj"
	}

	originalLabels := namer.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	targetPort := int32(8161)
	portNumber := int32(8162)
	deploymentSize := getDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels["statefulset.kubernetes.io/pod-name"] = namer.SsNameBuilder.Name() + "-" + ordinalString

		targetPortName := consoleName + "-" + ordinalString
		targetServiceName := customResource.Name + "-" + targetPortName + "-svc"

		serviceDefinition := svc.NewServiceDefinitionForCR(targetServiceName, client, namespacedName, consoleName, targetPort, serviceRoutelabels, namer.LabelBuilder.Labels())

		serviceDefinition.Spec.Ports = append(serviceDefinition.Spec.Ports, corev1.ServicePort{
			Name:       targetPortName,
			Port:       portNumber,
			Protocol:   "TCP",
			TargetPort: intstr.FromInt(int(targetPort)),
		})
		if console.Expose {
			reconciler.checkExistingService(customResource, serviceDefinition, client)
			reconciler.trackDesired(serviceDefinition)

			isOpenshift := false
			isOpenshift, _ = environments.DetectOpenshift()
			if isOpenshift {
				clog.V(2).Info("routeDefinition for " + targetPortName)
				var existing *routev1.Route = nil
				obj := reconciler.cloneOfDeployed(reflect.TypeOf(routev1.Route{}), targetServiceName+"-rte")
				if obj != nil {
					existing = obj.(*routev1.Route)
				}
				brokerHost := formatIngressHost(customResource, customResource.Spec.Console.IngressHost, ordinalString, consoleName, "rte")
				routeDefinition := routes.NewRouteDefinitionForCR(existing, namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled, customResource.Spec.IngressDomain, brokerHost)
				reconciler.trackDesired(routeDefinition)

			} else {
				clog.V(2).Info("ingress for " + targetPortName)
				var existing *netv1.Ingress = nil
				obj := reconciler.cloneOfDeployed(reflect.TypeOf(netv1.Ingress{}), targetServiceName+"-ing")
				if obj != nil {
					existing = obj.(*netv1.Ingress)
				}
				brokerHost := formatIngressHost(customResource, customResource.Spec.Console.IngressHost, ordinalString, consoleName, "ing")
				ingressDefinition := ingresses.NewIngressForCRWithSSL(existing, namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled, customResource.Spec.IngressDomain, brokerHost)
				reconciler.trackDesired(ingressDefinition)
			}
		}
	}
}

func formatIngressHost(customResource *brokerv1beta1.ActiveMQArtemis, ingressHost string, brokerOrdinal string, itemName string, resType string) string {

	if ingressHost != "" {
		if strings.Contains(ingressHost, "$(CR_NAME)") {
			ingressHost = strings.Replace(ingressHost, "$(CR_NAME)", customResource.Name, -1)
		}
		if strings.Contains(ingressHost, "$(CR_NAMESPACE)") {
			ingressHost = strings.Replace(ingressHost, "$(CR_NAMESPACE)", customResource.Namespace, -1)
		}
		if strings.Contains(ingressHost, "$(BROKER_ORDINAL)") {
			ingressHost = strings.Replace(ingressHost, "$(BROKER_ORDINAL)", brokerOrdinal, -1)
		}
		if strings.Contains(ingressHost, "$(ITEM_NAME)") {
			ingressHost = strings.Replace(ingressHost, "$(ITEM_NAME)", itemName, -1)
		}
		if strings.Contains(ingressHost, "$(RES_TYPE)") {
			ingressHost = strings.Replace(ingressHost, "$(RES_TYPE)", resType, -1)
		}
		if strings.Contains(ingressHost, "$(INGRESS_DOMAIN)") {
			ingressHost = strings.Replace(ingressHost, "$(INGRESS_DOMAIN)", customResource.Spec.IngressDomain, -1)
		}
	}

	return ingressHost
}

func generateConsoleSSLFlags(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, secretName string) string {

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
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, namer.LabelBuilder.Labels())

	keyStorePassword := "password"
	keyStorePath := "/etc/" + secretName + "-volume/broker.ks"
	trustStorePassword := "password"
	trustStorePath := "/etc/" + secretName + "-volume/client.ts"
	if err := resources.Retrieve(secretNamespacedName, client, userPasswordSecret); err == nil {
		if string(userPasswordSecret.Data["keyStorePassword"]) != "" {
			keyStorePassword = string(userPasswordSecret.Data["keyStorePassword"])
		}
		if string(userPasswordSecret.Data["keyStorePath"]) != "" {
			keyStorePath = string(userPasswordSecret.Data["keyStorePath"])
		}
		if string(userPasswordSecret.Data["trustStorePassword"]) != "" {
			trustStorePassword = string(userPasswordSecret.Data["trustStorePassword"])
		}
		if string(userPasswordSecret.Data["trustStorePath"]) != "" {
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

func generateAcceptorConnectorSSLArguments(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, client rtclient.Client, secretName string) string {

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
	userPasswordSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, namer.LabelBuilder.Labels())

	keyStorePassword := "password"
	keyStorePath := "\\/etc\\/" + secretName + "-volume\\/broker.ks"
	trustStorePassword := "password"
	trustStorePath := "\\/etc\\/" + secretName + "-volume\\/client.ts"
	if err := resources.Retrieve(secretNamespacedName, client, userPasswordSecret); err == nil {
		if string(userPasswordSecret.Data["keyStorePassword"]) != "" {
			//noinspection GoUnresolvedReference
			keyStorePassword = strings.ReplaceAll(string(userPasswordSecret.Data["keyStorePassword"]), "/", "\\/")
		}
		if string(userPasswordSecret.Data["keyStorePath"]) != "" {
			//noinspection GoUnresolvedReference
			keyStorePath = strings.ReplaceAll(string(userPasswordSecret.Data["keyStorePath"]), "/", "\\/")
		}
		if string(userPasswordSecret.Data["trustStorePassword"]) != "" {
			//noinspection GoUnresolvedReference
			trustStorePassword = strings.ReplaceAll(string(userPasswordSecret.Data["trustStorePassword"]), "/", "\\/")
		}
		if string(userPasswordSecret.Data["trustStorePath"]) != "" {
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

	if acceptor.EnabledCipherSuites != "" {
		sslOptionalArguments = sslOptionalArguments + "enabledCipherSuites=" + acceptor.EnabledCipherSuites
	}
	if acceptor.EnabledProtocols != "" {
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
	if acceptor.SSLProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "sslProvider=" + acceptor.SSLProvider
	}
	if acceptor.SNIHost != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "sniHost=" + acceptor.SNIHost
	}

	if acceptor.KeyStoreProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "keyStoreProvider=" + acceptor.KeyStoreProvider
	}

	if acceptor.TrustStoreType != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreType=" + acceptor.TrustStoreType
	}

	if acceptor.TrustStoreProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreProvider=" + acceptor.TrustStoreProvider
	}

	return sslOptionalArguments
}

func generateConnectorSSLOptionalArguments(connector brokerv1beta1.ConnectorType) string {

	sslOptionalArguments := ""

	if connector.EnabledCipherSuites != "" {
		sslOptionalArguments = sslOptionalArguments + "enabledCipherSuites=" + connector.EnabledCipherSuites
	}
	if connector.EnabledProtocols != "" {
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
	if connector.SSLProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "sslProvider=" + connector.SSLProvider
	}
	if connector.SNIHost != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "sniHost=" + connector.SNIHost
	}

	if connector.KeyStoreProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "keyStoreProvider=" + connector.KeyStoreProvider
	}

	if connector.TrustStoreType != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreType=" + connector.TrustStoreType
	}

	if connector.TrustStoreProvider != "" {
		sslOptionalArguments = sslOptionalArguments + ";" + "trustStoreProvider=" + connector.TrustStoreProvider
	}

	return sslOptionalArguments
}

func (reconciler *ActiveMQArtemisReconcilerImpl) CurrentDeployedResources(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) {
	reqLogger := clog.WithValues("ActiveMQArtemis Name", customResource.Name)
	reqLogger.Info("currentDeployedResources")

	var err error
	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		checkExistingPersistentVolumes(customResource, client)
	}
	reconciler.deployed, err = getDeployedResources(customResource, client)
	if err != nil {
		reqLogger.Error(err, "error getting deployed resources")
		return
	}

	// track persisted cr secret
	for _, secret := range reconciler.deployed[reflect.TypeOf(corev1.Secret{})] {
		if strings.HasPrefix(secret.GetName(), "secret-broker-") {
			// track this as it is managed by the controller state machine, not by reconcile
			reconciler.trackDesired(secret)
		}
	}

	for t, objs := range reconciler.deployed {
		for _, obj := range objs {
			reqLogger.V(3).Info("Deployed ", "Type", t, "Name", obj.GetName())
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessResources(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) error {

	reqLogger := clog.WithValues("ActiveMQArtemis Name", customResource.Name)

	for index := range reconciler.requestedResources {
		reconciler.requestedResources[index].SetNamespace(customResource.Namespace)
	}

	var currenCount int
	for index := range reconciler.deployed {
		currenCount += len(reconciler.deployed[index])
	}
	reqLogger.Info("Processing resources", "num requested", len(reconciler.requestedResources), "num current", currenCount)

	requested := compare.NewMapBuilder().Add(reconciler.requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()

	comparator.Comparator.SetComparator(reflect.TypeOf(appsv1.StatefulSet{}), func(deployed, requested rtclient.Object) bool {
		ss1 := deployed.(*appsv1.StatefulSet)
		ss2 := requested.(*appsv1.StatefulSet)
		isEqual := equality.Semantic.DeepEqual(ss1.Spec, ss2.Spec)

		if !isEqual {
			reqLogger.V(3).Info("Unequal", "depoyed", ss1.Spec, "requested", ss2.Spec)
		}
		return isEqual
	})

	comparator.Comparator.SetComparator(reflect.TypeOf(netv1.Ingress{}), func(deployed, requested rtclient.Object) bool {
		return equality.Semantic.DeepEqual(deployed.(*netv1.Ingress).Spec, requested.(*netv1.Ingress).Spec)
	})

	comparator.Comparator.SetComparator(reflect.TypeOf(policyv1.PodDisruptionBudget{}), func(deployed, requested rtclient.Object) bool {
		pdb1 := deployed.(*policyv1.PodDisruptionBudget)
		pdb2 := requested.(*policyv1.PodDisruptionBudget)
		isEqual := equality.Semantic.DeepEqual(pdb1.Spec, pdb2.Spec)
		if !isEqual {
			reqLogger.V(3).Info("Unequal", "depoyed", pdb1.Spec, "requested", pdb2.Spec)
		}
		return isEqual
	})

	var compositeError []error
	deltas := comparator.Compare(reconciler.deployed, requested)
	for _, resourceType := range getOrderedTypeList() {
		delta, ok := deltas[resourceType]
		if !ok {
			// not all types will have deltas
			continue
		}
		reqLogger.Info("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			trackError(&compositeError, reconciler.createResource(customResource, client, scheme, resourceToAdd, resourceType, reqLogger))
		}
		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			trackError(&compositeError, reconciler.updateResource(customResource, client, scheme, resourceToUpdate, resourceType, reqLogger))
		}
		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			trackError(&compositeError, reconciler.deleteResource(customResource, client, scheme, resourceToRemove, resourceType, reqLogger))
		}
	}

	//empty the collected objects
	reconciler.requestedResources = nil

	if len(compositeError) == 0 {
		return nil
	} else {
		// maybe errors.Join in go1.20
		// using %q(uote) to keep errors separate
		return fmt.Errorf("%q", compositeError)
	}
}

func trackError(compositeError *[]error, err error) {
	if err != nil {
		if *compositeError == nil {
			*compositeError = make([]error, 0, 1)
		}
		*compositeError = append(*compositeError, err)
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, reqLogger logr.Logger) error {
	reqLogger.V(1).Info("Adding delta resources, i.e. creating ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.createRequestedResource(customResource, client, scheme, requested, reqLogger, kind)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, reqLogger logr.Logger) error {
	reqLogger.V(1).Info("Updating delta resources, i.e. updating ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.updateRequestedResource(customResource, client, scheme, requested, reqLogger, kind)

}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type, reqLogger logr.Logger) error {
	reqLogger.V(1).Info("Deleting delta resources, i.e. removing ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.deleteRequestedResource(customResource, client, scheme, requested, reqLogger, kind)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, reqLogger logr.Logger, kind reflect.Type) error {
	reqLogger.Info("Creating ", "kind ", kind, "named ", requested.GetName())

	return resources.Create(customResource, client, scheme, requested)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, reqLogger logr.Logger, kind reflect.Type) error {

	var updateError error
	if updateError = resources.Update(client, requested); updateError == nil {
		reqLogger.Info("updated", "kind ", kind, "named ", requested.GetName())
	} else {
		reqLogger.Error(updateError, "updated Failed", "kind ", kind, "named ", requested.GetName())
	}
	return updateError
}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, reqLogger logr.Logger, kind reflect.Type) error {

	var deleteError error
	if deleteError := resources.Delete(client, requested); deleteError == nil {
		reqLogger.V(2).Info("deleted", "kind", kind, " named ", requested.GetName())
	} else {
		reqLogger.Error(deleteError, "delete Failed", "kind", kind, " named ", requested.GetName())
	}
	return deleteError
}

// older version of the operator would drop the owner reference, we need to adopt such secrets and update them
func (reconciler *ActiveMQArtemisReconcilerImpl) adoptExistingSecretWithNoOwnerRefForUpdate(cr *brokerv1beta1.ActiveMQArtemis, candidate *corev1.Secret, client rtclient.Client) {
	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	key := types.NamespacedName{Name: candidate.Name, Namespace: candidate.Namespace}
	existingSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), key, existingSecret)
	if err == nil {
		if len(existingSecret.OwnerReferences) == 0 {
			reconciler.updateOwnerReferencesAndMatchVersion(cr, existingSecret, candidate)
			reconciler.addToDeployed(reflect.TypeOf(corev1.Secret{}), existingSecret)
			log.Info("found matching secret without owner reference, reclaiming", "Name", key)
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateOwnerReferencesAndMatchVersion(cr *brokerv1beta1.ActiveMQArtemis, existing rtclient.Object, candidate rtclient.Object) {

	resources.SetOwnerAndController(cr, existing)
	candidate.SetOwnerReferences(existing.GetOwnerReferences())
	// we need to have 'candidate' match for update
	candidate.SetResourceVersion(existing.GetResourceVersion())
	candidate.SetUID(existing.GetUID())
}

func (reconciler *ActiveMQArtemisReconcilerImpl) checkExistingService(cr *brokerv1beta1.ActiveMQArtemis, candidate *corev1.Service, client rtclient.Client) {
	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	serviceType := reflect.TypeOf(corev1.Service{})
	obj := reconciler.getFromDeployed(serviceType, candidate.Name)
	if obj != nil {
		// happy path we already own this
		return
	}
	if obj == nil {
		// check for existing match and track
		key := types.NamespacedName{Name: candidate.Name, Namespace: candidate.Namespace}
		existingService := &corev1.Service{}
		err := client.Get(context.TODO(), key, existingService)
		if err == nil {
			if len(existingService.OwnerReferences) == 0 {
				reconciler.updateOwnerReferencesAndMatchVersion(cr, existingService, candidate)
				reconciler.addToDeployed(serviceType, existingService)
				log.Info("found matching service without owner reference, reclaiming", "Name", key)
			} else {
				log.Info("found matching service with unexpected owner reference, it may need manual removal", "Name", key)
			}
		}
	}
}

func checkExistingPersistentVolumes(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) {
	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	var i int32
	for i = 0; i < getDeploymentSize(instance); i++ {
		ordinalString := strconv.Itoa(int(i))
		pvcKey := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name + "-" + namer.CrToSS(instance.Name) + "-" + ordinalString}
		pvc := &corev1.PersistentVolumeClaim{}
		err := client.Get(context.TODO(), pvcKey, pvc)

		if err == nil {
			if len(pvc.OwnerReferences) > 0 {
				found := false
				newOwnerReferences := make([]metav1.OwnerReference, 0)
				for _, oref := range pvc.OwnerReferences {
					if oref.UID == instance.UID {
						found = true
					} else {
						newOwnerReferences = append(newOwnerReferences, oref)
					}
				}
				if found {
					log.Info("removing owner ref from pvc to avoid potential data loss")
					pvc.OwnerReferences = newOwnerReferences
					if er := client.Update(context.TODO(), pvc); er != nil {
						log.Error(er, "failed to remove ownerReference from pvc", "pvc", *pvc)
					}
				}
			}
		} else {
			if !k8serrors.IsNotFound(err) {
				log.Error(err, "got error in getting pvc")
			}
		}
	}
}

var orderedTypes *([]reflect.Type)

func getOrderedTypeList() []reflect.Type {
	return genOrderedTypesLists()
}

func genOrderedTypesLists() []reflect.Type {

	if orderedTypes == nil {
		isOpenshift, _ := environments.DetectOpenshift()
		types := make([]reflect.Type, 6)

		// we want to create/update in this order
		types[0] = reflect.TypeOf(corev1.Secret{})
		types[1] = reflect.TypeOf(corev1.ConfigMap{})
		types[2] = reflect.TypeOf(appsv1.StatefulSet{})
		types[3] = reflect.TypeOf(corev1.Service{})

		if isOpenshift {
			types[4] = reflect.TypeOf(routev1.Route{})
		} else {
			types[4] = reflect.TypeOf(netv1.Ingress{})
		}
		types[5] = reflect.TypeOf(policyv1.PodDisruptionBudget{})
		orderedTypes = &types
	}
	return *orderedTypes
}

func getDeployedResources(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (map[reflect.Type][]rtclient.Object, error) {

	var log = ctrl.Log.WithName("controller_v1beta1activemqartemis")

	reader := read.New(client).WithNamespace(instance.Namespace).WithOwnerObject(instance)
	var resourceMap map[reflect.Type][]rtclient.Object
	var err error
	if isOpenshift, _ := environments.DetectOpenshift(); isOpenshift {
		resourceMap, err = reader.ListAll(
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&routev1.RouteList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
			&policyv1.PodDisruptionBudgetList{},
		)
	} else {
		resourceMap, err = reader.ListAll(
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&netv1.IngressList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
			&policyv1.PodDisruptionBudgetList{},
		)
	}
	if err != nil {
		log.Error(err, "Failed to list deployed objects.")
		return nil, err
	}

	return resourceMap, nil
}

func addNewVolumes(existingNames map[string]string, existing *[]corev1.Volume, newVolumeName *string) {
	if _, ok := existingNames[*newVolumeName]; !ok {
		volume := volumes.MakeVolume(*newVolumeName)
		*existing = append(*existing, volume)
		existingNames[*newVolumeName] = *newVolumeName
	}
}

func MakeVolumes(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers) []corev1.Volume {

	volumeDefinitions := []corev1.Volume{}
	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		basicCRVolume := volumes.MakePersistentVolume(customResource.Name)
		volumeDefinitions = append(volumeDefinitions, basicCRVolume...)
	}

	secretVolumes := make(map[string]string)
	// Scan acceptors for any with sslEnabled
	for _, acceptor := range customResource.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		secretName := customResource.Name + "-" + acceptor.Name + "-secret"
		if acceptor.SSLSecret != "" {
			secretName = acceptor.SSLSecret
		}
		addNewVolumes(secretVolumes, &volumeDefinitions, &secretName)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range customResource.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		secretName := customResource.Name + "-" + connector.Name + "-secret"
		if connector.SSLSecret != "" {
			secretName = connector.SSLSecret
		}
		addNewVolumes(secretVolumes, &volumeDefinitions, &secretName)
	}

	if customResource.Spec.Console.SSLEnabled {
		clog.V(1).Info("Make volumes for ssl console exposure on k8s")
		secretName := namer.SecretsConsoleNameBuilder.Name()
		if customResource.Spec.Console.SSLSecret != "" {
			secretName = customResource.Spec.Console.SSLSecret
		}
		addNewVolumes(secretVolumes, &volumeDefinitions, &secretName)
	}

	return volumeDefinitions
}

func addNewVolumeMounts(existingNames map[string]string, existing *[]corev1.VolumeMount, newVolumeMountName *string) {
	if _, ok := existingNames[*newVolumeMountName]; !ok {
		volumeMount := volumes.MakeVolumeMount(*newVolumeMountName)
		*existing = append(*existing, volumeMount)
		existingNames[*newVolumeMountName] = *newVolumeMountName
	}
}

func MakeVolumeMounts(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		persistentCRVlMnt := volumes.MakePersistentVolumeMount(customResource.Name, namer.GLOBAL_DATA_PATH)
		volumeMounts = append(volumeMounts, persistentCRVlMnt...)
	}

	// Scan acceptors for any with sslEnabled
	secretVolumeMounts := make(map[string]string)
	for _, acceptor := range customResource.Spec.Acceptors {
		if !acceptor.SSLEnabled {
			continue
		}
		volumeMountName := customResource.Name + "-" + acceptor.Name + "-secret-volume"
		if acceptor.SSLSecret != "" {
			volumeMountName = acceptor.SSLSecret + "-volume"
		}
		addNewVolumeMounts(secretVolumeMounts, &volumeMounts, &volumeMountName)
	}

	// Scan connectors for any with sslEnabled
	for _, connector := range customResource.Spec.Connectors {
		if !connector.SSLEnabled {
			continue
		}
		volumeMountName := customResource.Name + "-" + connector.Name + "-secret-volume"
		if connector.SSLSecret != "" {
			volumeMountName = connector.SSLSecret + "-volume"
		}
		addNewVolumeMounts(secretVolumeMounts, &volumeMounts, &volumeMountName)
	}

	if customResource.Spec.Console.SSLEnabled {
		clog.V(1).Info("Make volume mounts for ssl console exposure on k8s")
		volumeMountName := namer.SecretsConsoleNameBuilder.Name() + "-volume"
		if customResource.Spec.Console.SSLSecret != "" {
			volumeMountName = customResource.Spec.Console.SSLSecret + "-volume"
		}
		addNewVolumeMounts(secretVolumeMounts, &volumeMounts, &volumeMountName)
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
	consoleContainerPort := corev1.ContainerPort{
		Name:          "wconsj",
		ContainerPort: 8161,
		Protocol:      "TCP",
	}
	containerPorts = append(containerPorts, consoleContainerPort)

	return containerPorts
}

func (reconciler *ActiveMQArtemisReconcilerImpl) NewPodTemplateSpecForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, current *corev1.PodTemplateSpec, client rtclient.Client) (*corev1.PodTemplateSpec, error) {

	reqLogger := ctrl.Log.WithName(customResource.Name)

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	terminationGracePeriodSeconds := int64(60)

	// custom labels provided in CR applied only to the pod template spec
	// note: work with a clone of the default labels to not modify defaults
	labels := make(map[string]string)
	for key, value := range namer.LabelBuilder.Labels() {
		labels[key] = value
	}
	if customResource.Spec.DeploymentPlan.Labels != nil {
		for key, value := range customResource.Spec.DeploymentPlan.Labels {
			reqLogger.V(1).Info("Adding CR Label", "key", key, "value", value)
			if key == selectors.LabelAppKey || key == selectors.LabelResourceKey {

				meta.SetStatusCondition(&customResource.Status.Conditions, metav1.Condition{
					Type:    brokerv1beta1.ValidConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  brokerv1beta1.ValidConditionFailedReservedLabelReason,
					Message: fmt.Sprintf("'%s' is a reserved label, it is not allowed in Spec.DeploymentPlan.Labels", key),
				})
				return nil, fmt.Errorf("label key '%s' not allowed because it is reserved", key)
			} else {
				labels[key] = value
			}
		}
	}
	// validation success
	prevCondition := meta.FindStatusCondition(customResource.Status.Conditions, brokerv1beta1.ValidConditionType)
	if prevCondition == nil {
		meta.SetStatusCondition(&customResource.Status.Conditions, metav1.Condition{
			Type:   brokerv1beta1.ValidConditionType,
			Status: metav1.ConditionTrue,
			Reason: brokerv1beta1.ValidConditionSuccessReason,
		})
	}

	pts := pods.MakePodTemplateSpec(current, namespacedName, labels, customResource.Spec.DeploymentPlan.Annotations)
	podSpec := &pts.Spec

	// REVISIT: don't know when this is nil
	if podSpec == nil {
		podSpec = &corev1.PodSpec{}
	}

	podSpec.ImagePullSecrets = customResource.Spec.DeploymentPlan.ImagePullSecrets

	container := containers.MakeContainer(podSpec, customResource.Name, resolveImage(customResource, BrokerImageKey), MakeEnvVarArrayForCR(customResource, namer))

	container.Resources = customResource.Spec.DeploymentPlan.Resources

	containerPorts := MakeContainerPorts(customResource)
	if len(containerPorts) > 0 {
		reqLogger.V(1).Info("Adding new ports to main", "len", len(containerPorts))
		container.Ports = containerPorts
	}

	reqLogger.Info("Checking out extraMounts", "extra config", customResource.Spec.DeploymentPlan.ExtraMounts)

	configMapsToCreate := customResource.Spec.DeploymentPlan.ExtraMounts.ConfigMaps
	secretsToCreate := customResource.Spec.DeploymentPlan.ExtraMounts.Secrets
	brokerPropertiesResourceName, isSecret, brokerPropertiesMapData := reconciler.addResourceForBrokerProperties(customResource, namer)
	if isSecret {
		secretsToCreate = append(secretsToCreate, brokerPropertiesResourceName)
	} else {
		configMapsToCreate = append(configMapsToCreate, brokerPropertiesResourceName)
	}
	extraVolumes, extraVolumeMounts := createExtraConfigmapsAndSecretsVolumeMounts(container, configMapsToCreate, secretsToCreate, brokerPropertiesResourceName, brokerPropertiesMapData)

	reqLogger.Info("Extra volumes", "volumes", extraVolumes)
	reqLogger.Info("Extra mounts", "mounts", extraVolumeMounts)
	container.VolumeMounts = MakeVolumeMounts(customResource, namer)
	if len(extraVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, extraVolumeMounts...)
	}

	container.StartupProbe = configureStartupProbe(container, customResource.Spec.DeploymentPlan.StartupProbe)
	container.LivenessProbe = configureLivenessProbe(container, customResource.Spec.DeploymentPlan.LivenessProbe)
	container.ReadinessProbe = configureReadinessProbe(container, customResource.Spec.DeploymentPlan.ReadinessProbe)

	if len(customResource.Spec.DeploymentPlan.NodeSelector) > 0 {
		reqLogger.V(1).Info("Adding Node Selectors", "len", len(customResource.Spec.DeploymentPlan.NodeSelector))
		podSpec.NodeSelector = customResource.Spec.DeploymentPlan.NodeSelector
	}

	configureAffinity(podSpec, &customResource.Spec.DeploymentPlan.Affinity)

	if len(customResource.Spec.DeploymentPlan.Tolerations) > 0 {
		reqLogger.V(1).Info("Adding Tolerations", "len", len(customResource.Spec.DeploymentPlan.Tolerations))
		podSpec.Tolerations = customResource.Spec.DeploymentPlan.Tolerations
	}

	newContainersArray := []corev1.Container{}
	podSpec.Containers = append(newContainersArray, *container)
	brokerVolumes := MakeVolumes(customResource, namer)
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

	// JAAS Config
	if jaasConfigPath, found := getJaasConfigExtraMountPath(customResource); found {
		debugArgs := corev1.EnvVar{
			Name:  "DEBUG_ARGS",
			Value: fmt.Sprintf("-Djava.security.auth.login.config=%v", jaasConfigPath),
		}
		environments.CreateOrAppend(podSpec.Containers, &debugArgs)
	}

	if loggingConfigPath, found := getLoggingConfigExtraMountPath(customResource); found {
		loggerOpts := corev1.EnvVar{
			Name:  "JAVA_ARGS_APPEND",
			Value: fmt.Sprintf("-Dlog4j2.configurationFile=%v", loggingConfigPath),
		}
		environments.CreateOrAppend(podSpec.Containers, &loggerOpts)
	}

	//add empty-dir volume and volumeMounts to main container
	volumeForCfg := volumes.MakeVolumeForCfg(cfgVolumeName)
	podSpec.Volumes = append(podSpec.Volumes, volumeForCfg)

	// add TopologySpreadConstraints config
	podSpec.TopologySpreadConstraints = customResource.Spec.DeploymentPlan.TopologySpreadConstraints

	volumeMountForCfg := volumes.MakeRwVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMountForCfg)

	clog.Info("Creating init container for broker configuration")
	initContainer := containers.MakeInitContainer(podSpec, customResource.Name, resolveImage(customResource, InitImageKey), MakeEnvVarArrayForCR(customResource, namer))
	initContainer.Resources = customResource.Spec.DeploymentPlan.Resources

	var initCmds []string
	var initCfgRootDir = "/init_cfg_root"

	compactVersionToUse, verr := determineCompactVersionToUse(customResource)
	if verr != nil {
		reqLogger.Error(verr, "failed to get compact version", "Spec.Version", customResource.Spec.Version)
		return nil, verr
	}
	yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.FullVersionFromCompactVersion[compactVersionToUse]]
	yacfgProfileName := version.YacfgProfileName

	//address settings
	addressSettings := customResource.Spec.AddressSettings.AddressSetting
	if len(addressSettings) > 0 {
		reqLogger.Info("processing address-settings")

		var configYaml strings.Builder
		var configSpecials map[string]string = make(map[string]string)

		brokerYaml, specials := cr2jinja2.MakeBrokerCfgOverrides(customResource, nil, nil)

		configYaml.WriteString(brokerYaml)

		for k, v := range specials {
			configSpecials[k] = v
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

		clog.V(1).Info("initCmd: " + initCmd)
		initCmds = append(initCmds, initCmd)

		//populate args of init container

		podSpec.InitContainers = []corev1.Container{
			*initContainer,
		}

		//expose env for address-settings
		envVarApplyRule := "APPLY_RULE"
		envVarApplyRuleValue := customResource.Spec.AddressSettings.ApplyRule

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
		podSpec.InitContainers = []corev1.Container{
			*initContainer,
		}
	}
	//now make volumes mount available to init image

	//setup volumeMounts from scratch
	podSpec.InitContainers[0].VolumeMounts = []corev1.VolumeMount{}
	volumeMountForCfgRoot := volumes.MakeRwVolumeMountForCfg(cfgVolumeName, brokerConfigRoot)
	podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, volumeMountForCfgRoot)

	volumeMountForCfg = volumes.MakeRwVolumeMountForCfg("tool-dir", initCfgRootDir)
	podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, volumeMountForCfg)

	//add empty-dir volume
	volumeForCfg = volumes.MakeVolumeForCfg("tool-dir")
	podSpec.Volumes = append(podSpec.Volumes, volumeForCfg)

	clog.V(1).Info("Total volumes ", "volumes", podSpec.Volumes)

	// this depends on init container passing --java-opts to artemis create via launch.sh *and* it
	// not getting munged on the way. We CreateOrAppend to any value from spec.Env
	var mountPoint = secretPathBase
	if !isSecret {
		mountPoint = cfgMapPathBase
	}
	javaOpts := corev1.EnvVar{
		Name:  "JAVA_OPTS",
		Value: brokerPropertiesConfigSystemPropValue(mountPoint, brokerPropertiesResourceName, brokerPropertiesMapData),
	}
	environments.CreateOrAppend(podSpec.InitContainers, &javaOpts)

	var initArgs []string = []string{"-c"}

	//provide a way to configuration after launch.sh
	var brokerHandlerCmds []string = []string{}
	brokerConfigHandler := GetBrokerConfigHandler(namespacedName)
	if brokerConfigHandler != nil {
		clog.Info("there is a config handler")
		handlerCmds := brokerConfigHandler.Config(podSpec.InitContainers, initCfgRootDir+"/security", yacfgProfileVersion, yacfgProfileName)
		clog.Info("Getting back some init commands", "handlerCmds", handlerCmds)
		if len(handlerCmds) > 0 {
			clog.Info("appending to initCmd array...")
			brokerHandlerCmds = append(brokerHandlerCmds, handlerCmds...)

			securitySecretVolumeName := "secret-security-" + brokerConfigHandler.GetCRName()
			securitySecretVolume := volumes.MakeVolume(securitySecretVolumeName)
			podSpec.Volumes = append(podSpec.Volumes, securitySecretVolume)

			securitySecretVoluneMountName := securitySecretVolumeName + "-volume"
			securitySecretVoluneMount := volumes.MakeVolumeMount(securitySecretVoluneMountName)
			podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, securitySecretVoluneMount)
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

	clog.V(2).Info("The final init cmds to init ", "the cmd array", initArgs)

	podSpec.InitContainers[0].Args = initArgs

	if len(extraVolumeMounts) > 0 {
		podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, extraVolumeMounts...)
		clog.V(1).Info("Added some extra mounts to init", "total mounts: ", podSpec.InitContainers[0].VolumeMounts)
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
	configPodSecurity(podSpec, &customResource.Spec.DeploymentPlan.PodSecurity)
	configurePodSecurityContext(podSpec, customResource.Spec.DeploymentPlan.PodSecurityContext)

	clog.V(3).Info("Final Init spec", "Detail", podSpec.InitContainers)

	pts.Spec = *podSpec

	return pts, nil
}

func resolveImage(customResource *brokerv1beta1.ActiveMQArtemis, key string) string {
	var imageName string

	if key == InitImageKey && isLockedDown(customResource.Spec.DeploymentPlan.InitImage) {
		imageName = customResource.Spec.DeploymentPlan.InitImage
	} else if key == BrokerImageKey && isLockedDown(customResource.Spec.DeploymentPlan.Image) {
		imageName = customResource.Spec.DeploymentPlan.Image
	} else {
		imageName = determineImageToUse(customResource, key)
	}
	return imageName
}

func isLockedDown(imageAttribute string) bool {
	return imageAttribute != "placeholder" && imageAttribute != ""
}

func brokerPropertiesConfigSystemPropValue(mountPoint, resourceName string, brokerPropertiesData map[string]string) string {
	if len(brokerPropertiesData) == 1 {
		// single entry, no ordinal subpath - broker will log if arg is not found for the watcher so make conditional
		return fmt.Sprintf("-Dbroker.properties=%s%s/%s", mountPoint, resourceName, BrokerPropertiesName)
	} else {
		// directory works on broker image versions >= 2.27.1
		return fmt.Sprintf("-Dbroker.properties=%s%s/,%s%s/%s${STATEFUL_SET_ORDINAL}/", mountPoint, resourceName, mountPoint, resourceName, OrdinalPrefix)
	}
}

func getJaasConfigExtraMountPath(customResource *brokerv1beta1.ActiveMQArtemis) (string, bool) {
	if t, name, found := getConfigExtraMount(customResource, jaasConfigSuffix); found {
		return fmt.Sprintf("/amq/extra/%v/%v/login.config", t, name), true
	}
	return "", false
}

func getLoggingConfigExtraMountPath(customResource *brokerv1beta1.ActiveMQArtemis) (string, bool) {
	if t, name, found := getConfigExtraMount(customResource, loggingConfigSuffix); found {
		return fmt.Sprintf("/amq/extra/%v/%v/logging.properties", t, name), true
	}
	return "", false
}

func getConfigExtraMount(customResource *brokerv1beta1.ActiveMQArtemis, suffix string) (string, string, bool) {
	for _, cm := range customResource.Spec.DeploymentPlan.ExtraMounts.ConfigMaps {
		if strings.HasSuffix(cm, suffix) {
			return "configmaps", cm, true
		}
	}
	for _, s := range customResource.Spec.DeploymentPlan.ExtraMounts.Secrets {
		if strings.HasSuffix(s, suffix) {
			return "secrets", s, true
		}
	}
	return "", "", false
}

func configureStartupProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {

	var startupProbe *corev1.Probe = container.StartupProbe
	clog.V(1).Info("Configuring Startup Probe", "existing", startupProbe)

	if probeFromCr != nil {
		if startupProbe == nil {
			startupProbe = &corev1.Probe{}
		}

		applyNonDefaultedValues(startupProbe, probeFromCr)
		startupProbe.ProbeHandler = probeFromCr.ProbeHandler
	} else {
		startupProbe = nil
	}

	return startupProbe
}

func configureLivenessProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {
	var livenessProbe *corev1.Probe = container.LivenessProbe
	clog.V(1).Info("Configuring Liveness Probe", "existing", livenessProbe)

	if livenessProbe == nil {
		livenessProbe = &corev1.Probe{}
	}

	if probeFromCr != nil {
		applyNonDefaultedValues(livenessProbe, probeFromCr)

		// not complete in this case!
		if probeFromCr.Exec == nil && probeFromCr.HTTPGet == nil && probeFromCr.TCPSocket == nil {
			clog.V(1).Info("Adding default TCP check")
			livenessProbe.ProbeHandler = corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(TCPLivenessPort),
				},
			}
		} else if probeFromCr.TCPSocket != nil {
			clog.V(1).Info("Using user specified TCPSocket")
			livenessProbe.ProbeHandler = corev1.ProbeHandler{
				TCPSocket: probeFromCr.TCPSocket,
			}
		} else {
			clog.V(1).Info("Using user provided Liveness Probe Exec " + probeFromCr.Exec.String())
			livenessProbe.Exec = probeFromCr.Exec
		}
	} else {
		clog.V(1).Info("Creating Default Liveness Probe")

		livenessProbe.InitialDelaySeconds = defaultLivenessProbeInitialDelay
		livenessProbe.TimeoutSeconds = 5
		livenessProbe.ProbeHandler = corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(TCPLivenessPort),
			},
		}
	}

	return livenessProbe
}

var command = []string{
	"/bin/bash",
	"-c",
	"/opt/amq/bin/readinessProbe.sh",
}

var betterCommand = []string{
	"/bin/bash",
	"-c",
	"/opt/amq/bin/readinessProbe.sh 1", // retries/count - so we get fast feedback and can configure via the Probe
	// "1", sleep seconds not applicable with 1 retry
}

func configureReadinessProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {

	var readinessProbe *corev1.Probe = container.ReadinessProbe
	clog.V(1).Info("Configuring Readyness Probe", "existing", readinessProbe)

	if readinessProbe == nil {
		readinessProbe = &corev1.Probe{}
	}

	if probeFromCr != nil {
		applyNonDefaultedValues(readinessProbe, probeFromCr)
		if probeFromCr.Exec == nil && probeFromCr.HTTPGet == nil && probeFromCr.TCPSocket == nil {
			clog.V(1).Info("adding default handler to user provided readiness Probe")

			// respect existing command where already deployed
			if readinessProbe.ProbeHandler.Exec != nil && reflect.DeepEqual(readinessProbe.ProbeHandler.Exec.Command, command) {
				// leave it be so we don't force a reconcile
			} else {
				// upgrade to betterCommand!
				readinessProbe.ProbeHandler = corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: betterCommand,
					},
				}
			}
		} else {
			readinessProbe.ProbeHandler = probeFromCr.ProbeHandler
		}
	} else {
		clog.V(1).Info("vreating default readiness Probe")
		readinessProbe.InitialDelaySeconds = defaultLivenessProbeInitialDelay
		readinessProbe.TimeoutSeconds = 5

		// respect existing command where already deployed
		if readinessProbe.ProbeHandler.Exec != nil && reflect.DeepEqual(readinessProbe.ProbeHandler.Exec.Command, command) {
			// leave it be so we don't force a reconcile
		} else {
			// upgrade to betterCommand!
			readinessProbe.ProbeHandler = corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: betterCommand,
				},
			}
		}
	}

	return readinessProbe
}

func applyNonDefaultedValues(readinessProbe *corev1.Probe, probeFromCr *corev1.Probe) {
	//copy the probe, but only for non default (init values)
	if probeFromCr.InitialDelaySeconds > 0 {
		readinessProbe.InitialDelaySeconds = probeFromCr.InitialDelaySeconds
	}
	if probeFromCr.TimeoutSeconds > 0 {
		readinessProbe.TimeoutSeconds = probeFromCr.TimeoutSeconds
	}
	if probeFromCr.PeriodSeconds > 0 {
		readinessProbe.PeriodSeconds = probeFromCr.PeriodSeconds
	}
	if probeFromCr.TerminationGracePeriodSeconds != nil {
		readinessProbe.TerminationGracePeriodSeconds = probeFromCr.TerminationGracePeriodSeconds
	}
	if probeFromCr.SuccessThreshold > 0 {
		readinessProbe.SuccessThreshold = probeFromCr.SuccessThreshold
	}
	if probeFromCr.FailureThreshold > 0 {
		readinessProbe.FailureThreshold = probeFromCr.FailureThreshold
	}
}

func getConfigAppliedConfigMapName(artemis *brokerv1beta1.ActiveMQArtemis) types.NamespacedName {
	return types.NamespacedName{
		Namespace: artemis.Namespace,
		Name:      artemis.Name + "-props",
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) addResourceForBrokerProperties(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers) (string, bool, map[string]string) {

	// fetch and do idempotent transform based on CR

	// deal with upgrade to immutable secret, only upgrade to mutable on not found
	alder32Bytes := alder32Of(customResource.Spec.BrokerProperties)
	shaOfMap := hex.EncodeToString(alder32Bytes)
	resourceName := types.NamespacedName{
		Namespace: customResource.Namespace,
		Name:      customResource.Name + "-props-" + shaOfMap,
	}

	obj := reconciler.cloneOfDeployed(reflect.TypeOf(corev1.ConfigMap{}), resourceName.Name)
	if obj != nil {
		existing := obj.(*corev1.ConfigMap)
		// found existing (immuable) map with sha in the name
		clog.V(1).Info("Requesting configMap for broker properties", "name", resourceName.Name)
		reconciler.trackDesired(existing)

		return resourceName.Name, false, existing.Data
	}

	var desired *corev1.Secret
	resourceName = getConfigAppliedConfigMapName(customResource)

	obj = reconciler.cloneOfDeployed(reflect.TypeOf(corev1.Secret{}), resourceName.Name)
	if obj != nil {
		desired = obj.(*corev1.Secret)
	}

	data := brokerPropertiesData(customResource.Spec.BrokerProperties)
	if desired == nil {
		secret := secrets.MakeSecret(resourceName, resourceName.Name, data, namer.LabelBuilder.Labels())
		desired = &secret
	} else {
		desired.StringData = data
	}

	clog.V(1).Info("Requesting secret for broker properties", "name", resourceName.Name)
	reconciler.trackDesired(desired)

	clog.V(1).Info("Requesting mount for broker properties secret")
	return resourceName.Name, true, data
}

func alder32StringValue(alder32Bytes []byte) string {
	return fmt.Sprintf("%d", binary.BigEndian.Uint32(alder32Bytes))
}

func alder32Of(props []string) []byte {

	digest := adler32.New()
	for _, k := range props {
		digest.Write([]byte(k))
	}

	return digest.Sum(nil)
}

func brokerPropertiesData(props []string) map[string]string {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "# generated by crd")
	fmt.Fprintln(buf, "#")

	hasOrdinalPrefix := false
	for _, propertyKeyVal := range props {
		if strings.HasPrefix(propertyKeyVal, OrdinalPrefix) && strings.Index(propertyKeyVal, OrdinalPrefixSep) > 0 {
			hasOrdinalPrefix = true
			continue
		}
		fmt.Fprintf(buf, "%s\n", propertyKeyVal)
	}

	contents := map[string]string{BrokerPropertiesName: buf.String()}

	if hasOrdinalPrefix {
		for _, propertyKeyVal := range props {
			if strings.HasPrefix(propertyKeyVal, OrdinalPrefix) {
				if i := strings.Index(propertyKeyVal, OrdinalPrefixSep); i > 0 {
					// use a key that will match the volume projection and broker status
					mapKey := propertyKeyVal[:i+len(OrdinalPrefixSep)] + BrokerPropertiesName
					value := propertyKeyVal[i+len(OrdinalPrefixSep):]

					existing, found := contents[mapKey]
					if found {
						contents[mapKey] = fmt.Sprintf("%s%s\n", existing, value)
					} else {
						contents[mapKey] = fmt.Sprintf("%s\n", value)
					}
				}
			}
		}
	}

	return contents
}

func configureAffinity(podSpec *corev1.PodSpec, affinity *brokerv1beta1.AffinityConfig) {
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

func determineImageToUse(customResource *brokerv1beta1.ActiveMQArtemis, imageTypeKey string) string {

	found := false
	imageName := ""
	compactVersionToUse, _ := determineCompactVersionToUse(customResource)

	genericRelatedImageEnvVarName := ImageNamePrefix + imageTypeKey + "_" + compactVersionToUse
	// Default case of x86_64/amd64 covered here
	archSpecificRelatedImageEnvVarName := genericRelatedImageEnvVarName
	if osruntime.GOARCH == "arm64" || osruntime.GOARCH == "s390x" || osruntime.GOARCH == "ppc64le" {
		archSpecificRelatedImageEnvVarName = genericRelatedImageEnvVarName + "_" + osruntime.GOARCH
	}
	imageName, found = os.LookupEnv(archSpecificRelatedImageEnvVarName)
	clog.V(1).Info("DetermineImageToUse", "env", archSpecificRelatedImageEnvVarName, "imageName", imageName)

	// Use genericRelatedImageEnvVarName if archSpecificRelatedImageEnvVarName is not found
	if !found {
		imageName, found = os.LookupEnv(genericRelatedImageEnvVarName)
		clog.V(1).Info("DetermineImageToUse - from generic", "env", genericRelatedImageEnvVarName, "imageName", imageName)
	}

	// Use latest images if archSpecificRelatedImageEnvVarName and genericRelatedImageEnvVarName are not found
	if !found {
		imageName = version.DefaultImageName(archSpecificRelatedImageEnvVarName)
		clog.V(1).Info("DetermineImageToUse - from default", "env", archSpecificRelatedImageEnvVarName, "imageName", imageName)
	}

	return imageName
}

func resolveBrokerVersion(cr *brokerv1beta1.ActiveMQArtemis) (string, error) {

	if cr.Spec.Version != "" {
		_, verr := semver.ParseTolerant(cr.Spec.Version)
		if verr != nil {
			return "", verr
		}
	}

	result := common.ResolveBrokerVersion(version.SupportedActiveMQArtemisSemanticVersions(), cr.Spec.Version)
	if result == nil {
		return "", errors.Errorf("did not find a matching broker in the supported list for %v", cr.Spec.Version)
	}
	return result.String(), nil
}

func determineCompactVersionToUse(customResource *brokerv1beta1.ActiveMQArtemis) (string, error) {

	resolvedFullVersion, err := resolveBrokerVersion(customResource)
	if err != nil {
		clog.V(1).Error(err, "failed to determine broker version from cr")
		return "", err
	}
	compactVersionToUse := version.CompactActiveMQArtemisVersion(resolvedFullVersion)

	return compactVersionToUse, nil
}

func createExtraConfigmapsAndSecretsVolumeMounts(brokerContainer *corev1.Container, configMaps []string, secrets []string, brokePropertiesResourceName string, brokerPropsData map[string]string) ([]corev1.Volume, []corev1.VolumeMount) {

	var extraVolumes []corev1.Volume
	var extraVolumeMounts []corev1.VolumeMount

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
			cfgmapVolumeMount := volumes.MakeVolumeMountForCfg(cfgmapVol.Name, cfgmapPath, true)
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

			if secret == brokePropertiesResourceName && len(brokerPropsData) > 1 {
				// place ordinal data in subpath in order
				for _, key := range sortedKeys(brokerPropsData) {
					if hasOrdinal, separatorIndex := extractOrdinalPrefixSeperatorIndex(key); hasOrdinal {
						subPath := key[:separatorIndex]
						secretVol.VolumeSource.Secret.Items = append(secretVol.VolumeSource.Secret.Items, corev1.KeyToPath{Key: key, Path: fmt.Sprintf("%s/%s", subPath, key)})
					} else {
						secretVol.VolumeSource.Secret.Items = append(secretVol.VolumeSource.Secret.Items, corev1.KeyToPath{Key: key, Path: key})
					}
				}
			}
			secretVolumeMount := volumes.MakeVolumeMountForCfg(secretVol.Name, secretPath, true)
			extraVolumes = append(extraVolumes, secretVol)
			extraVolumeMounts = append(extraVolumeMounts, secretVolumeMount)
		}
	}

	return extraVolumes, extraVolumeMounts
}

func extractOrdinalPrefixSeperatorIndex(key string) (bool, int) {

	prefixIndex := strings.Index(key, OrdinalPrefix)
	separatorIndex := strings.Index(key, OrdinalPrefixSep)

	if prefixIndex == 0 && separatorIndex > len(OrdinalPrefix) {
		return true, separatorIndex
	}
	return false, -1
}

func (reconciler *ActiveMQArtemisReconcilerImpl) NewStatefulSetForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, currentStateFullSet *appsv1.StatefulSet, client rtclient.Client) (*appsv1.StatefulSet, error) {

	reqLogger := ctrl.Log.WithName(customResource.Name)

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	replicas := getDeploymentSize(customResource)
	currentStateFullSet = ss.MakeStatefulSet(currentStateFullSet, namer.SsNameBuilder.Name(), namer.SvcHeadlessNameBuilder.Name(), namespacedName, customResource.Annotations, namer.LabelBuilder.Labels(), &replicas)

	podTemplateSpec, err := reconciler.NewPodTemplateSpecForCR(customResource, namer, &currentStateFullSet.Spec.Template, client)
	if err != nil {
		reqLogger.Error(err, "Error creating new pod template")
		return nil, err
	}

	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		currentStateFullSet.Spec.VolumeClaimTemplates = *NewPersistentVolumeClaimArrayForCR(customResource, namer, 1)
	}
	currentStateFullSet.Spec.Template = *podTemplateSpec

	return currentStateFullSet, nil
}

func NewPersistentVolumeClaimArrayForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers, arrayLength int) *[]corev1.PersistentVolumeClaim {

	var pvc *corev1.PersistentVolumeClaim = nil
	capacity := "2Gi"
	pvcArray := make([]corev1.PersistentVolumeClaim, 0, arrayLength)
	storageClassName := ""

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	if customResource.Spec.DeploymentPlan.Storage.Size != "" {
		capacity = customResource.Spec.DeploymentPlan.Storage.Size
	}

	if customResource.Spec.DeploymentPlan.Storage.StorageClassName != "" {
		storageClassName = customResource.Spec.DeploymentPlan.Storage.StorageClassName
	}

	for i := 0; i < arrayLength; i++ {
		pvc = persistentvolumeclaims.NewPersistentVolumeClaimWithCapacityAndStorageClassName(namespacedName, capacity, namer.LabelBuilder.Labels(), storageClassName)
		pvcArray = append(pvcArray, *pvc)
	}

	return &pvcArray
}

func ProcessStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName, namer Namers, reconcileError error) {

	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	updateVersionStatus(cr)

	updateScaleStatus(cr, namer)

	reqLogger.V(1).Info("Updating status for pods")

	podStatus := updatePodStatus(cr, client, namespacedName)

	reqLogger.V(1).Info("PodStatus current..................", "info:", podStatus)
	reqLogger.V(1).Info("Ready Count........................", "info:", len(podStatus.Ready))
	reqLogger.V(1).Info("Stopped Count......................", "info:", len(podStatus.Stopped))
	reqLogger.V(1).Info("Starting Count.....................", "info:", len(podStatus.Starting))

	ValidCondition := getValidCondition(cr)
	meta.SetStatusCondition(&cr.Status.Conditions, ValidCondition)
	meta.SetStatusCondition(&cr.Status.Conditions, getDeploymentCondition(cr, podStatus, ValidCondition.Status != metav1.ConditionFalse, reconcileError))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		reqLogger.V(1).Info("Pods status updated")
		cr.Status.PodStatus = podStatus
	} else {
		// could leave this to kube, it will do a []byte comparison
		reqLogger.V(1).Info("Pods status unchanged")
	}
}

func updateScaleStatus(cr *brokerv1beta1.ActiveMQArtemis, namer Namers) {
	labels := make([]string, 0, len(namer.LabelBuilder.Labels())+len(cr.Spec.DeploymentPlan.Labels))
	for k, v := range namer.LabelBuilder.Labels() {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range cr.Spec.DeploymentPlan.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(labels)
	cr.Status.ScaleLabelSelector = strings.Join(labels[:], ",")
}

func updateVersionStatus(cr *brokerv1beta1.ActiveMQArtemis) {
	cr.Status.Version.Image = resolveImage(cr, BrokerImageKey)
	cr.Status.Version.InitImage = resolveImage(cr, InitImageKey)

	if isLockedDown(cr.Spec.DeploymentPlan.Image) || isLockedDown(cr.Spec.DeploymentPlan.InitImage) {
		cr.Status.Version.BrokerVersion = ""
		cr.Status.Upgrade.SecurityUpdates = false
		cr.Status.Upgrade.MajorUpdates = false
		cr.Status.Upgrade.MinorUpdates = false
		cr.Status.Upgrade.PatchUpdates = false

	} else {
		cr.Status.Version.BrokerVersion, _ = resolveBrokerVersion(cr)
		cr.Status.Upgrade.SecurityUpdates = true

		if cr.Spec.Version == "" {
			cr.Status.Upgrade.MajorUpdates = true
			cr.Status.Upgrade.MinorUpdates = true
			cr.Status.Upgrade.PatchUpdates = true
		} else {

			cr.Status.Upgrade.MajorUpdates = false
			cr.Status.Upgrade.MinorUpdates = false
			cr.Status.Upgrade.PatchUpdates = false

			// flip defaults based on specificity of Version
			switch len(strings.Split(cr.Spec.Version, ".")) {
			case 1:
				cr.Status.Upgrade.MinorUpdates = true
				fallthrough
			case 2:
				cr.Status.Upgrade.PatchUpdates = true
			}
		}
	}

}

func getValidCondition(cr *brokerv1beta1.ActiveMQArtemis) metav1.Condition {
	// add valid true if none exists
	for _, c := range cr.Status.Conditions {
		if c.Type == brokerv1beta1.ValidConditionType {
			return c
		}
	}
	return metav1.Condition{
		Type:   brokerv1beta1.ValidConditionType,
		Reason: brokerv1beta1.ValidConditionSuccessReason,
		Status: metav1.ConditionTrue,
	}
}

func getDeploymentCondition(cr *brokerv1beta1.ActiveMQArtemis, podStatus olm.DeploymentStatus, valid bool, reconcileError error) metav1.Condition {

	if !valid {
		return metav1.Condition{
			Type:   brokerv1beta1.DeployedConditionType,
			Status: metav1.ConditionFalse,
			Reason: brokerv1beta1.DeployedConditionValidationFailedReason,
		}
	}

	if reconcileError != nil {
		return metav1.Condition{
			Type:    brokerv1beta1.DeployedConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.DeployedConditionCrudKindErrorReason,
			Message: reconcileError.Error(),
		}
	}

	deploymentSize := getDeploymentSize(cr)
	if deploymentSize == 0 {
		return metav1.Condition{
			Type:    brokerv1beta1.DeployedConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.DeployedConditionZeroSizeReason,
			Message: common.DeployedConditionZeroSizeMessage,
		}
	}
	if len(podStatus.Ready) != int(deploymentSize) {
		return metav1.Condition{
			Type:    brokerv1beta1.DeployedConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.DeployedConditionNotReadyReason,
			Message: fmt.Sprintf("%d/%d pods ready", len(podStatus.Ready), deploymentSize),
		}
	}
	return metav1.Condition{
		Type:   brokerv1beta1.DeployedConditionType,
		Reason: brokerv1beta1.DeployedConditionReadyReason,
		Status: metav1.ConditionTrue,
	}
}

func updatePodStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {

	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.V(1).Info("Getting status for pods")

	var status olm.DeploymentStatus
	var lastStatus olm.DeploymentStatus

	lastStatusExist := false
	if lastStatus, lastStatusExist = lastStatusMap[namespacedName]; !lastStatusExist {
		ctrl.Log.Info("Creating lastStatus for new CR", "name", namespacedName)
		lastStatus = olm.DeploymentStatus{}
		lastStatusMap[namespacedName] = lastStatus
	}

	ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(namespacedName.Name), Namespace: namespacedName.Namespace}
	sfsFound := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), ssNamespacedName, sfsFound)
	if err == nil {
		status = getSingleStatefulSetStatus(sfsFound, cr)
	}

	// TODO: Remove global usage
	reqLogger.V(1).Info("lastStatus.Ready len is " + fmt.Sprint(len(lastStatus.Ready)))
	reqLogger.V(1).Info("status.Ready len is " + fmt.Sprint(len(status.Ready)))
	if len(status.Ready) > len(lastStatus.Ready) {
		// More pods ready, let the address controller know
		for i := len(lastStatus.Ready); i < len(status.Ready); i++ {
			reqLogger.V(1).Info("Notifying address controller", "new ready", i)
			channels.AddressListeningCh <- types.NamespacedName{Namespace: namespacedName.Namespace, Name: status.Ready[i]}
		}
	}
	lastStatusMap[namespacedName] = status

	return status
}

func getSingleStatefulSetStatus(ss *appsv1.StatefulSet, cr *brokerv1beta1.ActiveMQArtemis) olm.DeploymentStatus {
	var ready, starting, stopped []string
	var requestedCount = int32(0)
	if ss.Spec.Replicas != nil {
		requestedCount = *ss.Spec.Replicas
	}
	cr.Status.DeploymentPlanSize = requestedCount

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

func MakeEnvVarArrayForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer Namers) []corev1.EnvVar {

	requireLogin := "false"
	if customResource.Spec.DeploymentPlan.RequireLogin {
		requireLogin = "true"
	} else {
		requireLogin = "false"
	}

	journalType := "aio"
	if strings.ToLower(customResource.Spec.DeploymentPlan.JournalType) == "aio" {
		journalType = "aio"
	} else {
		journalType = "nio"
	}

	jolokiaAgentEnabled := "false"
	if customResource.Spec.DeploymentPlan.JolokiaAgentEnabled {
		jolokiaAgentEnabled = "true"
	} else {
		jolokiaAgentEnabled = "false"
	}

	managementRBACEnabled := "false"
	if customResource.Spec.DeploymentPlan.ManagementRBACEnabled {
		managementRBACEnabled = "true"
	} else {
		managementRBACEnabled = "false"
	}

	metricsPluginEnabled := "false"
	if customResource.Spec.DeploymentPlan.EnableMetricsPlugin != nil {
		metricsPluginEnabled = strconv.FormatBool(*customResource.Spec.DeploymentPlan.EnableMetricsPlugin)
	}

	envVar := []corev1.EnvVar{}
	envVarArrayForBasic := environments.AddEnvVarForBasic(requireLogin, journalType, namer.SvcPingNameBuilder.Name())
	envVar = append(envVar, envVarArrayForBasic...)
	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		envVarArrayForPresistent := environments.AddEnvVarForPersistent(customResource.Name)
		envVar = append(envVar, envVarArrayForPresistent...)
	}

	envVarArrayForCluster := environments.AddEnvVarForCluster(isClustered(customResource))
	envVar = append(envVar, envVarArrayForCluster...)

	envVarArrayForJolokia := environments.AddEnvVarForJolokia(jolokiaAgentEnabled)
	envVar = append(envVar, envVarArrayForJolokia...)

	envVarArrayForManagement := environments.AddEnvVarForManagement(managementRBACEnabled)
	envVar = append(envVar, envVarArrayForManagement...)

	envVarArrayForMetricsPlugin := environments.AddEnvVarForMetricsPlugin(metricsPluginEnabled)
	envVar = append(envVar, envVarArrayForMetricsPlugin...)

	// appending any Env from CR, to allow potential override
	envVar = append(envVar, customResource.Spec.Env...)

	return envVar
}

type brokerStatus struct {
	BrokerConfigStatus brokerConfigStatus `json:"configuration"`
	ServerStatus       serverStatus       `json:"server"`
}

type serverStatus struct {
	Jaas jaasStatus `json:"jaas"`
}

type jaasStatus struct {
	PropertiesStatus map[string]propertiesStatus `json:"properties"`
}

type brokerConfigStatus struct {
	PropertiesStatus map[string]propertiesStatus `json:"properties"`
}

type propertiesStatus struct {
	Alder32     string       `json:"alder32"`
	ReloadTime  string       `json:"reloadTime"`
	ApplyErrors []applyError `json:"errors"`
}

type applyError struct {
	PropKeyValue string `json:"value"`
	Reason       string `json:"reason"`
}

func ProcessBrokerStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) (retry bool) {
	var condition metav1.Condition

	err := AssertBrokersAvailable(cr, client, scheme)
	if err != nil {
		condition = trapErrorAsCondition(err, brokerv1beta1.ConfigAppliedConditionType)
		meta.SetStatusCondition(&cr.Status.Conditions, condition)
		return err.Requeue()
	}

	err = AssertBrokerPropertiesStatus(cr, client, scheme)
	if err == nil {
		condition = metav1.Condition{
			Type:   brokerv1beta1.ConfigAppliedConditionType,
			Status: metav1.ConditionTrue,
			Reason: brokerv1beta1.ConfigAppliedConditionSynchedReason,
		}
	} else {
		condition = trapErrorAsCondition(err, brokerv1beta1.ConfigAppliedConditionType)
		retry = err.Requeue()
	}
	meta.SetStatusCondition(&cr.Status.Conditions, condition)

	if _, _, found := getConfigExtraMount(cr, jaasConfigSuffix); found {
		err = AssertJaasPropertiesStatus(cr, client, scheme)
		if err == nil {
			condition = metav1.Condition{
				Type:   brokerv1beta1.JaasConfigAppliedConditionType,
				Status: metav1.ConditionTrue,
				Reason: brokerv1beta1.ConfigAppliedConditionSynchedReason,
			}
		} else {
			condition = trapErrorAsCondition(err, brokerv1beta1.JaasConfigAppliedConditionType)
			retry = retry || err.Requeue()
		}

		meta.SetStatusCondition(&cr.Status.Conditions, condition)
	}
	return retry
}

func trapErrorAsCondition(err ArtemisError, conditionType string) metav1.Condition {
	var condition metav1.Condition
	switch err.(type) {
	case jolokiaClientNotFoundError:
		condition = metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.ConfigAppliedConditionNoJolokiaClientsAvailableReason,
			Message: err.Error(),
		}
	case statusOutOfSyncError:
		condition = metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.ConfigAppliedConditionOutOfSyncReason,
			Message: err.Error(),
		}
	case statusOutOfSyncMissingKeyError:
		condition = metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionUnknown,
			Reason:  brokerv1beta1.ConfigAppliedConditionOutOfSyncReason,
			Message: err.Error(),
		}
	case inSyncApplyError:
		condition = metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.ConfigAppliedConditionSynchedWithErrorReason,
			Message: err.Error(),
		}
	default:
		condition = metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionUnknown,
			Reason:  brokerv1beta1.ConfigAppliedConditionUnknownReason,
			Message: err.Error(),
		}
	}
	return condition
}

type projection struct {
	Name            string
	ResourceVersion string
	Generation      int64
	Files           map[string]propertyFile
	Ordinals        []string
}

type propertyFile struct {
	Alder32 string
}

func AssertBrokersAvailable(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	// pre-condition, we must be deployed, avoid broker status roundtrip till ready
	DeployedCondition := meta.FindStatusCondition(cr.Status.Conditions, brokerv1beta1.DeployedConditionType)
	if DeployedCondition == nil || DeployedCondition.Status == metav1.ConditionFalse {
		reqLogger.Info("There are no available brokers from DeployedCondition", "condition", DeployedCondition)
		return NewUnknownJolokiaError(errors.New("no available brokers from deployed condition"))
	}
	return nil
}

func AssertBrokerPropertiesStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	Projection, err := getConfigMappedBrokerProperties(cr, client)
	if err != nil {
		reqLogger.Info("error retrieving config resources. requeing")
		return NewUnknownJolokiaError(err)
	}

	return checkStatus(cr, client, Projection, func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool) {
		current, present := BrokerStatus.BrokerConfigStatus.PropertiesStatus[FileName]
		return current, present
	})
}

func AssertJaasPropertiesStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	Projection, err := getConfigMappedJaasProperties(cr, client)
	if err != nil {
		reqLogger.Info("error retrieving config resources. requeing")
		return NewUnknownJolokiaError(err)
	}

	statusError := checkStatus(cr, client, Projection, func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool) {
		current, present := BrokerStatus.ServerStatus.Jaas.PropertiesStatus[FileName]
		return current, present
	})

	if statusError == nil {
		updateExtraConfigStatus(cr, Projection)
	}

	return statusError
}

func checkStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, Projection *projection, extractStatus func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool)) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	resource := types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}

	ssInfos := ss.GetDeployedStatefulSetNames(client, cr.Namespace, []types.NamespacedName{resource})

	jks := jolokia_client.GetBrokers(resource, ssInfos, client)

	if len(jks) == 0 {
		reqLogger.Info("not found Jolokia Clients available. requeing")
		return NewJolokiaClientsNotFoundError(errors.New("Waiting for Jolokia Clients to become available"))
	}

	reqLogger.V(2).Info("in sync check", "projection", Projection)

	for _, jk := range jks {
		currentJson, err := jk.Artemis.GetStatus()

		if err != nil {
			reqLogger.Info("unknown status reported from Jolokia.", "IP", jk.IP, "Ordinal", jk.Ordinal, "error", err)
			return NewUnknownJolokiaError(err)
		}

		reqLogger.V(2).Info("raw json status", "IP", jk.IP, "ordinal", jk.Ordinal, "status json", currentJson)

		brokerStatus, err := unmarshallStatus(currentJson)
		if err != nil {
			reqLogger.Error(err, "unable to unmarshall broker status", "json", currentJson)
			return NewUnknownJolokiaError(err)
		}

		reqLogger.V(2).Info("broker status", "ordinal", jk.Ordinal, "status", brokerStatus)

		var current propertiesStatus
		var present bool
		missingKeys := []string{}

		for name, file := range Projection.Files {

			current, present = extractStatus(brokerStatus, name)

			if !present {
				// with ordinal prefix or extras in the map this can be the case
				isForOrdinal, _ := extractOrdinalPrefixSeperatorIndex(name)
				if !(name == JaasConfigKey || strings.HasPrefix(name, "_") || isForOrdinal) {
					missingKeys = append(missingKeys, name)
				}
				continue
			}

			if current.Alder32 == "" {
				err = errors.Errorf("out of sync on pod %s-%s, property file %s has an empty checksum",
					namer.CrToSS(cr.Name), jk.Ordinal, name)
				reqLogger.Info(err.Error(), "status", brokerStatus, "tracked", Projection)
				return NewStatusOutOfSyncError(err)
			}

			if file.Alder32 != current.Alder32 {
				err = errors.Errorf("out of sync on pod %s-%s, mismatched checksum on property file %s, expected: %s, current: %s. A delay can occur before a volume mount projection is refreshed.",
					namer.CrToSS(cr.Name), jk.Ordinal, name, file.Alder32, current.Alder32)
				reqLogger.Info(err.Error(), "status", brokerStatus, "tracked", Projection)
				return NewStatusOutOfSyncError(err)
			}
		}

		if len(missingKeys) > 0 {
			if strings.HasSuffix(Projection.Name, jaasConfigSuffix) {
				err = errors.Errorf("out of sync on pod %s-%s, property files are not visible on the broker: %v. Reloadable JAAS LoginModule property files are only visible after the first login attempt that references them. If the property files are for by a third party LoginModule or not reloadable, prefix the property file names with an underscore to exclude them from this condition",
					namer.CrToSS(cr.Name), jk.Ordinal, missingKeys)
			} else {
				err = errors.Errorf("out of sync on pod %s-%s, configuration property files are not visible on the broker: %v. A delay can occur before a volume mount projection is refreshed.",
					namer.CrToSS(cr.Name), jk.Ordinal, missingKeys)
			}
			reqLogger.Info(err.Error(), "status", brokerStatus, "tracked", Projection)
			return NewStatusOutOfSyncMissingKeyError(err)
		}

		// all in sync, check for apply errors
		var applyError *inSyncApplyError = nil
		for k, v := range brokerStatus.BrokerConfigStatus.PropertiesStatus {
			if len(v.ApplyErrors) > 0 {
				// some props did not apply for k
				if applyError == nil {
					applyError = NewInSyncWithError(fmt.Sprintf("%s-%s", namer.CrToSS(cr.Name), jk.Ordinal))
				}
				applyError.ErrorApplyDetail(k, marshallApplyErrors(v.ApplyErrors))
			}
		}
		if applyError != nil {
			reqLogger.Info("in sync with apply error", "error", applyError)
			return *applyError
		}

		// this oridinal is happy
		Projection.Ordinals = append(Projection.Ordinals, jk.Ordinal)
	}

	reqLogger.Info("successfully synced with broker", "status", statusMessageFromProjection(Projection))
	return nil
}

func statusMessageFromProjection(Projection *projection) string {
	var statusMessage string
	statusMessageJson, err := json.Marshal(*Projection)
	if err == nil {
		statusMessage = string(statusMessageJson)
	} else {
		statusMessage = fmt.Sprintf("%+v", *Projection)
	}
	return statusMessage
}

func updateExtraConfigStatus(cr *brokerv1beta1.ActiveMQArtemis, Projection *projection) {
	if len(cr.Status.ExternalConfigs) > 0 {
		for index, s := range cr.Status.ExternalConfigs {
			if s.Name == Projection.Name {
				cr.Status.ExternalConfigs[index].ResourceVersion = Projection.ResourceVersion
				return // update complete
			}
		}
	}

	// add an entry
	cr.Status.ExternalConfigs = append(cr.Status.ExternalConfigs,
		brokerv1beta1.ExternalConfigStatus{Name: Projection.Name, ResourceVersion: Projection.ResourceVersion})
}

func getConfigMappedBrokerProperties(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (*projection, error) {

	cmName := getConfigAppliedConfigMapName(cr)
	resource := corev1.Secret{}
	err := client.Get(context.TODO(), cmName, &resource)
	if err != nil {
		return nil, NewUnknownJolokiaError(errors.Wrap(err, "unable to retrieve mutable properties resource"))
	}
	return newProjectionFromByteValues(resource.ObjectMeta, resource.Data), nil
}

func getConfigMappedJaasProperties(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (*projection, error) {
	var instance *projection
	if _, name, found := getConfigExtraMount(cr, jaasConfigSuffix); found {
		resource := corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Namespace: cr.Namespace, Name: name}, &resource)
		if err != nil {
			return nil, NewUnknownJolokiaError(errors.Wrap(err, "unable to retrieve mutable secret to hash"))
		}
		instance = newProjectionFromByteValues(resource.ObjectMeta, resource.Data)
	}
	return instance, nil
}

func newProjectionFromByteValues(resourceMeta metav1.ObjectMeta, configKeyValue map[string][]byte) *projection {
	projection := projection{Name: resourceMeta.Name, ResourceVersion: resourceMeta.ResourceVersion, Generation: resourceMeta.Generation, Files: map[string]propertyFile{}}
	for prop_file_name, data := range configKeyValue {
		projection.Files[prop_file_name] = propertyFile{Alder32: alder32FromData([]byte(data))}
	}
	return &projection
}

func alder32FromData(data []byte) string {
	// need to skip white space and comments for checksum
	keyValuePairs := []string{}

	uniCodeDataLines := strings.Split(string(data), "\n")
	for _, lineToTrim := range uniCodeDataLines {
		line := strings.TrimLeftFunc(lineToTrim, unicode.IsSpace)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			// ignore comments
			continue
		}
		keyValuePairs = appendNonEmpty(keyValuePairs, line)
	}
	return alder32StringValue(alder32Of(keyValuePairs))
}

func appendNonEmpty(propsKvs []string, data string) []string {
	keyAndValue := strings.TrimSpace(string(data))
	if keyAndValue != "" {
		// need to trim space arround the '=' in x = y to match properties loader check sum
		equalsSeparator := "="
		keyAndValueTokens := strings.SplitN(keyAndValue, equalsSeparator, 2)
		numTokens := len(keyAndValueTokens)
		if numTokens == 2 {
			keyAndValue =
				strings.TrimRightFunc(keyAndValueTokens[0], unicode.IsSpace) +
					equalsSeparator +
					strings.TrimLeftFunc(keyAndValueTokens[1], unicode.IsSpace)
		}
		// escaped x will converted on read, need to replace for check sum
		keyAndValue = strings.ReplaceAll(keyAndValue, `\ `, ` `)
		keyAndValue = strings.ReplaceAll(keyAndValue, `\:`, `:`)
		keyAndValue = strings.ReplaceAll(keyAndValue, `\=`, `=`)
		keyAndValue = strings.ReplaceAll(keyAndValue, `\"`, `"`)
		if keyAndValue != "" {
			propsKvs = append(propsKvs, keyAndValue)
		}
	}
	return propsKvs
}

func unmarshallStatus(jsonStatus string) (brokerStatus, error) {
	cmStatus := brokerStatus{}
	err := json.Unmarshal([]byte(jsonStatus), &cmStatus)
	return cmStatus, err
}

func marshallApplyErrors(applyErrors []applyError) string {
	val, _ := json.Marshal(applyErrors)
	return string(val)
}

func getDeploymentSize(cr *brokerv1beta1.ActiveMQArtemis) int32 {
	if cr.Spec.DeploymentPlan.Size == nil {
		return DefaultDeploymentSize
	}
	return *cr.Spec.DeploymentPlan.Size
}
