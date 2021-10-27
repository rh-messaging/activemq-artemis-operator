package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/controller"
	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
	log.Info(fmt.Sprintf("Version of the operator: %s", version.Version))
	log.Info(fmt.Sprintf("Product Version: %s", version.LatestVersion))
	log.Info(fmt.Sprintf("Supported AMQ Broker Product Versions: %s", getSupportedBrokerVersions()))
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	isOpenshift, err1 := environments.DetectOpenshift()
	if err1 != nil {
		log.Error(err1, "Failed to get env")
		os.Exit(1)
	}

	if isOpenshift {
		log.Info("environment is openshift")
	} else {
		log.Info("environment is not openshift")
	}

	printVersion()

	oprNameSpace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		log.Error(err, "failed to get operator namespace")
		os.Exit(1)
	}
	log.Info("Got operator namespace", "operator ns", oprNameSpace)

	watchNameSpace, _ := k8sutil.GetWatchNamespace()
	watchAllNamespaces := false
	if watchNameSpace == "*" || watchNameSpace == "" {
		log.Info("Setting up to watch all namespaces")
		watchAllNamespaces = true
		watchNameSpace = ""
	}
	nsoptions.SetWatchAll(watchAllNamespaces)

	if !watchAllNamespaces && strings.Contains(watchNameSpace, ",") {
		watchList := strings.Split(watchNameSpace, ",")
		nsoptions.SetWatchList(watchList)
		log.Info("Watching multiple namespaces", "value", watchList)
		//watch all namespace but filter out those not in the list
		watchNameSpace = ""
		// following api not available in current client-go
		//mgrOptions = manager.Options{
		//	Namespace:          "",
		//	NewCache:           cache.MultiNamespacedCacheBuilder(watchList),
		//	MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		//}
	} else {
		log.Info("Wating namespace", "namespace", watchNameSpace)
		nsoptions.SetWatchNamespace(watchNameSpace)
	}

	// Expose the operator's namespace and watchNamespace
	if err := os.Setenv("OPERATOR_NAMESPACE", oprNameSpace); err != nil {
		log.Error(err, "failed to set operator's namespace to env")
	}
	if err := os.Setenv("OPERATOR_WATCH_NAMESPACE", watchNameSpace); err != nil {
		log.Error(err, "failed to set operator's watch namespace to env")
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding
	//Should this user service account name instead?
	err = leader.Become(ctx, "activemq-artemis-operator-lock")
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	mgrOptions := manager.Options{
		Namespace:          watchNameSpace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	}

	mgr, err := manager.New(cfg, mgrOptions)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Set the service account name for the drainer pod
	// It will be broken without this as it won't have
	// permission to list the endpoints in drain.sh
	name := os.Getenv("POD_NAME")
	clnt, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Error(err, "can't create client from config")
		os.Exit(1)
	} else {
		setupAccountName(clnt, ctx, oprNameSpace, name)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create Service object to expose the metrics port.
	_, err = metrics.ExposeMetricsPort(ctx, metricsPort)
	if err != nil {
		log.Info(err.Error())
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

func setupAccountName(clnt client.Client, ctx context.Context, ns, podname string) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	key := client.ObjectKey{Namespace: ns, Name: podname}
	err := clnt.Get(ctx, key, pod)
	if err != nil {
		log.Error(err, "failed to get pod")
	} else {
		log.Info("service account name: " + pod.Spec.ServiceAccountName)
		err = os.Setenv("SERVICE_ACCOUNT", pod.Spec.ServiceAccountName)
		if err != nil {
			log.Error(err, "failed to set env variable")
		}
	}
}

func getSupportedBrokerVersions() string {
	allSupportVersions := make([]string, 0, 10)
	relatedImageEnvVarPrefix := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_Kubernetes_"
	// The full env var name should be relatedImageEnvVarPrefix + compactVersion
	for _, envLine := range os.Environ() {
		envPair := strings.Split(envLine, "=")
		if strings.HasPrefix(envPair[0], relatedImageEnvVarPrefix) {
			//try get compact version
			compactVersion := envPair[0][len(relatedImageEnvVarPrefix):]
			if fullVersion, ok := version.FullVersionFromCompactVersion[compactVersion]; ok {
				allSupportVersions = append(allSupportVersions, fullVersion)
			}
		}
	}
	sort.Strings(allSupportVersions)

	supportedProductVersions := ""
	for _, k := range allSupportVersions {
		supportedProductVersions += k
		supportedProductVersions += " "
	}

	return strings.TrimSpace(supportedProductVersions)
}
