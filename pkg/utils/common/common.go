package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	osruntime "runtime"

	"github.com/RHsyseng/operator-utils/pkg/olm"
	"github.com/RHsyseng/operator-utils/pkg/resource/read"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/channels"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/blang/semver/v4"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	policyv1 "k8s.io/api/policy/v1"
)

// extra kinds
const (
	ImageNamePrefix        = "RELATED_IMAGE_ActiveMQ_Artemis_Broker_"
	BrokerImageKey         = "Kubernetes"
	InitImageKey           = "Init"
	DefaultDeploymentSize  = int32(1)
	RouteKind              = "Route"
	OpenShiftAPIServerKind = "OpenShiftAPIServer"
	DEFAULT_RESYNC_PERIOD  = 30 * time.Second
	// comments push this over the edge a little when dealing with white space
	// as en env var it can be disabled by setting to "" or can be improved!
	JaasConfigSyntaxMatchRegExDefault = `^(?:(\s*|(?://.*)|(?s:/\*.*\*/))*\S+\s*{(?:(\s*|(?://.*)|(?s:/\*.*\*/))*\S+\s+(?i:required|optional|sufficient|requisite)(?:\s*\S+\s*=((\s*\S+\s*)|("[^"]*")))*\s*;)+(\s*|(?://.*)|(?s:/\*.*\*/))*}\s*;)+\s*\z`
)

var lastStatusMap map[types.NamespacedName]olm.DeploymentStatus = make(map[types.NamespacedName]olm.DeploymentStatus)

var theManager manager.Manager

var resyncPeriod time.Duration = DEFAULT_RESYNC_PERIOD

var jaasConfigSyntaxMatchRegEx = JaasConfigSyntaxMatchRegExDefault

var ClusterDomain *string

type Namers struct {
	SsGlobalName                  string
	SsNameBuilder                 namer.NamerData
	SvcHeadlessNameBuilder        namer.NamerData
	SvcPingNameBuilder            namer.NamerData
	PodsNameBuilder               namer.NamerData
	SecretsCredentialsNameBuilder namer.NamerData
	SecretsConsoleNameBuilder     namer.NamerData
	SecretsNettyNameBuilder       namer.NamerData
	LabelBuilder                  selectors.LabelerData
	GLOBAL_DATA_PATH              string
}

func init() {
	if period, defined := os.LookupEnv("RECONCILE_RESYNC_PERIOD"); defined {
		var err error
		if resyncPeriod, err = time.ParseDuration(period); err != nil {
			resyncPeriod = DEFAULT_RESYNC_PERIOD
		}
	} else {
		resyncPeriod = DEFAULT_RESYNC_PERIOD
	}

	if regEx, defined := os.LookupEnv("JAAS_CONFIG_SYNTAX_MATCH_REGEX"); defined {
		jaasConfigSyntaxMatchRegEx = regEx
	} else {
		jaasConfigSyntaxMatchRegEx = JaasConfigSyntaxMatchRegExDefault
	}
}

func GetJaasConfigSyntaxMatchRegEx() string {
	return jaasConfigSyntaxMatchRegEx
}

func GetReconcileResyncPeriod() time.Duration {
	return resyncPeriod
}

type ActiveMQArtemisConfigHandler interface {
	GetCRName() string
	IsApplicableFor(brokerNamespacedName types.NamespacedName) bool
	Config(initContainers []corev1.Container, outputDirRoot string, yacfgProfileVersion string, yacfgProfileName string) (value []string)
}

func compareQuantities(resList1 corev1.ResourceList, resList2 corev1.ResourceList, keys []corev1.ResourceName) bool {

	for _, key := range keys {
		if q1, ok1 := resList1[key]; ok1 {
			if q2, ok2 := resList2[key]; ok2 {
				if q1.Cmp(q2) != 0 {
					return false
				}
			} else {
				return false
			}
		} else {
			if _, ok2 := resList2[key]; ok2 {
				return false
			}
		}
	}
	return true
}

func CompareRequiredResources(res1 *corev1.ResourceRequirements, res2 *corev1.ResourceRequirements) bool {

	resNames := []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceStorage, corev1.ResourceEphemeralStorage}
	if !compareQuantities(res1.Limits, res2.Limits, resNames) {
		return false
	}

	if !compareQuantities(res1.Requests, res2.Requests, resNames) {
		return false
	}
	return true
}

func ToJson(obj interface{}) (string, error) {
	bytes, err := json.Marshal(obj)
	if err == nil {
		return string(bytes), nil
	}
	return "", err
}

func FromJson(jsonStr *string, obj interface{}) error {
	return json.Unmarshal([]byte(*jsonStr), obj)
}

func SetManager(mgr manager.Manager) {
	theManager = mgr
}

func GetManager() manager.Manager {
	return theManager
}

func NewTrue() *bool {
	b := true
	return &b
}

// Given the operator's namespace and a string representation of
// it's WATCH_NAMESPACE value, this method returns whether
// the operator is watching its own(single) namespace, or watching multiple
// namespaces, or all namespace
// For watching single: it returns (true, nil)
// For watching multiple: it returns (false, [n]string) where n > 0
// For watching all: it returns (false, nil)
func ResolveWatchNamespaceForManager(oprNamespace string, watchNamespace string) (bool, []string) {

	if oprNamespace == watchNamespace {
		return true, nil
	}
	if watchNamespace == "*" || watchNamespace == "" {
		return false, nil
	}
	return false, strings.Split(watchNamespace, ",")
}

func ResolveBrokerVersion(versions []semver.Version, desired string) *semver.Version {

	if len(versions) == 0 {
		return nil
	}
	if desired == "" {
		// latest
		return &versions[len(versions)-1]
	}

	major, minor, patch := resolveVersionComponents(desired)

	// walk the ordered tree in reverse, locking down match based on desired version components
	var i int = len(versions) - 1
	for ; i >= 0; i-- {
		if major != nil {
			if *major == versions[i].Major {
				if minor == nil {
					break
				} else if *minor == versions[i].Minor {
					if patch == nil {
						break
					} else if *patch == versions[i].Patch {
						break
					}
				}
			}
		}
	}
	if i >= 0 {
		return &versions[i]
	}
	return nil
}

func resolveVersionComponents(desired string) (major, minor, patch *uint64) {

	parts := strings.SplitN(desired, ".", 3)
	switch len(parts) {
	case 3:
		if v, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
			patch = &v
		}
		fallthrough
	case 2:
		if v, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
			minor = &v
		}
		fallthrough
	case 1:
		if v, err := strconv.ParseUint(parts[0], 10, 64); err == nil {
			major = &v
		}
	}

	return major, minor, patch
}

func Int32ToPtr(v int32) *int32 {
	return &v
}

func GetClusterDomain() string {
	if ClusterDomain == nil {
		apiSvc := "kubernetes.default.svc"
		cname, err := net.LookupCNAME(apiSvc)
		if err == nil {
			clusterDomain := strings.TrimPrefix(cname, apiSvc)
			clusterDomain = strings.TrimPrefix(clusterDomain, ".")
			clusterDomain = strings.TrimSuffix(clusterDomain, ".")
			ClusterDomain = &clusterDomain
		} else {
			defaultClusterDomain := "cluster.local"
			ClusterDomain = &defaultClusterDomain
		}
	}

	return *ClusterDomain
}

func DetermineCompactVersionToUse(customResource *brokerv1beta1.ActiveMQArtemis) (string, error) {
	log := ctrl.Log.WithName("util_common")
	resolvedFullVersion, err := ResolveBrokerVersionFromCR(customResource)
	if err != nil {
		log.Error(err, "failed to determine broker version from cr")
		return "", err
	}
	compactVersionToUse := version.CompactActiveMQArtemisVersion(resolvedFullVersion)

	return compactVersionToUse, nil
}

func ResolveBrokerVersionFromCR(cr *brokerv1beta1.ActiveMQArtemis) (string, error) {

	if cr.Spec.Version != "" {
		_, verr := semver.ParseTolerant(cr.Spec.Version)
		if verr != nil {
			return "", verr
		}
	}

	result := ResolveBrokerVersion(version.SupportedActiveMQArtemisSemanticVersions(), cr.Spec.Version)
	if result == nil {
		return "", errors.Errorf("did not find a matching broker in the supported list for %v", cr.Spec.Version)
	}
	return result.String(), nil
}

func DetermineImageToUse(customResource *brokerv1beta1.ActiveMQArtemis, imageTypeKey string) string {

	log := ctrl.Log.WithName("util_common")
	found := false
	imageName := ""
	compactVersionToUse, _ := DetermineCompactVersionToUse(customResource)

	genericRelatedImageEnvVarName := ImageNamePrefix + imageTypeKey + "_" + compactVersionToUse
	// Default case of x86_64/amd64 covered here
	archSpecificRelatedImageEnvVarName := genericRelatedImageEnvVarName
	if osruntime.GOARCH == "arm64" || osruntime.GOARCH == "s390x" || osruntime.GOARCH == "ppc64le" {
		archSpecificRelatedImageEnvVarName = genericRelatedImageEnvVarName + "_" + osruntime.GOARCH
	}
	imageName, found = os.LookupEnv(archSpecificRelatedImageEnvVarName)
	log.V(1).Info("DetermineImageToUse", "env", archSpecificRelatedImageEnvVarName, "imageName", imageName)

	// Use genericRelatedImageEnvVarName if archSpecificRelatedImageEnvVarName is not found
	if !found {
		imageName, found = os.LookupEnv(genericRelatedImageEnvVarName)
		log.V(1).Info("DetermineImageToUse - from generic", "env", genericRelatedImageEnvVarName, "imageName", imageName)
	}

	// Use latest images if archSpecificRelatedImageEnvVarName and genericRelatedImageEnvVarName are not found
	if !found {
		imageName = version.DefaultImageName(archSpecificRelatedImageEnvVarName)
		log.V(1).Info("DetermineImageToUse - from default", "env", archSpecificRelatedImageEnvVarName, "imageName", imageName)
	}

	return imageName
}

func ProcessStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName, namer Namers, reconcileError error) {

	reqLogger := ctrl.Log.WithName("util_process_status").WithValues("ActiveMQArtemis Name", cr.Name)

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

func updateVersionStatus(cr *brokerv1beta1.ActiveMQArtemis) {
	cr.Status.Version.Image = ResolveImage(cr, BrokerImageKey)
	cr.Status.Version.InitImage = ResolveImage(cr, InitImageKey)
	cr.Status.Version.BrokerVersion, _ = ResolveBrokerVersionFromCR(cr)

	if isLockedDown(cr.Spec.DeploymentPlan.Image) || isLockedDown(cr.Spec.DeploymentPlan.InitImage) {
		cr.Status.Upgrade.SecurityUpdates = false
		cr.Status.Upgrade.MajorUpdates = false
		cr.Status.Upgrade.MinorUpdates = false
		cr.Status.Upgrade.PatchUpdates = false

	} else {
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

func ResolveImage(customResource *brokerv1beta1.ActiveMQArtemis, key string) string {
	var imageName string

	if key == InitImageKey && isLockedDown(customResource.Spec.DeploymentPlan.InitImage) {
		imageName = customResource.Spec.DeploymentPlan.InitImage
	} else if key == BrokerImageKey && isLockedDown(customResource.Spec.DeploymentPlan.Image) {
		imageName = customResource.Spec.DeploymentPlan.Image
	} else {
		imageName = DetermineImageToUse(customResource, key)
	}
	return imageName
}

func isLockedDown(imageAttribute string) bool {
	return imageAttribute != "placeholder" && imageAttribute != ""
}

func ValidateBrokerImageVersion(customResource *brokerv1beta1.ActiveMQArtemis) *metav1.Condition {
	var result *metav1.Condition = nil

	_, err := ResolveBrokerVersionFromCR(customResource)
	if err != nil {
		result = &metav1.Condition{
			Type:    brokerv1beta1.ValidConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.ValidConditionInvalidVersionReason,
			Message: fmt.Sprintf(".Spec.Version does not resolve to a supported broker version, reason %v", err),
		}
	} else {
		if isLockedDown(customResource.Spec.DeploymentPlan.Image) || isLockedDown(customResource.Spec.DeploymentPlan.InitImage) {
			if !isLockedDown(customResource.Spec.DeploymentPlan.InitImage) || !isLockedDown(customResource.Spec.DeploymentPlan.Image) {
				result = &metav1.Condition{
					Type:    brokerv1beta1.ValidConditionType,
					Status:  metav1.ConditionUnknown,
					Reason:  brokerv1beta1.ValidConditionUnknownReason,
					Message: ImageDependentPairMessage,
				}
			} else {
				if customResource.Spec.Version != "" {
					if !version.IsSupportedActiveMQArtemisVersion(customResource.Spec.Version) {
						result = &metav1.Condition{
							Type:    brokerv1beta1.ValidConditionType,
							Status:  metav1.ConditionUnknown,
							Reason:  brokerv1beta1.ValidConditionUnknownReason,
							Message: NotSupportedImageVersionMessage,
						}
					}
				} else {
					result = &metav1.Condition{
						Type:    brokerv1beta1.ValidConditionType,
						Status:  metav1.ConditionUnknown,
						Reason:  brokerv1beta1.ValidConditionUnknownReason,
						Message: UnkonwonImageVersionMessage,
					}
				}
			}
		}
	}

	return result
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

func updatePodStatus(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, namespacedName types.NamespacedName) olm.DeploymentStatus {

	reqLogger := ctrl.Log.WithName("util_update_pod_status").WithValues("ActiveMQArtemis Name", namespacedName.Name)
	reqLogger.V(1).Info("Getting status for pods")

	var status olm.DeploymentStatus
	var lastStatus olm.DeploymentStatus

	lastStatusExist := false
	if lastStatus, lastStatusExist = lastStatusMap[namespacedName]; !lastStatusExist {
		reqLogger.V(2).Info("Creating lastStatus for new CR", "name", namespacedName)
		lastStatus = olm.DeploymentStatus{}
		lastStatusMap[namespacedName] = lastStatus
	}

	ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(namespacedName.Name), Namespace: namespacedName.Namespace}
	sfsFound := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), ssNamespacedName, sfsFound)
	if err == nil {
		status = GetSingleStatefulSetStatus(sfsFound, cr)
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

func GetSingleStatefulSetStatus(ss *appsv1.StatefulSet, cr *brokerv1beta1.ActiveMQArtemis) olm.DeploymentStatus {
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

	deploymentSize := GetDeploymentSize(cr)
	if deploymentSize == 0 {
		return metav1.Condition{
			Type:    brokerv1beta1.DeployedConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.DeployedConditionZeroSizeReason,
			Message: DeployedConditionZeroSizeMessage,
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

func GetDeploymentSize(cr *brokerv1beta1.ActiveMQArtemis) int32 {
	if cr.Spec.DeploymentPlan.Size == nil {
		return DefaultDeploymentSize
	}
	return *cr.Spec.DeploymentPlan.Size
}

func GetDeployedResources(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) (map[reflect.Type][]rtclient.Object, error) {
	log := ctrl.Log.WithName("util_common")
	reader := read.New(client).WithNamespace(instance.Namespace).WithOwnerObject(instance)
	var resourceMap map[reflect.Type][]rtclient.Object
	var err error
	if isOpenshift, _ := DetectOpenshift(); isOpenshift {
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

func DetectOpenshift() (bool, error) {
	log := ctrl.Log.WithName("util_common")
	value, ok := os.LookupEnv("OPERATOR_OPENSHIFT")
	if ok {
		log.V(1).Info("Set by env-var 'OPERATOR_OPENSHIFT': " + value)
		return strings.ToLower(value) == "true", nil
	}

	// Find out if we're on OpenShift or Kubernetes
	stateManager := GetStateManager()
	isOpenshift, keyExists := stateManager.GetState(OpenShiftAPIServerKind).(bool)

	if keyExists {
		if isOpenshift {
			log.V(1).Info("environment is openshift")
		} else {
			log.V(1).Info("environment is not openshift")
		}

		return isOpenshift, nil
	}
	return false, errors.New("environment not yet determined")
}

func ApplyAnnotations(objectMeta *metav1.ObjectMeta, annotations map[string]string) {
	if annotations != nil {
		if objectMeta.Annotations == nil {
			objectMeta.Annotations = map[string]string{}
		}
		for k, v := range annotations {
			objectMeta.Annotations[k] = v
		}
	}
}

func ToResourceList(resourceMap map[reflect.Type]map[string]rtclient.Object) []rtclient.Object {
	var resourceList []rtclient.Object
	for _, resMap := range resourceMap {
		for _, res := range resMap {
			resourceList = append(resourceList, res)
		}
	}
	return resourceList
}

func HasVolumeInStatefulset(ss *appsv1.StatefulSet, volumeName string) bool {
	for _, v := range ss.Spec.Template.Spec.Volumes {
		if v.Name == volumeName {
			return true
		}
	}
	return false
}

func HasVolumeMount(container *corev1.Container, mountName string) bool {
	for _, vm := range container.VolumeMounts {
		if vm.Name == mountName {
			return true
		}
	}
	return false
}
