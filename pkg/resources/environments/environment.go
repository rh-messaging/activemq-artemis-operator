package environments

import (
	"math"
	"os"
	"strconv"
	"strings"

	svc "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/random"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("package environments")

//TODO: Remove this blatant hack
var GLOBAL_AMQ_CLUSTER_USER string = ""
var GLOBAL_AMQ_CLUSTER_PASSWORD string = ""

type defaults struct {
	AMQ_USER             string
	AMQ_PASSWORD         string
	AMQ_CLUSTER_USER     string
	AMQ_CLUSTER_PASSWORD string
}

var Defaults defaults

func init() {
	if "" == Defaults.AMQ_USER {
		Defaults.AMQ_USER = random.GenerateRandomString(8)
	}
	if "" == Defaults.AMQ_PASSWORD {
		Defaults.AMQ_PASSWORD = random.GenerateRandomString(8)
	}
	if "" == Defaults.AMQ_CLUSTER_USER {
		Defaults.AMQ_CLUSTER_USER = random.GenerateRandomString(8)
		// TODO: remove this hack
		GLOBAL_AMQ_CLUSTER_USER = Defaults.AMQ_CLUSTER_USER
	}
	if "" == Defaults.AMQ_CLUSTER_PASSWORD {
		Defaults.AMQ_CLUSTER_PASSWORD = random.GenerateRandomString(8)
		// TODO: remove this hack
		GLOBAL_AMQ_CLUSTER_PASSWORD = Defaults.AMQ_CLUSTER_PASSWORD
	}
}

func DetectOpenshift() (bool, error) {

	log.Info("Detect if openshift is running")

	value, ok := os.LookupEnv("OPERATOR_OPENSHIFT")
	if ok {
		log.Info("Set by env-var 'OPERATOR_OPENSHIFT': " + value)
		return strings.ToLower(value) == "true", nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Error getting config: %v")
		return false, err
	}

	groupName := "route.openshift.io"
	gv := schema.GroupVersion{Group: groupName, Version: "v1"}
	cfg.APIPath = "/apis"

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: codecs}

	if cfg.UserAgent == "" {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	cfg.GroupVersion = &gv

	client, err := rest.RESTClientFor(cfg)

	if err != nil {
		log.Error(err, "Error getting client: %v")
		return false, err
	}

	_, err = client.Get().DoRaw()

	return err == nil, nil
}

func AddEnvVarForBasic(requireLogin string, journalType string) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_ROLE",
			"admin", //GetPropertyForCR("AMQ_ROLE", cr, "admin"),
			nil,
		},
		{
			"AMQ_NAME",
			"amq-broker", //GetPropertyForCR("AMQ_NAME", cr, "amq-broker"),
			nil,
		},
		{
			"AMQ_TRANSPORTS",
			"", //GetPropertyForCR("AMQ_TRANSPORTS", cr, ""),
			nil,
		},
		{
			"AMQ_QUEUES",
			"", //GetPropertyForCR("AMQ_QUEUES", cr, ""),
			nil,
		},
		{
			"AMQ_ADDRESSES",
			"", //GetPropertyForCR("AMQ_ADDRESSES", cr, ""),
			nil,
		},
		{
			"AMQ_GLOBAL_MAX_SIZE",
			"100 mb", //GetPropertyForCR("AMQ_GLOBAL_MAX_SIZE", cr, "100 mb"),
			nil,
		},
		{
			"AMQ_REQUIRE_LOGIN",
			requireLogin, //GetPropertyForCR("AMQ_REQUIRE_LOGIN", cr, "false"),
			nil,
		},
		{
			"AMQ_EXTRA_ARGS",
			"--no-autotune", //GetPropertyForCR("AMQ_EXTRA_ARGS", cr, "--no-autotune"),
			nil,
		},
		{
			"AMQ_ANYCAST_PREFIX",
			"", //GetPropertyForCR("AMQ_ANYCAST_PREFIX", cr, ""),
			nil,
		},
		{
			"AMQ_MULTICAST_PREFIX",
			"", //GetPropertyForCR("AMQ_MULTICAST_PREFIX", cr, ""),
			nil,
		},
		{
			"POD_NAMESPACE",
			"", // Set to the field metadata.namespace in current object
			nil,
		},
		{
			"AMQ_JOURNAL_TYPE",
			journalType, //GetPropertyForCR("AMQ_JOURNAL_TYPE", cr, "nio"),
			nil,
		},
		{
			"TRIGGERED_ROLL_COUNT",
			"0",
			nil,
		},
		{
			"PING_SVC_NAME",
			svc.PingNameBuilder.Name(),
			nil,
		},
	}

	return envVarArray
}

func AddEnvVarForBasic2(requireLogin string, journalType string, svcPingName string) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_ROLE",
			"admin", //GetPropertyForCR("AMQ_ROLE", cr, "admin"),
			nil,
		},
		{
			"AMQ_NAME",
			"amq-broker", //GetPropertyForCR("AMQ_NAME", cr, "amq-broker"),
			nil,
		},
		{
			"AMQ_TRANSPORTS",
			"", //GetPropertyForCR("AMQ_TRANSPORTS", cr, ""),
			nil,
		},
		{
			"AMQ_QUEUES",
			"", //GetPropertyForCR("AMQ_QUEUES", cr, ""),
			nil,
		},
		{
			"AMQ_ADDRESSES",
			"", //GetPropertyForCR("AMQ_ADDRESSES", cr, ""),
			nil,
		},
		{
			"AMQ_GLOBAL_MAX_SIZE",
			"100 mb", //GetPropertyForCR("AMQ_GLOBAL_MAX_SIZE", cr, "100 mb"),
			nil,
		},
		{
			"AMQ_REQUIRE_LOGIN",
			requireLogin, //GetPropertyForCR("AMQ_REQUIRE_LOGIN", cr, "false"),
			nil,
		},
		{
			"AMQ_EXTRA_ARGS",
			"--no-autotune", //GetPropertyForCR("AMQ_EXTRA_ARGS", cr, "--no-autotune"),
			nil,
		},
		{
			"AMQ_ANYCAST_PREFIX",
			"", //GetPropertyForCR("AMQ_ANYCAST_PREFIX", cr, ""),
			nil,
		},
		{
			"AMQ_MULTICAST_PREFIX",
			"", //GetPropertyForCR("AMQ_MULTICAST_PREFIX", cr, ""),
			nil,
		},
		{
			"POD_NAMESPACE",
			"", // Set to the field metadata.namespace in current object
			nil,
		},
		{
			"AMQ_JOURNAL_TYPE",
			journalType, //GetPropertyForCR("AMQ_JOURNAL_TYPE", cr, "nio"),
			nil,
		},
		{
			"TRIGGERED_ROLL_COUNT",
			"0",
			nil,
		},
		{
			"PING_SVC_NAME",
			svcPingName,
			nil,
		},
	}

	return envVarArray
}

func AddEnvVarForPersistent(customResourceName string) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_DATA_DIR",
			"/opt/" + customResourceName + "/data", //GetPropertyForCR("AMQ_DATA_DIR", cr, "/opt/"+cr.Name+"/data"),
			nil,
		},
		{
			"AMQ_DATA_DIR_LOGGING",
			"true", //GetPropertyForCR("AMQ_DATA_DIR_LOGGING", cr, "true"),
			nil,
		},
	}

	return envVarArray
}

func AddEnvVarForCluster() []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_CLUSTERED",
			"true", //GetPropertyForCR("AMQ_CLUSTERED", cr, "true"),
			nil,
		},
	}

	return envVarArray
}

func AddEnvVarForJolokia(jolokiaAgentEnabled string) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_ENABLE_JOLOKIA_AGENT",
			jolokiaAgentEnabled,
			nil,
		},
	}

	return envVarArray
}

func AddEnvVarForManagement(managementRBACEnabled string) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_ENABLE_MANAGEMENT_RBAC",
			managementRBACEnabled,
			nil,
		},
	}

	return envVarArray
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []corev1.EnvVar, i int) []corev1.EnvVar {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

func BoolSyncCausedUpdateOn(containers []corev1.Container, envVarName string, updatedValue bool) *corev1.EnvVar {

	var retEnvVar *corev1.EnvVar = nil

	found := false
	needsUpdate := false

	// Find the existing values
	for _, v := range containers[0].Env {
		if v.Name == envVarName {
			found = true
			currentValue, _ := strconv.ParseBool(v.Value)
			if currentValue != updatedValue {
				needsUpdate = true
			}
		}
	}

	if !found || needsUpdate {
		retEnvVar = &corev1.EnvVar{
			envVarName,
			strconv.FormatBool(updatedValue),
			nil,
		}
	}

	return retEnvVar
}

func StringSyncCausedUpdateOn(containers []corev1.Container, envVarName string, updatedValue string) *corev1.EnvVar {

	var retEnvVar *corev1.EnvVar = nil

	found := false
	needsUpdate := false

	// Find the existing values
	for _, v := range containers[0].Env {
		if v.Name == envVarName {
			found = true
			currentValue := v.Value
			if currentValue != updatedValue {
				needsUpdate = true
			}
		}
	}

	if !found || needsUpdate {
		retEnvVar = &corev1.EnvVar{
			envVarName,
			updatedValue,
			nil,
		}
	}

	return retEnvVar
}

func IncrementTriggeredRollCount(containers []corev1.Container) error {

	// Find the existing values
	var err error = nil
	var triggeredRollCount int = 0
	for _, v := range containers[0].Env {
		if v.Name == "TRIGGERED_ROLL_COUNT" {
			if triggeredRollCount, err = strconv.Atoi(v.Value); err == nil {
				triggeredRollCount++
				if math.MaxInt32 == triggeredRollCount {
					triggeredRollCount = 0
				}
			}
			break
		}
	}

	newTriggeredRollCountEnvVar := corev1.EnvVar{
		"TRIGGERED_ROLL_COUNT",
		strconv.Itoa(triggeredRollCount),
		nil,
	}
	Update(containers, &newTriggeredRollCountEnvVar)

	return err
}

func Create(containers []corev1.Container, envVar *corev1.EnvVar) {

	for i := 0; i < len(containers); i++ {
		containers[i].Env = append(containers[i].Env, *envVar)
	}
}

func Retrieve(containers []corev1.Container, envVarName string) *corev1.EnvVar {

	var retEnvVar *corev1.EnvVar = nil
	for i := 0; i < len(containers) && nil == retEnvVar; i++ {
		for j := len(containers[i].Env) - 1; j >= 0; j-- {
			if envVarName == containers[i].Env[j].Name {
				retEnvVar = &containers[i].Env[j]
				break
			}
		}
	}

	return retEnvVar
}

func Update(containers []corev1.Container, envVar *corev1.EnvVar) {

	for i := 0; i < len(containers); i++ {
		for j := len(containers[i].Env) - 1; j >= 0; j-- {
			if envVar.Name == containers[i].Env[j].Name {
				containers[i].Env = remove(containers[i].Env, j)
				containers[i].Env = append(containers[i].Env, *envVar)
			}
		}
	}
}

func Delete(containers []corev1.Container, envVarName string) {

	for i := 0; i < len(containers); i++ {
		for j := len(containers[i].Env) - 1; j >= 0; j-- {
			if envVarName == containers[i].Env[j].Name {
				containers[i].Env = remove(containers[i].Env, j)
			}
		}
	}
}
