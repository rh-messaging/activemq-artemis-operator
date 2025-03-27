package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	brokerv1beta1 "github.com/arkmq-org/activemq-artemis-operator/api/v1beta1"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/resources"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/utils/channels"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/utils/namer"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/utils/selectors"
	"github.com/arkmq-org/activemq-artemis-operator/version"
	"github.com/blang/semver/v4"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

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

	// defaultRetries is the number of times a resource discovery is retried
	defaultRetries = 10

	//defaultRetryInterval is the interval to wait before retring a resource discovery
	defaultRetryInterval = 3 * time.Second

	// https://cert-manager.io/docs/trust/trust-manager/#preparing-for-production
	DefaultOperatorCertSecretName = "activemq-artemis-manager-cert"
	DefaultOperatorCASecretName   = "activemq-artemis-manager-ca"
	DefaultOperandCertSecretName  = "broker-cert" // or can be prefixed with `cr.Name-`

	BlockReconcileAnnotation = "arkmq.org/block-reconcile"
)

var lastStatusMap map[types.NamespacedName]olm.DeploymentStatus = make(map[types.NamespacedName]olm.DeploymentStatus)

var resyncPeriod time.Duration = DEFAULT_RESYNC_PERIOD

var jaasConfigSyntaxMatchRegEx = JaasConfigSyntaxMatchRegExDefault

var ClusterDomain *string

var isOpenshift *bool

var operatorCertSecretName, operatorCASecretName *string

// we may want to cache and require operator restart on rotation
//var operatorCert *tls.Certificate
//var operatorCertPool *x509.CertPool

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

func NewTrue() *bool {
	b := true
	return &b
}

func NewFalse() *bool {
	b := false
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
	meta.SetStatusCondition(&cr.Status.Conditions, getDeploymentCondition(cr, client, podStatus, ValidCondition.Status != metav1.ConditionFalse, reconcileError))

	if !reflect.DeepEqual(podStatus, cr.Status.PodStatus) {
		reqLogger.V(1).Info("Pods status updated")
		cr.Status.PodStatus = podStatus
	} else {
		// could leave this to kube, it will do a []byte comparison
		reqLogger.V(1).Info("Pods status unchanged")
	}
}

func UpdateBlockedStatus(cr *brokerv1beta1.ActiveMQArtemis, blocked bool) {
	if blocked {
		meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
			Type:    brokerv1beta1.ReconcileBlockedType,
			Status:  metav1.ConditionTrue,
			Reason:  brokerv1beta1.ReconcileBlockedReason,
			Message: "Reconcile blocked by presence of annotation " + BlockReconcileAnnotation,
		})
	} else {
		meta.RemoveStatusCondition(&cr.Status.Conditions, brokerv1beta1.ReconcileBlockedType)
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

func getDeploymentCondition(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, podStatus olm.DeploymentStatus, valid bool, reconcileError error) metav1.Condition {

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
		crReadyCondition := metav1.Condition{
			Type:    brokerv1beta1.DeployedConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  brokerv1beta1.DeployedConditionNotReadyReason,
			Message: fmt.Sprintf("%d/%d pods ready", len(podStatus.Ready), deploymentSize),
		}
		for _, startingPodName := range podStatus.Starting {
			podNamespacedName := types.NamespacedName{Namespace: cr.Namespace, Name: startingPodName}
			pod := &corev1.Pod{}
			if err := client.Get(context.TODO(), podNamespacedName, pod); err == nil {
				ctrl.Log.V(1).Info("Pod "+startingPodName, "starting status", pod.Status)
				crReadyCondition.Message = fmt.Sprintf("%s %s", crReadyCondition.Message, PodStartingStatusDigestMessage(startingPodName, pod.Status))
			}
		}
		return crReadyCondition
	}
	return metav1.Condition{
		Type:   brokerv1beta1.DeployedConditionType,
		Reason: brokerv1beta1.DeployedConditionReadyReason,
		Status: metav1.ConditionTrue,
	}
}

// take useful diagnostic info from the PodStatus for a free form Message string
func PodStartingStatusDigestMessage(podName string, status corev1.PodStatus) string {
	buf := &bytes.Buffer{}

	fmt.Fprintf(buf, "{%s", podName) // open curly 1

	if status.Phase != "" {
		fmt.Fprintf(buf, ": %s", status.Phase)
	}

	if len(status.Conditions) > 0 {
		fmt.Fprintf(buf, " [") // open square
	}
	for _, condition := range status.Conditions {
		fmt.Fprintf(buf, "{%s", condition.DeepCopy().Type) // open curly 2
		if condition.Status != "" {
			fmt.Fprintf(buf, "=%s", condition.Status)
		}
		if condition.Reason != "" {
			fmt.Fprintf(buf, " %s", condition.Reason)
		}
		if condition.Message != "" {
			fmt.Fprintf(buf, " %s", condition.Message)
		}
		fmt.Fprintf(buf, "}") // close curly 2
	}
	if len(status.Conditions) > 0 {
		fmt.Fprintf(buf, "]") // close square
	}
	fmt.Fprintf(buf, "}") // close curly 1
	return buf.String()
}

func IsRestricted(customResource *brokerv1beta1.ActiveMQArtemis) bool {
	return customResource.Spec.Restricted != nil && *customResource.Spec.Restricted
}

func GetDeploymentSize(cr *brokerv1beta1.ActiveMQArtemis) int32 {
	if cr.Spec.DeploymentPlan.Size == nil || IsRestricted(cr) {
		return DefaultDeploymentSize
	}
	return *cr.Spec.DeploymentPlan.Size
}

func GetDeployedResources(instance *brokerv1beta1.ActiveMQArtemis, client rtclient.Client, onOpenShift bool) (map[reflect.Type][]rtclient.Object, error) {
	log := ctrl.Log.WithName("util_common")
	reader := read.New(client).WithNamespace(instance.Namespace).WithOwnerObject(instance)
	var resourceMap map[reflect.Type][]rtclient.Object
	var err error
	if onOpenShift {
		resourceMap, err = reader.ListAll(
			&corev1.ServiceList{},
			&appsv1.StatefulSetList{},
			&routev1.RouteList{},
			&corev1.SecretList{},
			&corev1.ConfigMapList{},
			&policyv1.PodDisruptionBudgetList{},
			&netv1.IngressList{},
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

func DetectOpenshiftWith(config *rest.Config) (bool, error) {
	if isOpenshift == nil {
		value, ok := os.LookupEnv("OPERATOR_OPENSHIFT")
		if ok {
			ctrl.Log.V(1).Info("Set by env-var 'OPERATOR_OPENSHIFT': " + value)
			return strings.ToLower(value) == "true", nil
		}

		var err error
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
		if err != nil {
			return false, err
		}

		var isOpenShiftResourcePresent bool
		for i := 0; i < defaultRetries; i++ {
			isOpenShiftResourcePresent, err = discovery.IsResourceEnabled(discoveryClient,
				schema.GroupVersionResource{
					Group:    "route.openshift.io",
					Version:  "v1",
					Resource: "routes",
				})

			if err == nil {
				break
			}

			time.Sleep(defaultRetryInterval)
		}

		if err != nil {
			return false, err
		}

		isOpenshift = &isOpenShiftResourcePresent
	}
	return *isOpenshift, nil
}

func GetOperandCertSecretName(cr *brokerv1beta1.ActiveMQArtemis, client rtclient.Client) string {

	customName := cr.Name + "-" + DefaultOperandCertSecretName

	if _, err := secrets.RetriveSecret(types.NamespacedName{Namespace: cr.Namespace, Name: customName}, nil, client); err == nil {
		return customName
	}
	return DefaultOperandCertSecretName
}

func GetOperatorCertSecretName() string {
	if operatorCertSecretName == nil {
		operatorCertSecretName = fromEnv("ACTIVEMQ_ARTEMIS_MANAGER_CERT_SECRET_NAME", DefaultOperatorCertSecretName)
	}
	return *operatorCertSecretName
}

func GetOperatorCASecretName() string {
	if operatorCASecretName == nil {
		operatorCASecretName = fromEnv("ACTIVEMQ_ARTEMIS_MANAGER_CA_SECRET_NAME", DefaultOperatorCASecretName)
	}
	return *operatorCASecretName
}

func GetOperatorCASecretKey(client rtclient.Client, bundleSecret *corev1.Secret) (key string, err error) {
	if bundleSecret == nil {
		if bundleSecret, err = GetOperatorCASecret(client); err != nil {
			ctrl.Log.V(1).Info("ca secret not found", "err", err)
			return key, errors.Errorf("failed to get ca bundle secret to find ca key %v", err)
		}
	}
	return FindFirstDotPemKey(bundleSecret)
}

func FindFirstDotPemKey(secret *corev1.Secret) (string, error) {
	//extract the bundle target secret key that ends with .pem
	//the bundle target secret could include keys for additional formats jks/pkcs12
	for key := range secret.Data {
		//the bundle target secret key must ends with .pem
		if strings.HasSuffix(key, ".pem") {
			return key, nil
		}
	}

	return "", fmt.Errorf("no keys with the suffix .pem found in the secret %s", secret.Name)
}

func fromEnv(envVarName, defaultValue string) *string {
	if valueFromEnv, found := os.LookupEnv(envVarName); found {
		return &valueFromEnv
	} else {
		return &defaultValue
	}
}

func GetRootCAs(client rtclient.Client) (pool *x509.CertPool, err error) {

	if client == nil {
		return nil, nil
	}

	var certSecret *corev1.Secret
	if certSecret, err = GetOperatorCASecret(client); err != nil {
		return nil, err
	}

	var bundleSecretKey string
	if bundleSecretKey, err = GetOperatorCASecretKey(client, certSecret); err != nil {
		return nil, err
	}

	pool = x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM([]byte(certSecret.Data[bundleSecretKey])); !ok {
		return nil, errors.Errorf("Failed to extact key %s from ca secret %v", bundleSecretKey, certSecret.Name)
	}
	return pool, nil
}

var operatorNameSpaceFromEnv *string

func GetOperatorNamespaceFromEnv() (ns string, err error) {
	if operatorNameSpaceFromEnv == nil {
		if ns, found := os.LookupEnv("OPERATOR_NAMESPACE"); found {
			operatorNameSpaceFromEnv = &ns
		} else {
			return "", errors.New("failed to get operator namespace from env")
		}
	}
	return *operatorNameSpaceFromEnv, nil
}

func GetOperatorCASecret(client rtclient.Client) (*corev1.Secret, error) {
	return GetOperatorSecret(client, GetOperatorCASecretName())
}

func GetOperatorClientCertSecret(client rtclient.Client) (*corev1.Secret, error) {
	return GetOperatorSecret(client, GetOperatorCertSecretName())
}

func GetOperatorSecret(client rtclient.Client, secretName string) (*corev1.Secret, error) {

	var operatorNamespace string
	var err error
	operatorNamespace, err = GetOperatorNamespaceFromEnv()
	if err != nil {
		return nil, err
	}

	secretNamespacedName := types.NamespacedName{Name: secretName, Namespace: operatorNamespace}
	secret := corev1.Secret{}
	if err := resources.Retrieve(secretNamespacedName, client, &secret); err != nil {
		ctrl.Log.V(1).Info("operator secret not found", "name", secretNamespacedName, "err", err)
		return nil, errors.Errorf("failed to get secret %s, %v", secretNamespacedName, err)
	}

	return &secret, nil
}

var operatorHasCertAndTrustBundle *bool

func OperatorHasCertAndTrustBundle(client rtclient.Client) bool {
	if operatorHasCertAndTrustBundle == nil {
		var ok bool = false

		var secret *corev1.Secret
		var err error
		if secret, err = GetOperatorCASecret(client); secret != nil && err == nil {
			if secret, err = GetOperatorClientCertSecret(client); secret != nil && err == nil {
				ok = true
			}
		}
		operatorHasCertAndTrustBundle = &ok
	}
	return *operatorHasCertAndTrustBundle
}

func GetOperatorClientCertificate(client rtclient.Client, info *tls.CertificateRequestInfo) (cert *tls.Certificate, err error) {

	var secret *corev1.Secret
	if secret, err = GetOperatorClientCertSecret(client); err == nil {
		cert, err = ExtractCertFromSecret(secret)
	}
	return cert, err
}

func ExtractCertFromSecret(certSecret *corev1.Secret) (*tls.Certificate, error) {
	cert, err := tls.X509KeyPair(certSecret.Data["tls.crt"], certSecret.Data["tls.key"])
	if err != nil {
		return nil, errors.Errorf("invalid key pair in secret %v, %v", certSecret.Name, err)
	}
	return &cert, nil
}

func ExtractCertSubjectFromSecret(certSecretName string, namespace string, client rtclient.Client) (*pkix.Name, error) {

	secret, err := GetOperatorSecret(client, certSecretName)
	if err != nil {
		return nil, err
	}
	cert, err := ExtractCertFromSecret(secret)
	if err != nil {
		return nil, err
	}
	return ExtractCertSubject(cert)
}

func ExtractCertSubject(cert *tls.Certificate) (*pkix.Name, error) {
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, errors.Errorf("failed to parse tls.cert  %v", err)
	}
	return &x509Cert.Subject, nil
}

func OrdinalFQDNS(crName string, crNamespace string, i int32) string {
	return OrdinalStringFQDNS(crName, crNamespace, fmt.Sprintf("%d", i))
}

func OrdinalStringFQDNS(crName string, crNamespace string, ordinal string) string {
	// from NewHeadlessServiceForCR2 and
	// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	return fmt.Sprintf("%s-ss-%s.%s-hdls-svc.%s.svc.%s", crName, ordinal, crName, crNamespace, GetClusterDomain())
}

func ClusterDNSWildCard(crName string, crNamespace string) string {
	// from NewHeadlessServiceForCR2 and
	// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	return fmt.Sprintf("*.%s-hdls-svc.%s.svc.%s", crName, crNamespace, GetClusterDomain())
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

func GenerateArtemis(name string, namespace string) *brokerv1beta1.ActiveMQArtemis {
	return &brokerv1beta1.ActiveMQArtemis{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{},
	}
}
