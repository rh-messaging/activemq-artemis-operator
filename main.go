/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"
	"sort"
	"strings"

	"github.com/artemiscloud/activemq-artemis-operator/version"
	"github.com/go-logr/logr"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"fmt"
	goruntime "runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/sdkk8sutil"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v1alpha1"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha1"
	brokerv2alpha2 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha2"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha3"
	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha4"
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha5"
	"github.com/artemiscloud/activemq-artemis-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)

var (
	//hard coded because the sdk version pkg is moved in internal package
	sdkVersion = "1.15.0"
	scheme     = runtime.NewScheme()
	log        logr.Logger
)

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion))
	log.Info(fmt.Sprintf("Version of the operator: %s %s", version.Version, version.BuildTimestamp))
	log.Info(fmt.Sprintf("Supported ActiveMQArtemis Kubernetes Image Versions: %s", getSupportedBrokerVersions()))
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))

	utilruntime.Must(brokerv2alpha1.AddToScheme(scheme))
	utilruntime.Must(brokerv2alpha2.AddToScheme(scheme))
	utilruntime.Must(brokerv2alpha3.AddToScheme(scheme))
	utilruntime.Must(brokerv2alpha4.AddToScheme(scheme))
	utilruntime.Must(brokerv2alpha5.AddToScheme(scheme))
	utilruntime.Must(brokerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(brokerv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	os.Args = append(os.Args, strings.Split(os.Getenv("ARGS"), " ")...)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	log = ctrl.Log.WithName("setup")

	printVersion()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Error getting config for APIServer")
		os.Exit(1)
	}

	oprNamespace, err := sdkk8sutil.GetOperatorNamespace()
	if err != nil {
		log.Error(err, "failed to get operator namespace")
		os.Exit(1)
	}
	log.Info("Got operator namespace", "operator ns", oprNamespace)

	watchNamespace, _ := sdkk8sutil.GetWatchNamespace()

	// Expose the operator's namespace and watchNamespace
	if err := os.Setenv("OPERATOR_NAMESPACE", oprNamespace); err != nil {
		log.Error(err, "failed to set operator's namespace to env")
	}
	if err := os.Setenv("OPERATOR_WATCH_NAMESPACE", watchNamespace); err != nil {
		log.Error(err, "failed to set operator's watch namespace to env")
	}

	mgrOptions := ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		//webhook port
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d864aab0.amq.io",
	}

	isLocal, watchList := common.ResolveWatchNamespaceForManager(oprNamespace, watchNamespace)
	if isLocal {
		log.Info("setting up operator to watch local namespace")
		mgrOptions.Namespace = oprNamespace
	} else {
		mgrOptions.Namespace = ""
		if watchList != nil {
			log.Info("setting up operator to watch multiple namespaces", "namespace(s)", watchList)
			mgrOptions.NewCache = cache.MultiNamespacedCacheBuilder(watchList)
		} else {
			log.Info("setting up operator to watch all namespaces")
		}
	}

	mgr, err := ctrl.NewManager(cfg, mgrOptions)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create and start a new auto detect process for this operator
	autodetect, err := common.NewAutoDetect(mgr)
	if err != nil {
		log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		if err := autodetect.DetectOpenshift(); err != nil {
			log.Error(err, "failed in detecting openshift")
			os.Exit(1)
		}
	}

	common.SetManager(mgr)

	// Set the service account name for the drainer pod
	// It will be broken without this as it won't have
	// permission to list the endpoints in drain.sh
	name := os.Getenv("POD_NAME")
	clnt, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Error(err, "can't create client from config")
		os.Exit(1)
	} else {
		setupAccountName(clnt, context.TODO(), oprNamespace, name)
	}

	brokerReconciler := &controllers.ActiveMQArtemisReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = brokerReconciler.SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ActiveMQArtemis")
		os.Exit(1)
	}

	if err = (&controllers.ActiveMQArtemisAddressReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, context.TODO()); err != nil {
		log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisAddress")
		os.Exit(1)
	}
	if err = (&controllers.ActiveMQArtemisScaledownReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisScaledown")
		os.Exit(1)
	}
	if err = (&controllers.ActiveMQArtemisSecurityReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		BrokerReconciler: brokerReconciler,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisSecurity")
		os.Exit(1)
	}

	enableWebhooks := os.Getenv("ENABLE_WEBHOOKS")
	if enableWebhooks != "false" {
		log.Info("Setting up webhook functions", "ENABLE_WEBHOOKS", enableWebhooks)
		if err = (&brokerv1beta1.ActiveMQArtemis{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "ActiveMQArtemis")
			os.Exit(1)
		}
		if err = (&brokerv1beta1.ActiveMQArtemisSecurity{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "ActiveMQArtemisSecurity")
			os.Exit(1)
		}
		if err = (&brokerv1beta1.ActiveMQArtemisAddress{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "ActiveMQArtemisAddress")
			os.Exit(1)
		}
	} else {
		log.Info("NOT Setting up webhook functions", "ENABLE_WEBHOOKS", enableWebhooks)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// again this is moved to sdk's internal package.
	// but we may not need it
	// Create Service object to expose the metrics port.
	//_, err = metrics.ExposeMetricsPort(ctx, metricsPort)
	//if err != nil {
	//	log.Info(err.Error())
	//}

	log.Info("starting the Cmd.")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
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
		log.Error(err, "failed to get pod", "namespace", ns, "pod name", podname)
	} else {
		log.Info("service account name: " + pod.Spec.ServiceAccountName)
		err = os.Setenv("SERVICE_ACCOUNT", pod.Spec.ServiceAccountName)
		if err != nil {
			log.Error(err, "failed to set env variable")
		}
	}
}
