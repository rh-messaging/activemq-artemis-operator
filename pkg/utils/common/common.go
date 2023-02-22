package common

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// extra kinds
const (
	RouteKind              = "Route"
	OpenShiftAPIServerKind = "OpenShiftAPIServer"
	DEFAULT_RESYNC_PERIOD  = 30 * time.Second
	// comments push this over the edge a little when dealing with white space
	// as en env var it can be disabled by setting to "" or can be improved!
	JaasConfigSyntaxMatchRegExDefault = `^(?:(\s*|(?://.*)|(?s:/\*.*\*/))*\S+\s*{(?:(\s*|(?://.*)|(?s:/\*.*\*/))*\S+\s+(?i:required|optional|sufficient|requisite)+(?:\s*\S+=\S+\s*)*\s*;)+(\s*|(?://.*)|(?s:/\*.*\*/))*}\s*;)+\s*\z`
)

var theManager manager.Manager

var resyncPeriod time.Duration = DEFAULT_RESYNC_PERIOD

var jaasConfigSyntaxMatchRegEx = JaasConfigSyntaxMatchRegExDefault

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
