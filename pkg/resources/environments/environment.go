package environments

import (
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/secrets"
	svc "github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/random"
	corev1 "k8s.io/api/core/v1"
	"math"
	"os"
	"strconv"
	"strings"

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
	AMQ_USER     string
	AMQ_PASSWORD string
}

var Defaults defaults

func init() {
	if "" == Defaults.AMQ_USER {
		Defaults.AMQ_USER = random.GenerateRandomString(8)
	}
	if "" == Defaults.AMQ_PASSWORD {
		Defaults.AMQ_PASSWORD = random.GenerateRandomString(8)
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

func GetPropertyForCR(propName string, cr *brokerv2alpha1.ActiveMQArtemis, defaultValue string) string {

	result := defaultValue
	switch propName {
	case "AMQ_REQUIRE_LOGIN":
		if cr.Spec.DeploymentPlan.RequireLogin {
			result = "true"
		} else {
			result = "false"
		}
	case "AMQ_KEYSTORE_TRUSTSTORE_DIR":
		//if checkSSLEnabled(cr) && len(cr.Spec.SSLConfig.SecretName) > 0 {
		if false { // "TODO-FIX-REPLACE"
			//result = "/etc/amq-secret-volume"
		}
	case "AMQ_TRUSTSTORE":
		//if checkSSLEnabled(cr) && len(cr.Spec.SSLConfig.SecretName) > 0 {
		if false { // "TODO-FIX-REPLACE"
			//result = cr.Spec.SSLConfig.TrustStoreFilename
		}
	case "AMQ_TRUSTSTORE_PASSWORD":
		//if checkSSLEnabled(cr) && len(cr.Spec.SSLConfig.SecretName) > 0 {
		if false { // "TODO-FIX-REPLACE"
			//result = cr.Spec.SSLConfig.TrustStorePassword
		}
	case "AMQ_KEYSTORE":
		//if checkSSLEnabled(cr) && len(cr.Spec.SSLConfig.SecretName) > 0 {
		if false { // "TODO-FIX-REPLACE"
			//result = cr.Spec.SSLConfig.KeystoreFilename
		}
	case "AMQ_KEYSTORE_PASSWORD":
		//if checkSSLEnabled(cr) && len(cr.Spec.SSLConfig.SecretName) > 0 {
		if false { // "TODO-FIX-REPLACE"
			//result = cr.Spec.SSLConfig.KeyStorePassword
		}
	case "AMQ_JOURNAL_TYPE":
		if "aio" == strings.ToLower(cr.Spec.DeploymentPlan.JournalType) {
			result = "aio"
		} else {
			result = "nio"
		}
	}
	return result
}

func CheckSSLEnabled(cr *brokerv2alpha1.ActiveMQArtemis) bool {
	reqLogger := log.WithName(cr.Name)
	var sslEnabled = false
	//if len(cr.Spec.SSLConfig.SecretName) != 0 && len(cr.Spec.SSLConfig.KeyStorePassword) != 0 && len(cr.Spec.SSLConfig.KeystoreFilename) != 0 && len(cr.Spec.SSLConfig.TrustStorePassword) != 0 && len(cr.Spec.SSLConfig.TrustStoreFilename) != 0 {
	if false { // "TODO-FIX-REPLACE"
		reqLogger.Info("SSL enabled and SSLConfig Section Provided")
		sslEnabled = true
	}
	return sslEnabled
}

func MakeEnvVarArrayForCR(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.EnvVar {

	reqLogger := log.WithName(cr.Name)
	reqLogger.Info("Adding Env varibale ")
	envVar := []corev1.EnvVar{}
	envVarArrayForBasic := addEnvVarForBasic(cr)
	envVar = append(envVar, envVarArrayForBasic...)

	if CheckSSLEnabled(cr) {
		envVarArrayForSSL := addEnvVarForSSL(cr)
		envVar = append(envVar, envVarArrayForSSL...)
	}
	if cr.Spec.DeploymentPlan.PersistenceEnabled {
		envVarArrayForPresistent := addEnvVarForPersistent(cr)
		envVar = append(envVar, envVarArrayForPresistent...)
	}

	// TODO: Optimize for the single broker configuration
	envVarArrayForCluster := addEnvVarForCluster(cr)
	envVar = append(envVar, envVarArrayForCluster...)

	return envVar
}

func addEnvVarForBasic(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_ROLE",
			GetPropertyForCR("AMQ_ROLE", cr, "admin"),
			nil,
		},
		{
			"AMQ_NAME",
			GetPropertyForCR("AMQ_NAME", cr, "amq-broker"),
			nil,
		},
		{
			"AMQ_TRANSPORTS",
			GetPropertyForCR("AMQ_TRANSPORTS", cr, ""),
			nil,
		},
		{
			"AMQ_QUEUES",
			GetPropertyForCR("AMQ_QUEUES", cr, ""),
			nil,
		},
		{
			"AMQ_ADDRESSES",
			GetPropertyForCR("AMQ_ADDRESSES", cr, ""),
			nil,
		},
		{
			"AMQ_GLOBAL_MAX_SIZE",
			GetPropertyForCR("AMQ_GLOBAL_MAX_SIZE", cr, "100 mb"),
			nil,
		},
		{
			"AMQ_REQUIRE_LOGIN",
			GetPropertyForCR("AMQ_REQUIRE_LOGIN", cr, "false"),
			nil,
		},
		{
			"AMQ_EXTRA_ARGS",
			GetPropertyForCR("AMQ_EXTRA_ARGS", cr, "--no-autotune"),
			nil,
		},
		{
			"AMQ_ANYCAST_PREFIX",
			GetPropertyForCR("AMQ_ANYCAST_PREFIX", cr, ""),
			nil,
		},
		{
			"AMQ_MULTICAST_PREFIX",
			GetPropertyForCR("AMQ_MULTICAST_PREFIX", cr, ""),
			nil,
		},
		{
			"POD_NAMESPACE",
			"", // Set to the field metadata.namespace in current object
			nil,
		},
		{
			"AMQ_JOURNAL_TYPE",
			GetPropertyForCR("AMQ_JOURNAL_TYPE", cr, "nio"),
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

func addEnvVarForPersistent(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_DATA_DIR",
			GetPropertyForCR("AMQ_DATA_DIR", cr, "/opt/"+cr.Name+"/data"),
			nil,
		},
		{
			"AMQ_DATA_DIR_LOGGING",
			GetPropertyForCR("AMQ_DATA_DIR_LOGGING", cr, "true"),
			nil,
		},
	}

	return envVarArray
}

func addEnvVarForSSL(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.EnvVar {

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_KEYSTORE_TRUSTSTORE_DIR",
			GetPropertyForCR("AMQ_KEYSTORE_TRUSTSTORE_DIR", cr, "/etc/amq-secret-volume"),
			nil,
		},
		{
			"AMQ_TRUSTSTORE",
			GetPropertyForCR("AMQ_TRUSTSTORE", cr, ""),
			nil,
		},
		{
			"AMQ_TRUSTSTORE_PASSWORD",
			GetPropertyForCR("AMQ_TRUSTSTORE_PASSWORD", cr, ""),
			nil,
		},
		{
			"AMQ_KEYSTORE",
			GetPropertyForCR("AMQ_KEYSTORE", cr, ""),
			nil,
		},
		{
			"AMQ_KEYSTORE_PASSWORD",
			GetPropertyForCR("AMQ_KEYSTORE_PASSWORD", cr, ""),
			nil,
		},
	}

	return envVarArray

}

func addEnvVarForCluster(cr *brokerv2alpha1.ActiveMQArtemis) []corev1.EnvVar {

	clusterUserEnvVarSource := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secrets.CredentialsNameBuilder.Name(),
			},
			Key:      "clusterUser",
			Optional: nil,
		},
	}

	clusterPasswordEnvVarSource := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secrets.CredentialsNameBuilder.Name(),
			},
			Key:      "clusterPassword",
			Optional: nil,
		},
	}

	envVarArray := []corev1.EnvVar{
		{
			"AMQ_CLUSTERED",
			GetPropertyForCR("AMQ_CLUSTERED", cr, "true"),
			nil,
		},
		{
			"AMQ_CLUSTER_USER",
			"",
			clusterUserEnvVarSource,
		},
		{
			"AMQ_CLUSTER_PASSWORD",
			"",
			clusterPasswordEnvVarSource,
		},
	}

	return envVarArray
}

func newEnvVarArrayForCR(cr *brokerv2alpha1.ActiveMQArtemis) *[]corev1.EnvVar {

	envVarArray := MakeEnvVarArrayForCR(cr)

	return &envVarArray
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
