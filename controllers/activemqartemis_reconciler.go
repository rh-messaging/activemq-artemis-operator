package controllers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"regexp"
	"sort"
	"unicode"

	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/containers"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/ingresses"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/persistentvolumeclaims"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/routes"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/serviceports"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/jolokia_client"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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
	defaultLivenessProbeInitialDelay = 5
	TCPLivenessPort                  = 8161
	jaasConfigSuffix                 = "-jaas-config"
	loggingConfigSuffix              = "-logging-config"
	brokerPropsSuffix                = "-bp"

	cfgMapPathBase = "/amq/extra/configmaps/"
	secretPathBase = "/amq/extra/secrets/"

	OrdinalPrefix         = "broker-"
	OrdinalPrefixSep      = "."
	BrokerPropertiesName  = "broker.properties"
	JaasConfigKey         = "login.config"
	LoggingConfigKey      = "logging.properties"
	PodNameLabelKey       = "statefulset.kubernetes.io/pod-name"
	ServiceTypePostfix    = "svc"
	RouteTypePostfix      = "rte"
	IngressTypePostfix    = "ing"
	RemoveKeySpecialValue = "-"
)

var defaultMessageMigration bool = true

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
	log                logr.Logger
	customResource     *brokerv1beta1.ActiveMQArtemis
	scheme             *runtime.Scheme
}

func NewActiveMQArtemisReconcilerImpl(customResource *brokerv1beta1.ActiveMQArtemis, logger logr.Logger, schemeArg *runtime.Scheme) *ActiveMQArtemisReconcilerImpl {
	return &ActiveMQArtemisReconcilerImpl{
		log:            logger,
		customResource: customResource,
		scheme:         schemeArg,
	}
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

func (reconciler *ActiveMQArtemisReconcilerImpl) Process(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme) error {

	reconciler.log.V(1).Info("Reconciler Processing...", "Operator version", version.Version, "ActiveMQArtemis release", customResource.Spec.Version)
	reconciler.log.V(2).Info("Reconciler Processing...", "CRD.Name", customResource.Name, "CRD ver", customResource.ObjectMeta.ResourceVersion, "CRD Gen", customResource.ObjectMeta.Generation)

	reconciler.CurrentDeployedResources(customResource, client)

	// currentStateful Set is a clone of what exists if already deployed
	// what follows should transform the resources using the crd
	// if the transformation results in some change, process resources will respect that
	// comparisons should not be necessary, leave that to process resources
	desiredStatefulSet, err := reconciler.ProcessStatefulSet(customResource, namer, client)
	if err != nil {
		reconciler.log.Error(err, "Error processing stafulset")
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

	// this will apply any deltas/updates
	err = reconciler.ProcessResources(customResource, client, scheme)

	//empty the collected objects
	reconciler.requestedResources = nil

	reconciler.log.V(1).Info("Reconciler Processing... complete", "CRD ver:", customResource.ObjectMeta.ResourceVersion, "CRD Gen:", customResource.ObjectMeta.Generation)

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

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessStatefulSet(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client) (*appsv1.StatefulSet, error) {

	reqLogger := reconciler.log.WithName(customResource.Name)

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

	reqLogger.V(2).Info("Reconciling desired statefulset", "name", ssNamespacedName, "current", currentStatefulSet)
	currentStatefulSet, err = reconciler.NewStatefulSetForCR(customResource, namer, currentStatefulSet, client)
	if err != nil {
		reqLogger.Error(err, "Error creating new stafulset")
		return nil, err
	}

	var headlessServiceDefinition *corev1.Service
	headlesServiceName := namer.SvcHeadlessNameBuilder.Name()
	obj = reconciler.cloneOfDeployed(reflect.TypeOf(corev1.Service{}), headlesServiceName)
	if obj != nil {
		headlessServiceDefinition = obj.(*corev1.Service)
	}

	labels := namer.LabelBuilder.Labels()
	headlessServiceDefinition = svc.NewHeadlessServiceForCR2(client, headlesServiceName, ssNamespacedName.Namespace, serviceports.GetDefaultPorts(), labels, headlessServiceDefinition)
	reconciler.trackDesired(headlessServiceDefinition)

	if isClustered(customResource) {
		pingServiceName := namer.SvcPingNameBuilder.Name()
		var pingServiceDefinition *corev1.Service
		obj = reconciler.cloneOfDeployed(reflect.TypeOf(corev1.Service{}), pingServiceName)
		if obj != nil {
			pingServiceDefinition = obj.(*corev1.Service)
		}
		pingServiceDefinition = svc.NewPingServiceDefinitionForCR2(client, pingServiceName, ssNamespacedName.Namespace, labels, labels, pingServiceDefinition)
		reconciler.trackDesired(pingServiceDefinition)
	}

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

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessCredentials(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

	reconciler.log.V(1).Info("ProcessCredentials")

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

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessDeploymentPlan(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

	deploymentPlan := &customResource.Spec.DeploymentPlan

	reconciler.log.V(2).Info("Processing deployment plan", "plan", deploymentPlan, "broker cr", customResource.Name)
	// Ensure the StatefulSet size is the same as the spec
	replicas := common.GetDeploymentSize(customResource)
	currentStatefulSet.Spec.Replicas = &replicas

	reconciler.log.V(2).Info("Now sync Message migration", "for cr", customResource.Name)
	reconciler.syncMessageMigration(customResource, namer, client, scheme)

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

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessAcceptorsAndConnectors(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

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

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessConsole(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme, currentStatefulSet *appsv1.StatefulSet) {

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

func (r *ActiveMQArtemisReconcilerImpl) syncMessageMigration(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme) {

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
			r.log.V(2).Info("Won't set up scaledown for non persistent deployment")
			return
		}
		r.log.V(2).Info("we need scaledown for this cr", "crName", customResource.Name, "scheme", scheme)
		if err = resources.Retrieve(namespacedName, client, scaledown); err != nil {
			// err means not found so create
			r.log.V(2).Info("Creating builtin drainer CR ", "scaledown", scaledown)
			if retrieveError = resources.Create(customResource, client, scheme, scaledown); retrieveError == nil {
				r.log.V(2).Info("drainer created successfully", "drainer", scaledown)
			} else {
				r.log.Error(retrieveError, "we have error retrieving drainer", "drainer", scaledown, "scheme", scheme)
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

func (reconciler *ActiveMQArtemisReconcilerImpl) sourceEnvVarFromSecret(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, currentStatefulSet *appsv1.StatefulSet, envVars *map[string]ValueInfo, secretName string, client rtclient.Client, scheme *runtime.Scheme) {

	var log = reconciler.log.WithName("controller_v1beta1activemqartemis").WithName("sourceEnvVarFromSecret")

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
			log.V(1).Info("Did not find secret", "key", namespacedName)
		} else {
			log.Error(err, "Error while retrieving secret", "key", namespacedName)
		}
		if len(stringDataMap) > 0 {
			// we need to create and track
			desired = true
		}
	} else {
		log.V(2).Info("updating from " + secretName)
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
			log.V(2).Info("containers: failed to retrieve " + envVarName + " creating")
			environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, envVarDefinition)
		}

		//custom init container
		if len(currentStatefulSet.Spec.Template.Spec.InitContainers) > 0 {
			if retrievedEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarName); nil == retrievedEnvVar {
				log.V(2).Info("init_containers: failed to retrieve " + envVarName + " creating")
				environments.Create(currentStatefulSet.Spec.Template.Spec.InitContainers, envVarDefinition)
			}
		}
	}
}

func generateAcceptorsString(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client) string {

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
		if (acceptor.AMQPMinLargeMessageSize > 0) ||
			(acceptor.AMQPMinLargeMessageSize == -1) {
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

func generateConnectorsString(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client) string {

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

func (reconciler *ActiveMQArtemisReconcilerImpl) configureAcceptorsExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme) {
	originalLabels := namer.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	deploymentSize := common.GetDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels[PodNameLabelKey] = namer.SsNameBuilder.Name() + "-" + ordinalString

		for _, acceptor := range customResource.Spec.Acceptors {
			nameSuffix := acceptor.Name + "-" + ordinalString
			serviceName := types.NamespacedName{Namespace: namespacedName.Namespace, Name: namespacedName.Name + "-" + nameSuffix + "-" + ServiceTypePostfix}
			serviceDefinition := reconciler.ServiceDefinitionForCR(serviceName, client, nameSuffix, acceptor.Port, serviceRoutelabels, namer.LabelBuilder.Labels())
			reconciler.checkExistingService(customResource, serviceDefinition, client)
			reconciler.trackDesired(serviceDefinition)

			if acceptor.Expose {
				exposureDefinition := reconciler.ExposureDefinitionForCR(customResource, namespacedName, serviceRoutelabels, acceptor.SSLEnabled, acceptor.IngressHost, ordinalString, acceptor.Name)
				reconciler.trackDesired(exposureDefinition)
			}
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ServiceDefinitionForCR(serviceName types.NamespacedName, client rtclient.Client, nameSuffix string, portNumber int32, selectorLabels map[string]string, labels map[string]string) *corev1.Service {
	var serviceDefinition *corev1.Service
	obj := reconciler.cloneOfDeployed(reflect.TypeOf(corev1.Service{}), serviceName.Name)
	if obj != nil {
		serviceDefinition = obj.(*corev1.Service)
	}
	return svc.NewServiceDefinitionForCR(serviceName, client, nameSuffix, portNumber, selectorLabels, labels, serviceDefinition)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ExposureDefinitionForCR(customResource *brokerv1beta1.ActiveMQArtemis, namespacedName types.NamespacedName, labels map[string]string, passthroughTLS bool, ingressHost string, ordinalString string, itemName string) rtclient.Object {

	targetPortName := itemName + "-" + ordinalString
	targetServiceName := customResource.Name + "-" + targetPortName + "-" + ServiceTypePostfix

	if isOpenshift, err := common.DetectOpenshift(); isOpenshift && err == nil {
		reconciler.log.V(1).Info("creating route for "+targetPortName, "service", targetServiceName)

		var existing *routev1.Route = nil
		obj := reconciler.cloneOfDeployed(reflect.TypeOf(routev1.Route{}), targetServiceName+"-"+RouteTypePostfix)
		if obj != nil {
			existing = obj.(*routev1.Route)
		}
		brokerHost := formatTemplatedString(customResource, ingressHost, ordinalString, itemName, RouteTypePostfix)
		return routes.NewRouteDefinitionForCR(existing, namespacedName, labels, targetServiceName, targetPortName, passthroughTLS, customResource.Spec.IngressDomain, brokerHost)
	} else {
		reconciler.log.V(1).Info("creating ingress for "+targetPortName, "service", targetServiceName)

		var existing *netv1.Ingress = nil
		obj := reconciler.cloneOfDeployed(reflect.TypeOf(netv1.Ingress{}), targetServiceName+"-"+IngressTypePostfix)
		if obj != nil {
			existing = obj.(*netv1.Ingress)
		}
		brokerHost := formatTemplatedString(customResource, ingressHost, ordinalString, itemName, IngressTypePostfix)
		return ingresses.NewIngressForCRWithSSL(existing, namespacedName, labels, targetServiceName, targetPortName, passthroughTLS, customResource.Spec.IngressDomain, brokerHost)
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) trackDesired(desired rtclient.Object) {
	reconciler.requestedResources = append(reconciler.requestedResources, desired)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) applyTemplates(desired rtclient.Object) (err error) {
	for index, template := range reconciler.customResource.Spec.ResourceTemplates {
		if err = reconciler.applyTemplate(index, template, desired); err != nil {
			break
		}
	}
	return err
}

func (reconciler *ActiveMQArtemisReconcilerImpl) applyTemplate(index int, template brokerv1beta1.ResourceTemplate, target rtclient.Object) error {
	if match(template, target) {
		ordinal := extractOrdinal(target)
		itemName := extractItemName(target)
		resType := extractResType(target)
		if len(template.Annotations) > 0 {
			modified := make(map[string]string)
			for key, value := range target.GetAnnotations() {
				modified[key] = value
			}
			for key, value := range template.Annotations {
				reconciler.applyFormattedKeyValue(modified, ordinal, itemName, resType, key, value)
			}
			target.SetAnnotations(modified)
		}
		if len(template.Labels) > 0 {
			modified := make(map[string]string)
			for key, value := range target.GetLabels() {
				modified[key] = value
			}
			for key, value := range template.Labels {
				reconciler.applyFormattedKeyValue(modified, ordinal, itemName, resType, key, value)
			}
			target.SetLabels(modified)
		}

		if template.Patch != nil {

			// apply any patch
			converter := runtime.DefaultUnstructuredConverter

			var err error
			var targetAsUnstructured map[string]interface{}

			if targetAsUnstructured, err = converter.ToUnstructured(target); err == nil {
				// patch, part of our CR, needs to be mutable
				patch := make(map[string]interface{})
				for k, v := range template.Patch.Object {
					patch[k] = v
				}
				var patched strategicpatch.JSONMap
				if patched, err = strategicpatch.StrategicMergeMapPatch(targetAsUnstructured, patch, target); err == nil {
					err = converter.FromUnstructuredWithValidation(patched, target, true)
				}
			}
			if err != nil {
				return fmt.Errorf("error applying strategic merge patch from template[%d] to %s, got %v", index, target.GetName(), err)
			}
		}
	}
	return nil
}

func (reconciler *ActiveMQArtemisReconcilerImpl) applyFormattedKeyValue(collection map[string]string, ordinal string, itemName string, resType string, key string, value string) {
	formattedKey := formatTemplatedString(reconciler.customResource, key, ordinal, itemName, resType)
	if value == RemoveKeySpecialValue {
		delete(collection, formattedKey)
	} else {
		collection[formattedKey] = formatTemplatedString(reconciler.customResource, value, ordinal, itemName, resType)
	}
}

func extractItemName(desired rtclient.Object) string {
	return desired.GetName()
}

func extractResType(desired rtclient.Object) string {
	switch desired.(type) {
	case *corev1.Service:
		{
			return ServiceTypePostfix
		}
	case *netv1.Ingress:
		{
			return IngressTypePostfix
		}

	case *routev1.Route:
		{
			return RouteTypePostfix
		}
	}
	return "undefined-res-type"
}

func extractOrdinal(desired rtclient.Object) string {
	var ordinal = "undefined-ordinal"
	switch desired := desired.(type) {
	case *corev1.Service:
		{
			podName, found := desired.Spec.Selector[PodNameLabelKey]
			if found {
				ordinal = ordinalFromPodName(podName)
			}

		}
	case *netv1.Ingress, *routev1.Route:
		{
			podName, found := desired.GetLabels()[PodNameLabelKey]
			if found {
				ordinal = ordinalFromPodName(podName)
			}
		}
	}
	return ordinal
}

func ordinalFromPodName(podName string) string {
	return podName[strings.LastIndex(podName, "-")+1:]
}

func match(template brokerv1beta1.ResourceTemplate, target rtclient.Object) bool {
	if template.Selector == nil {
		return true
	}

	var groupVersion = ""
	if template.Selector.APIGroup != nil {
		groupVersion = *template.Selector.APIGroup
	}

	templateGv, _ := schema.ParseGroupVersion(groupVersion)

	var kind = ""
	if template.Selector.Kind != nil {
		kind = *template.Selector.Kind
	}

	templateGvk := templateGv.WithKind(kind)
	targetGvk := target.GetObjectKind().GroupVersionKind()

	if templateGvk.Group != "" {
		if templateGvk.Group != targetGvk.Group {
			return false
		}
	}

	if templateGvk.Version != "" {
		if templateGvk.Version != targetGvk.Version {
			return false
		}
	}

	if templateGvk.Kind != "" {
		if templateGvk.Kind != targetGvk.Kind {
			return false
		}
	}

	if template.Selector.Name != nil {
		nameMatcher, _ := regexp.Compile(*template.Selector.Name)
		if !nameMatcher.Match([]byte(target.GetName())) {
			return false
		}
	}

	return true
}

func (reconciler *ActiveMQArtemisReconcilerImpl) configureConnectorsExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme) {
	originalLabels := namer.LabelBuilder.Labels()
	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	deploymentSize := common.GetDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels[PodNameLabelKey] = namer.SsNameBuilder.Name() + "-" + ordinalString

		for _, connector := range customResource.Spec.Connectors {

			nameSuffix := connector.Name + "-" + ordinalString
			serviceName := types.NamespacedName{Namespace: namespacedName.Namespace, Name: namespacedName.Name + "-" + nameSuffix + "-" + ServiceTypePostfix}
			serviceDefinition := reconciler.ServiceDefinitionForCR(serviceName, client, nameSuffix, connector.Port, serviceRoutelabels, namer.LabelBuilder.Labels())
			reconciler.checkExistingService(customResource, serviceDefinition, client)
			reconciler.trackDesired(serviceDefinition)

			if connector.Expose {

				exposureDefinition := reconciler.ExposureDefinitionForCR(customResource, namespacedName, serviceRoutelabels, connector.SSLEnabled, connector.IngressHost, ordinalString, connector.Name)

				reconciler.trackDesired(exposureDefinition)
			}
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) configureConsoleExposure(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, scheme *runtime.Scheme) {
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
	deploymentSize := common.GetDeploymentSize(customResource)
	for i := int32(0); i < deploymentSize; i++ {
		ordinalString := strconv.Itoa(int(i))
		var serviceRoutelabels = make(map[string]string)
		for k, v := range originalLabels {
			serviceRoutelabels[k] = v
		}
		serviceRoutelabels[PodNameLabelKey] = namer.SsNameBuilder.Name() + "-" + ordinalString

		targetPortName := consoleName + "-" + ordinalString
		targetServiceName := customResource.Name + "-" + targetPortName + "-" + ServiceTypePostfix
		serviceName := types.NamespacedName{Namespace: namespacedName.Namespace, Name: targetServiceName}
		serviceDefinition := reconciler.ServiceDefinitionForCR(serviceName, client, consoleName, targetPort, serviceRoutelabels, namer.LabelBuilder.Labels())

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
			isOpenshift, _ = common.DetectOpenshift()
			if isOpenshift {
				reconciler.log.V(2).Info("routeDefinition for " + targetPortName)
				var existing *routev1.Route = nil
				obj := reconciler.cloneOfDeployed(reflect.TypeOf(routev1.Route{}), targetServiceName+"-"+RouteTypePostfix)
				if obj != nil {
					existing = obj.(*routev1.Route)
				}
				brokerHost := formatTemplatedString(customResource, customResource.Spec.Console.IngressHost, ordinalString, consoleName, RouteTypePostfix)
				routeDefinition := routes.NewRouteDefinitionForCR(existing, namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled, customResource.Spec.IngressDomain, brokerHost)
				reconciler.trackDesired(routeDefinition)

			} else {
				reconciler.log.V(2).Info("ingress for " + targetPortName)
				var existing *netv1.Ingress = nil
				obj := reconciler.cloneOfDeployed(reflect.TypeOf(netv1.Ingress{}), targetServiceName+"-"+IngressTypePostfix)
				if obj != nil {
					existing = obj.(*netv1.Ingress)
				}
				brokerHost := formatTemplatedString(customResource, customResource.Spec.Console.IngressHost, ordinalString, consoleName, IngressTypePostfix)
				ingressDefinition := ingresses.NewIngressForCRWithSSL(existing, namespacedName, serviceRoutelabels, targetServiceName, targetPortName, console.SSLEnabled, customResource.Spec.IngressDomain, brokerHost)
				reconciler.trackDesired(ingressDefinition)
			}
		}
	}
}

func formatTemplatedString(customResource *brokerv1beta1.ActiveMQArtemis, template string, brokerOrdinal string, itemName string, resType string) string {
	if template != "" {
		template = strings.Replace(template, "$(CR_NAME)", customResource.Name, -1)
		template = strings.Replace(template, "$(CR_NAMESPACE)", customResource.Namespace, -1)
		template = strings.Replace(template, "$(BROKER_ORDINAL)", brokerOrdinal, -1)
		template = strings.Replace(template, "$(ITEM_NAME)", itemName, -1)
		template = strings.Replace(template, "$(RES_TYPE)", resType, -1)
		template = strings.Replace(template, "$(INGRESS_DOMAIN)", customResource.Spec.IngressDomain, -1)
	}
	return template
}

func generateConsoleSSLFlags(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, secretName string) string {

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

func generateAcceptorConnectorSSLArguments(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, client rtclient.Client, secretName string) string {

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
	reqLogger := reconciler.log.WithValues("ActiveMQArtemis Name", customResource.Name)
	reqLogger.V(1).Info("currentDeployedResources")

	var err error
	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		reconciler.checkExistingPersistentVolumes(customResource, client)
	}
	reconciler.deployed, err = common.GetDeployedResources(customResource, client)
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
			reqLogger.V(2).Info("Deployed ", "Type", t, "Name", obj.GetName())
		}
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) ProcessResources(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) (err error) {

	reqLogger := reconciler.log.WithValues("ActiveMQArtemis Name", customResource.Name)

	for index := range reconciler.requestedResources {
		reconciler.requestedResources[index].SetNamespace(customResource.Namespace)
		if err = reconciler.applyTemplates(reconciler.requestedResources[index]); err != nil {
			return err
		}
	}

	var currenCount int
	for index := range reconciler.deployed {
		currenCount += len(reconciler.deployed[index])
	}
	reqLogger.V(1).Info("Processing resources", "num requested", len(reconciler.requestedResources), "num current", currenCount)

	requested := compare.NewMapBuilder().Add(reconciler.requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()

	comparator.Comparator.SetComparator(reflect.TypeOf(appsv1.StatefulSet{}), func(deployed, requested rtclient.Object) (isEqual bool) {
		deployedSs := deployed.(*appsv1.StatefulSet)
		requestedSs := requested.(*appsv1.StatefulSet)
		isEqual = equality.Semantic.DeepEqual(deployedSs.Spec, requestedSs.Spec)
		if isEqual {
			isEqual = equalObjectMeta(&deployedSs.ObjectMeta, &requestedSs.ObjectMeta)
		}
		if !isEqual {
			reqLogger.V(2).Info("unequal", "depoyed", deployedSs, "requested", requestedSs)
		}
		return isEqual
	})

	comparator.Comparator.SetComparator(reflect.TypeOf(netv1.Ingress{}), func(deployed, requested rtclient.Object) (isEqual bool) {
		deployedIngress := deployed.(*netv1.Ingress)
		requestedIngress := requested.(*netv1.Ingress)
		isEqual = equality.Semantic.DeepEqual(deployedIngress.Spec, requestedIngress.Spec)
		if isEqual {
			isEqual = equalObjectMeta(&deployedIngress.ObjectMeta, &requestedIngress.ObjectMeta)
		}
		if !isEqual {
			reqLogger.V(2).Info("unequal", "depoyed", deployedIngress, "requested", requestedIngress)
		}
		return isEqual
	})

	comparator.Comparator.SetComparator(reflect.TypeOf(policyv1.PodDisruptionBudget{}), func(deployed, requested rtclient.Object) (isEqual bool) {
		deployedPdb := deployed.(*policyv1.PodDisruptionBudget)
		requestedPdb := requested.(*policyv1.PodDisruptionBudget)
		isEqual = equality.Semantic.DeepEqual(deployedPdb.Spec, requestedPdb.Spec)
		if isEqual {
			isEqual = equalObjectMeta(&deployedPdb.ObjectMeta, &requestedPdb.ObjectMeta)
		}
		if !isEqual {
			reqLogger.V(2).Info("unequal", "depoyed", deployedPdb, "requested", requestedPdb)
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
		reqLogger.V(1).Info("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			trackError(&compositeError, reconciler.createResource(customResource, client, scheme, resourceToAdd, resourceType))
		}
		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			trackError(&compositeError, reconciler.updateResource(customResource, client, scheme, resourceToUpdate, resourceType))
		}
		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			trackError(&compositeError, reconciler.deleteResource(customResource, client, scheme, resourceToRemove, resourceType))
		}
	}

	if len(compositeError) == 0 {
		return nil
	} else {
		// maybe errors.Join in go1.20
		// using %q(uote) to keep errors separate
		return fmt.Errorf("%q", compositeError)
	}
}

// resourceTemplate means we can modify labels and annotatins so we need to
// respect those in our comparison logic
func equalObjectMeta(deployed *metav1.ObjectMeta, requested *metav1.ObjectMeta) (isEqual bool) {
	var pairs [][2]interface{}
	pairs = append(pairs, [2]interface{}{deployed.Labels, requested.Labels})
	pairs = append(pairs, [2]interface{}{deployed.Annotations, requested.Annotations})
	isEqual = compare.EqualPairs(pairs)
	return isEqual
}

func trackError(compositeError *[]error, err error) {
	if err != nil {
		if *compositeError == nil {
			*compositeError = make([]error, 0, 1)
		}
		*compositeError = append(*compositeError, err)
	}
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {
	reconciler.log.V(1).Info("Adding delta resources, i.e. creating ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.createRequestedResource(customResource, client, scheme, requested, kind)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {
	reconciler.log.V(1).Info("Updating delta resources, i.e. updating ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.updateRequestedResource(customResource, client, scheme, requested, kind)

}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {
	reconciler.log.V(1).Info("Deleting delta resources, i.e. removing ", "name ", requested.GetName(), "of kind ", kind)
	return reconciler.deleteRequestedResource(customResource, client, scheme, requested, kind)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) createRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {
	reconciler.log.V(1).Info("Creating ", "kind ", kind, "named ", requested.GetName())
	return resources.Create(customResource, client, scheme, requested)
}

func (reconciler *ActiveMQArtemisReconcilerImpl) updateRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {
	var updateError error
	if updateError = resources.Update(client, requested); updateError == nil {
		reconciler.log.V(1).Info("updated", "kind ", kind, "named ", requested.GetName())
	} else {
		reconciler.log.V(0).Info("updated Failed", "kind ", kind, "named ", requested.GetName(), "error ", updateError)
	}
	return updateError
}

func (reconciler *ActiveMQArtemisReconcilerImpl) deleteRequestedResource(customResource *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme, requested rtclient.Object, kind reflect.Type) error {

	var deleteError error
	if deleteError := resources.Delete(client, requested); deleteError == nil {
		reconciler.log.V(2).Info("deleted", "kind", kind, " named ", requested.GetName())
	} else {
		reconciler.log.Error(deleteError, "delete Failed", "kind", kind, " named ", requested.GetName())
	}
	return deleteError
}

// older version of the operator would drop the owner reference, we need to adopt such secrets and update them
func (reconciler *ActiveMQArtemisReconcilerImpl) adoptExistingSecretWithNoOwnerRefForUpdate(cr *brokerv1beta1.ActiveMQArtemis, candidate *corev1.Secret, client rtclient.Client) {

	key := types.NamespacedName{Name: candidate.Name, Namespace: candidate.Namespace}
	existingSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), key, existingSecret)
	if err == nil {
		if len(existingSecret.OwnerReferences) == 0 {
			reconciler.updateOwnerReferencesAndMatchVersion(cr, existingSecret, candidate)
			reconciler.addToDeployed(reflect.TypeOf(corev1.Secret{}), existingSecret)
			reconciler.log.V(1).Info("found matching secret without owner reference, reclaiming", "Name", key)
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
				reconciler.log.V(2).Info("found matching service without owner reference, reclaiming", "Name", key)
			} else {
				reconciler.log.V(2).Info("found matching service with unexpected owner reference, it may need manual removal", "Name", key)
			}
		}
	}
}

func (r *ActiveMQArtemisReconcilerImpl) checkExistingPersistentVolumes(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) {
	var i int32
	for i = 0; i < common.GetDeploymentSize(instance); i++ {
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
					r.log.V(1).Info("removing owner ref from pvc to avoid potential data loss")
					pvc.OwnerReferences = newOwnerReferences
					if er := client.Update(context.TODO(), pvc); er != nil {
						r.log.Error(er, "failed to remove ownerReference from pvc", "pvc", *pvc)
					}
				}
			}
		} else {
			if !k8serrors.IsNotFound(err) {
				r.log.Error(err, "got error in getting pvc")
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
		isOpenshift, _ := common.DetectOpenshift()
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

func addNewVolumes(existingNames map[string]string, existing *[]corev1.Volume, newVolumeName *string) {
	if _, ok := existingNames[*newVolumeName]; !ok {
		volume := volumes.MakeVolume(*newVolumeName)
		*existing = append(*existing, volume)
		existingNames[*newVolumeName] = *newVolumeName
	}
}

func (r *ActiveMQArtemisReconcilerImpl) MakeVolumes(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers) []corev1.Volume {

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
		r.log.V(1).Info("Make volumes for ssl console exposure on k8s")
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

func (r *ActiveMQArtemisReconcilerImpl) MakeVolumeMounts(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers) []corev1.VolumeMount {

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
		r.log.V(1).Info("Make volume mounts for ssl console exposure on k8s")
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

func (reconciler *ActiveMQArtemisReconcilerImpl) NewPodTemplateSpecForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, current *corev1.PodTemplateSpec, client rtclient.Client) (*corev1.PodTemplateSpec, error) {

	reqLogger := reconciler.log.WithName(customResource.Name)

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
			labels[key] = value
		}
	}

	pts := pods.MakePodTemplateSpec(current, namespacedName, labels, customResource.Spec.DeploymentPlan.Annotations)
	podSpec := &pts.Spec

	// REVISIT: don't know when this is nil
	if podSpec == nil {
		podSpec = &corev1.PodSpec{}
	}

	podSpec.ImagePullSecrets = customResource.Spec.DeploymentPlan.ImagePullSecrets

	container := containers.MakeContainer(podSpec, customResource.Name, common.ResolveImage(customResource, common.BrokerImageKey), MakeEnvVarArrayForCR(customResource, namer))

	container.Resources = customResource.Spec.DeploymentPlan.Resources

	containerPorts := MakeContainerPorts(customResource)
	if len(containerPorts) > 0 {
		reqLogger.V(1).Info("Adding new ports to main", "len", len(containerPorts))
		container.Ports = containerPorts
	}

	reqLogger.V(2).Info("Checking out extraMounts", "extra config", customResource.Spec.DeploymentPlan.ExtraMounts)

	configMapsToCreate := customResource.Spec.DeploymentPlan.ExtraMounts.ConfigMaps
	secretsToCreate := customResource.Spec.DeploymentPlan.ExtraMounts.Secrets
	brokerPropertiesResourceName, isSecret, brokerPropertiesMapData := reconciler.addResourceForBrokerProperties(customResource, namer)
	if isSecret {
		secretsToCreate = append(secretsToCreate, brokerPropertiesResourceName)
	} else {
		configMapsToCreate = append(configMapsToCreate, brokerPropertiesResourceName)
	}
	extraVolumes, extraVolumeMounts := reconciler.createExtraConfigmapsAndSecretsVolumeMounts(container, configMapsToCreate, secretsToCreate, brokerPropertiesResourceName, brokerPropertiesMapData)

	reqLogger.V(2).Info("Extra volumes", "volumes", extraVolumes)
	reqLogger.V(2).Info("Extra mounts", "mounts", extraVolumeMounts)
	container.VolumeMounts = reconciler.MakeVolumeMounts(customResource, namer)
	if len(extraVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, extraVolumeMounts...)
	}

	container.StartupProbe = reconciler.configureStartupProbe(container, customResource.Spec.DeploymentPlan.StartupProbe)
	container.LivenessProbe = reconciler.configureLivenessProbe(container, customResource.Spec.DeploymentPlan.LivenessProbe)
	container.ReadinessProbe = reconciler.configureReadinessProbe(container, customResource.Spec.DeploymentPlan.ReadinessProbe)

	if len(customResource.Spec.DeploymentPlan.NodeSelector) > 0 {
		reqLogger.V(1).Info("Adding Node Selectors", "len", len(customResource.Spec.DeploymentPlan.NodeSelector))
		podSpec.NodeSelector = customResource.Spec.DeploymentPlan.NodeSelector
	}

	reconciler.configureAffinity(podSpec, &customResource.Spec.DeploymentPlan.Affinity)

	if len(customResource.Spec.DeploymentPlan.Tolerations) > 0 {
		reqLogger.V(1).Info("Adding Tolerations", "len", len(customResource.Spec.DeploymentPlan.Tolerations))
		podSpec.Tolerations = customResource.Spec.DeploymentPlan.Tolerations
	}

	newContainersArray := []corev1.Container{}
	podSpec.Containers = append(newContainersArray, *container)
	brokerVolumes := reconciler.MakeVolumes(customResource, namer)
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

	reqLogger.V(2).Info("Creating init container for broker configuration")
	initContainer := containers.MakeInitContainer(podSpec, customResource.Name, common.ResolveImage(customResource, common.InitImageKey), MakeEnvVarArrayForCR(customResource, namer))
	initContainer.Resources = customResource.Spec.DeploymentPlan.Resources

	var initCmds []string
	var initCfgRootDir = "/init_cfg_root"

	compactVersionToUse, verr := common.DetermineCompactVersionToUse(customResource)
	if verr != nil {
		reqLogger.Error(verr, "failed to get compact version", "Spec.Version", customResource.Spec.Version)
		return nil, verr
	}
	yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.FullVersionFromCompactVersion[compactVersionToUse]]
	yacfgProfileName := version.YacfgProfileName

	//address settings
	addressSettings := customResource.Spec.AddressSettings.AddressSetting
	if len(addressSettings) > 0 {
		reqLogger.V(1).Info("processing address-settings")

		var configYaml strings.Builder
		var configSpecials map[string]string = make(map[string]string)

		brokerYaml, specials := cr2jinja2.MakeBrokerCfgOverrides(customResource, nil, nil)

		configYaml.WriteString(brokerYaml)

		for k, v := range specials {
			configSpecials[k] = v
		}

		byteArray, err := json.Marshal(configSpecials)
		if err != nil {
			reqLogger.Error(err, "failed to marshal specials")
		}
		jsonSpecials := string(byteArray)

		envVarTuneFilePath := "TUNE_PATH"
		outputDir := initCfgRootDir + "/yacfg_etc"

		initCmd := "mkdir -p " + outputDir + "; echo \"" + configYaml.String() + "\" > " + outputDir +
			"/broker.yaml; cat " + outputDir + "/broker.yaml; yacfg --profile " + yacfgProfileName + "/" +
			yacfgProfileVersion + "/default_with_user_address_settings.yaml.jinja2  --tune " +
			outputDir + "/broker.yaml --extra-properties '" + jsonSpecials + "' --output " + outputDir

		reqLogger.V(2).Info("initCmd: " + initCmd)
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

	reqLogger.V(2).Info("Total volumes ", "volumes", podSpec.Volumes)

	var mountPoint = secretPathBase
	if !isSecret {
		mountPoint = cfgMapPathBase
	}
	brokerPropsValue := brokerPropertiesConfigSystemPropValue(customResource, mountPoint, brokerPropertiesResourceName, brokerPropertiesMapData)

	// only use init container JAVA_OPTS on existing deployments and migrate to JDK_JAVA_OPTIONS for independence
	// from init containers and broker run scripts
	if environments.Retrieve(podSpec.InitContainers, "JAVA_OPTS") != nil {
		// this depends on init container passing --java-opts to artemis create via launch.sh *and* it
		// not getting munged on the way. We CreateOrAppend to any value from spec.Env
		javaOpts := corev1.EnvVar{
			Name:  "JAVA_OPTS",
			Value: brokerPropsValue,
		}
		environments.CreateOrAppend(podSpec.InitContainers, &javaOpts)
	} else {
		jdkJavaOpts := corev1.EnvVar{
			Name:  "JDK_JAVA_OPTIONS",
			Value: brokerPropsValue,
		}
		environments.CreateOrAppend(podSpec.Containers, &jdkJavaOpts)
	}

	var initArgs []string = []string{"-c"}

	//provide a way to configuration after launch.sh
	var brokerHandlerCmds []string = []string{}
	brokerConfigHandler := GetBrokerConfigHandler(namespacedName)
	if brokerConfigHandler != nil {
		reqLogger.V(1).Info("there is a config handler")
		handlerCmds := brokerConfigHandler.Config(podSpec.InitContainers, initCfgRootDir+"/security", yacfgProfileVersion, yacfgProfileName)
		reqLogger.V(2).Info("Getting back some init commands", "handlerCmds", handlerCmds)
		if len(handlerCmds) > 0 {
			reqLogger.V(2).Info("appending to initCmd array...")
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

	reqLogger.V(2).Info("The final init cmds to init ", "the cmd array", initArgs)

	podSpec.InitContainers[0].Args = initArgs

	if len(extraVolumeMounts) > 0 {
		podSpec.InitContainers[0].VolumeMounts = append(podSpec.InitContainers[0].VolumeMounts, extraVolumeMounts...)
		reqLogger.V(2).Info("Added some extra mounts to init", "total mounts: ", podSpec.InitContainers[0].VolumeMounts)
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
	reconciler.configPodSecurity(podSpec, &customResource.Spec.DeploymentPlan.PodSecurity)
	reconciler.configurePodSecurityContext(podSpec, customResource.Spec.DeploymentPlan.PodSecurityContext)

	reqLogger.V(2).Info("Final Init spec", "Detail", podSpec.InitContainers)

	pts.Spec = *podSpec

	return pts, nil
}

func brokerPropertiesConfigSystemPropValue(customResource *brokerv1beta1.ActiveMQArtemis, mountPoint, resourceName string, brokerPropertiesData map[string]string) string {
	var result = ""
	if len(brokerPropertiesData) == 1 {
		// single entry, no ordinal subpath - broker will log if arg is not found for the watcher so make conditional
		result = fmt.Sprintf("-Dbroker.properties=%s%s/%s", mountPoint, resourceName, BrokerPropertiesName)
	} else {
		// directory works on broker image versions >= 2.27.1
		result = fmt.Sprintf("-Dbroker.properties=%s%s/,%s%s/%s${STATEFUL_SET_ORDINAL}/", mountPoint, resourceName, mountPoint, resourceName, OrdinalPrefix)
	}

	for _, extraSecretName := range customResource.Spec.DeploymentPlan.ExtraMounts.Secrets {
		if strings.HasSuffix(extraSecretName, brokerPropsSuffix) {
			// formated append to result with comma
			result = fmt.Sprintf("%s,%s%s/", result, secretPathBase, extraSecretName)
		}
	}
	return result
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

func (r *ActiveMQArtemisReconcilerImpl) configureStartupProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {

	var startupProbe *corev1.Probe = container.StartupProbe
	r.log.V(1).Info("Configuring Startup Probe", "existing", startupProbe)

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

func (r *ActiveMQArtemisReconcilerImpl) configureLivenessProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {
	var livenessProbe *corev1.Probe = container.LivenessProbe
	r.log.V(1).Info("Configuring Liveness Probe", "existing", livenessProbe)

	if livenessProbe == nil {
		livenessProbe = &corev1.Probe{}
	}

	if probeFromCr != nil {
		applyNonDefaultedValues(livenessProbe, probeFromCr)

		// not complete in this case!
		if probeFromCr.Exec == nil && probeFromCr.HTTPGet == nil && probeFromCr.TCPSocket == nil {
			r.log.V(1).Info("Adding default TCP check")
			livenessProbe.ProbeHandler = corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(TCPLivenessPort),
				},
			}
		} else if probeFromCr.TCPSocket != nil {
			r.log.V(1).Info("Using user specified TCPSocket")
			livenessProbe.ProbeHandler = corev1.ProbeHandler{
				TCPSocket: probeFromCr.TCPSocket,
			}
		} else {
			r.log.V(1).Info("Using user provided Liveness Probe Exec " + probeFromCr.Exec.String())
			livenessProbe.Exec = probeFromCr.Exec
		}
	} else {
		r.log.V(1).Info("Creating Default Liveness Probe")

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

func (r *ActiveMQArtemisReconcilerImpl) configureReadinessProbe(container *corev1.Container, probeFromCr *corev1.Probe) *corev1.Probe {

	var readinessProbe *corev1.Probe = container.ReadinessProbe
	r.log.V(1).Info("Configuring Readyness Probe", "existing", readinessProbe)

	if readinessProbe == nil {
		readinessProbe = &corev1.Probe{}
	}

	if probeFromCr != nil {
		applyNonDefaultedValues(readinessProbe, probeFromCr)
		if probeFromCr.Exec == nil && probeFromCr.HTTPGet == nil && probeFromCr.TCPSocket == nil {
			r.log.V(2).Info("adding default handler to user provided readiness Probe")

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
		r.log.V(1).Info("creating default readiness Probe")
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

func (reconciler *ActiveMQArtemisReconcilerImpl) addResourceForBrokerProperties(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers) (string, bool, map[string]string) {

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
		reconciler.log.V(1).Info("Requesting configMap for broker properties", "name", resourceName.Name)
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

	reconciler.log.V(1).Info("Requesting secret for broker properties", "name", resourceName.Name)
	reconciler.trackDesired(desired)

	reconciler.log.V(1).Info("Requesting mount for broker properties secret")
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

func (r *ActiveMQArtemisReconcilerImpl) configureAffinity(podSpec *corev1.PodSpec, affinity *brokerv1beta1.AffinityConfig) {
	if affinity != nil {
		podSpec.Affinity = &corev1.Affinity{}
		if affinity.PodAffinity != nil {
			r.log.V(1).Info("Adding Pod Affinity")
			podSpec.Affinity.PodAffinity = affinity.PodAffinity
		}
		if affinity.PodAntiAffinity != nil {
			r.log.V(1).Info("Adding Pod AntiAffinity")
			podSpec.Affinity.PodAntiAffinity = affinity.PodAntiAffinity
		}
		if affinity.NodeAffinity != nil {
			r.log.V(1).Info("Adding Node Affinity")
			podSpec.Affinity.NodeAffinity = affinity.NodeAffinity
		}
	}
}

func (r *ActiveMQArtemisReconcilerImpl) configurePodSecurityContext(podSpec *corev1.PodSpec, podSecurityContext *corev1.PodSecurityContext) {
	r.log.V(1).Info("Configuring PodSecurityContext")

	if nil != podSecurityContext {
		r.log.V(2).Info("Incoming podSecurityContext is NOT nil, assigning")
		podSpec.SecurityContext = podSecurityContext
	} else {
		r.log.V(2).Info("Incoming podSecurityContext is nil, creating with default values")
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

func (r *ActiveMQArtemisReconcilerImpl) configPodSecurity(podSpec *corev1.PodSpec, podSecurity *brokerv1beta1.PodSecurityType) {
	if podSecurity.ServiceAccountName != nil {
		r.log.V(2).Info("Pod serviceAccountName specified", "existing", podSpec.ServiceAccountName, "new", *podSecurity.ServiceAccountName)
		podSpec.ServiceAccountName = *podSecurity.ServiceAccountName
	}
	if podSecurity.RunAsUser != nil {
		r.log.V(2).Info("Pod runAsUser specified", "runAsUser", *podSecurity.RunAsUser)
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

func (r *ActiveMQArtemisReconcilerImpl) createExtraConfigmapsAndSecretsVolumeMounts(brokerContainer *corev1.Container, configMaps []string, secrets []string, brokePropertiesResourceName string, brokerPropsData map[string]string) ([]corev1.Volume, []corev1.VolumeMount) {

	var extraVolumes []corev1.Volume
	var extraVolumeMounts []corev1.VolumeMount

	if len(configMaps) > 0 {
		for _, cfgmap := range configMaps {
			if cfgmap == "" {
				r.log.V(1).Info("No ConfigMap name specified, ignore", "configMap", cfgmap)
				continue
			}
			cfgmapPath := cfgMapPathBase + cfgmap
			r.log.V(2).Info("Resolved configMap path", "path", cfgmapPath)
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
				r.log.V(2).Info("No Secret name specified, ignore", "Secret", secret)
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

func (reconciler *ActiveMQArtemisReconcilerImpl) NewStatefulSetForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, currentStateFullSet *appsv1.StatefulSet, client rtclient.Client) (*appsv1.StatefulSet, error) {

	reqLogger := reconciler.log.WithName(customResource.Name)

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}
	replicas := common.GetDeploymentSize(customResource)
	currentStateFullSet = ss.MakeStatefulSet(currentStateFullSet, namer.SsNameBuilder.Name(), namer.SvcHeadlessNameBuilder.Name(), namespacedName, nil, namer.LabelBuilder.Labels(), &replicas)

	podTemplateSpec, err := reconciler.NewPodTemplateSpecForCR(customResource, namer, &currentStateFullSet.Spec.Template, client)
	if err != nil {
		reqLogger.Error(err, "Error creating new pod template")
		return nil, err
	}

	if customResource.Spec.DeploymentPlan.PersistenceEnabled {
		currentStateFullSet.Spec.VolumeClaimTemplates = *reconciler.NewPersistentVolumeClaimArrayForCR(customResource, namer, 1)
	}
	currentStateFullSet.Spec.Template = *podTemplateSpec

	return currentStateFullSet, nil
}

func (reconciler *ActiveMQArtemisReconcilerImpl) NewPersistentVolumeClaimArrayForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers, arrayLength int) *[]corev1.PersistentVolumeClaim {

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
		reconciler.applyTemplates(pvc)
		pvcArray = append(pvcArray, *pvc)
	}

	return &pvcArray
}

func MakeEnvVarArrayForCR(customResource *brokerv1beta1.ActiveMQArtemis, namer common.Namers) []corev1.EnvVar {

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
		reqLogger.V(2).Info("There are no available brokers from DeployedCondition", "condition", DeployedCondition)
		return NewUnknownJolokiaError(errors.New("no available brokers from deployed condition"))
	}
	return nil
}

func AssertBrokerPropertiesStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	secretProjection, err := getSecretProjection(getConfigAppliedConfigMapName(cr), client)
	if err != nil {
		reqLogger.V(2).Info("error retrieving config resources. requeing")
		return NewUnknownJolokiaError(err)
	}

	errorStatus := checkStatus(cr, client, secretProjection, func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool) {
		current, present := BrokerStatus.BrokerConfigStatus.PropertiesStatus[FileName]
		return current, present
	})

	if errorStatus == nil {
		for _, extraSecretName := range cr.Spec.DeploymentPlan.ExtraMounts.Secrets {
			if strings.HasSuffix(extraSecretName, brokerPropsSuffix) {

				secretProjection, err = getSecretProjection(types.NamespacedName{Name: extraSecretName, Namespace: cr.Namespace}, client)
				if err != nil {
					reqLogger.V(2).Info("error retrieving -bp extra mount resource. requeing")
					return NewUnknownJolokiaError(err)
				}
				errorStatus = checkStatus(cr, client, secretProjection, func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool) {
					current, present := BrokerStatus.BrokerConfigStatus.PropertiesStatus[FileName]
					return current, present
				})
				if errorStatus == nil {
					updateExtraConfigStatus(cr, secretProjection)
				} else {
					// report the first error
					break
				}
			}
		}
	}

	return errorStatus
}

func AssertJaasPropertiesStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, scheme *runtime.Scheme) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	Projection, err := getConfigMappedJaasProperties(cr, client)
	if err != nil {
		reqLogger.V(2).Info("error retrieving config resources. requeing")
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

func checkStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, secretProjection *projection, extractStatus func(BrokerStatus brokerStatus, FileName string) (propertiesStatus, bool)) ArtemisError {
	reqLogger := ctrl.Log.WithValues("ActiveMQArtemis Name", cr.Name)

	resource := types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}

	ssInfos := ss.GetDeployedStatefulSetNames(client, cr.Namespace, []types.NamespacedName{resource})

	jks := jolokia_client.GetBrokers(resource, ssInfos, client)

	if len(jks) == 0 {
		reqLogger.V(1).Info("not found Jolokia Clients available. requeing")
		return NewJolokiaClientsNotFoundError(errors.New("Waiting for Jolokia Clients to become available"))
	}

	reqLogger.V(2).Info("in sync check", "projection", secretProjection)

	for _, jk := range jks {
		currentJson, err := jk.Artemis.GetStatus()

		if err != nil {
			reqLogger.V(2).Info("unknown status reported from Jolokia.", "IP", jk.IP, "Ordinal", jk.Ordinal, "error", err)
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
		var applyError *inSyncApplyError = nil

		for name, file := range secretProjection.Files {

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
				reqLogger.V(1).Info(err.Error(), "status", brokerStatus, "tracked", secretProjection)
				return NewStatusOutOfSyncError(err)
			}

			if file.Alder32 != current.Alder32 {
				err = errors.Errorf("out of sync on pod %s-%s, mismatched checksum on property file %s, expected: %s, current: %s. A delay can occur before a volume mount projection is refreshed.",
					namer.CrToSS(cr.Name), jk.Ordinal, name, file.Alder32, current.Alder32)
				reqLogger.V(1).Info(err.Error(), "status", brokerStatus, "tracked", secretProjection)
				return NewStatusOutOfSyncError(err)
			}

			// check for apply errors
			if len(current.ApplyErrors) > 0 {
				// some props did not apply for k
				if applyError == nil {
					applyError = NewInSyncWithError(secretProjection, fmt.Sprintf("%s-%s", namer.CrToSS(cr.Name), jk.Ordinal))
				}
				applyError.ErrorApplyDetail(name, marshallApplyErrors(current.ApplyErrors))
			}
		}

		if applyError != nil {
			reqLogger.V(1).Info("in sync with apply error", "error", applyError)
			return *applyError
		}

		if len(missingKeys) > 0 {
			if strings.HasSuffix(secretProjection.Name, jaasConfigSuffix) {
				err = errors.Errorf("out of sync on pod %s-%s, property files are not visible on the broker: %v. Reloadable JAAS LoginModule property files are only visible after the first login attempt that references them. If the property files are for by a third party LoginModule or not reloadable, prefix the property file names with an underscore to exclude them from this condition",
					namer.CrToSS(cr.Name), jk.Ordinal, missingKeys)
			} else {
				err = errors.Errorf("out of sync on pod %s-%s, configuration property files are not visible on the broker: %v. A delay can occur before a volume mount projection is refreshed.",
					namer.CrToSS(cr.Name), jk.Ordinal, missingKeys)
			}
			reqLogger.V(1).Info(err.Error(), "status", brokerStatus, "tracked", secretProjection)
			return NewStatusOutOfSyncMissingKeyError(err)
		}

		// this oridinal is happy
		secretProjection.Ordinals = append(secretProjection.Ordinals, jk.Ordinal)
	}

	reqLogger.V(1).Info("successfully synced with broker", "status", statusMessageFromProjection(secretProjection))
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

func getSecretProjection(secretName types.NamespacedName, client rtclient.Client) (*projection, error) {
	resource := corev1.Secret{}
	err := client.Get(context.TODO(), secretName, &resource)
	if err != nil {
		return nil, NewUnknownJolokiaError(errors.Wrap(err, "unable to retrieve mutable properties secret"))
	}
	return newProjectionFromByteValues(resource.ObjectMeta, resource.Data), nil
}

func getConfigMappedJaasProperties(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (*projection, error) {
	if _, name, found := getConfigExtraMount(cr, jaasConfigSuffix); found {
		return getSecretProjection(types.NamespacedName{Namespace: cr.Namespace, Name: name}, client)
	}
	return nil, nil
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
