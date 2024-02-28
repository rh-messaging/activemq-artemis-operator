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

package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"go.uber.org/zap/zapcore"

	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v1alpha1"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha1"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha3"
	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha4"
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha5"

	//+kubebuilder:scaffold:imports

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/ingresses"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// Define utility constants for object names and testing timeouts/durations and intervals.
const (
	defaultNamespace        = "test"
	otherNamespace          = "other"
	restrictedNamespace     = "restricted"
	timeout                 = time.Second * 30
	duration                = time.Second * 10
	interval                = time.Millisecond * 500
	existingClusterTimeout  = time.Second * 180
	existingClusterInterval = time.Second * 2
	namespace1              = "namespace1"
	namespace2              = "namespace2"
	namespace3              = "namespace3"
	specShortNameLimit      = 25
)

var (
	resCount   int64
	specCount  int64
	currentDir string
	k8sClient  client.Client
	restConfig *rest.Config
	testEnv    *envtest.Environment
	ctx        context.Context
	cancel     context.CancelFunc

	// the cluster url
	clusterUrl *url.URL

	// the cluster ingress host
	clusterIngressHost string

	// the manager may be stopped/restarted via tests
	managerCtx    context.Context
	managerCancel context.CancelFunc
	k8Manager     manager.Manager

	stateManager *common.StateManager

	brokerReconciler   *ActiveMQArtemisReconciler
	securityReconciler *ActiveMQArtemisSecurityReconciler

	oprRes = []string{
		"../deploy/service_account.yaml",
		"../deploy/role.yaml",
		"../deploy/role_binding.yaml",
		"../deploy/election_role.yaml",
		"../deploy/election_role_binding.yaml",
		"../deploy/operator.yaml",
	}
	managerChannel chan struct{}

	testWriter = common.BufferWriter{}

	artemisGvk = schema.GroupVersionKind{Group: "broker", Version: "v1beta1", Kind: "ActiveMQArtemis"}

	isOpenshift                    = false
	isIngressSSLPassthroughEnabled = false
	verbose                        = false
	kubeTool                       = "kubectl"
	defaultOperatorInstalled       = true
	defaultUid                     = int64(185)
)

func init() {
	if isVerboseStr, defined := os.LookupEnv("TEST_VERBOSE"); defined {
		if isVerbose, err := strconv.ParseBool(isVerboseStr); err == nil {
			verbose = isVerbose
		}
	}
}

func TestAPIs(t *testing.T) {

	RegisterFailHandler(Fail)

	suiteConfig, _ := GinkgoConfiguration()
	suiteConfig.Timeout = time.Duration(duration.Minutes()) * 20
	RunSpecs(t, "Controller Suite", suiteConfig)
}

func setUpEnvTest() {
	// for run in ide
	// os.Setenv("KUBEBUILDER_ASSETS", " .. <path from makefile> /kubebuilder-envtest/k8s/1.22.1-linux-amd64")
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	testEnv.CRDInstallOptions.CleanUpAfterUse = true

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	clusterUrl, err = url.Parse(testEnv.Config.Host)
	Expect(err).NotTo(HaveOccurred())

	setUpK8sClient()

	setUpIngress()

	setUpNamespace()

	setUpTestProxy()

	stateManager = common.GetStateManager()

	createControllerManagerForSuite()
}

func setUpNamespace() {
	testNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultNamespace,
			Namespace: defaultNamespace,
		},
		Spec: corev1.NamespaceSpec{},
	}

	err := k8sClient.Create(ctx, &testNamespace)
	Expect(err == nil || errors.IsConflict(err))

	if isOpenshift {
		testNamespaceKey := types.NamespacedName{Name: defaultNamespace}
		Expect(k8sClient.Get(ctx, testNamespaceKey, &testNamespace)).Should(Succeed())
		uidRange := testNamespace.Annotations["openshift.io/sa.scc.uid-range"]
		uidRangeTokens := strings.Split(uidRange, "/")
		defaultUid, err = strconv.ParseInt(uidRangeTokens[0], 10, 64)
		Expect(err).Should(Succeed())
	}
}

func setUpIngress() {
	ingressConfig := &configv1.Ingress{}
	ingressConfigKey := types.NamespacedName{Name: "cluster"}
	ingressConfigErr := k8sClient.Get(ctx, ingressConfigKey, ingressConfig)

	if ingressConfigErr == nil {
		isOpenshift = true
		isIngressSSLPassthroughEnabled = true
		clusterIngressHost = "ingress." + ingressConfig.Spec.Domain
	} else {
		isOpenshift = false
		isIngressSSLPassthroughEnabled = false
		clusterIngressHost = clusterUrl.Hostname()
		ingressNginxControllerDeployment := &appsv1.Deployment{}
		ingressNginxControllerDeploymentKey := types.NamespacedName{Name: "ingress-nginx-controller", Namespace: "ingress-nginx"}
		err := k8sClient.Get(ctx, ingressNginxControllerDeploymentKey, ingressNginxControllerDeployment)
		if err == nil {
			if len(ingressNginxControllerDeployment.Spec.Template.Spec.Containers) > 0 {
				ingressNginxControllerContainer := &ingressNginxControllerDeployment.Spec.Template.Spec.Containers[0]

				for i := 0; i < len(ingressNginxControllerContainer.Args); i++ {
					if ingressNginxControllerContainer.Args[i] == "--enable-ssl-passthrough" {
						isIngressSSLPassthroughEnabled = true
						break
					}
				}

				if !isIngressSSLPassthroughEnabled && os.Getenv("ENABLE_INGRESS_SSL_PASSTHROUGH") == "true" {
					ingressNginxControllerContainer.Args = append(ingressNginxControllerContainer.Args, "--enable-ssl-passthrough")

					Expect(k8sClient.Update(ctx, ingressNginxControllerDeployment)).Should(Succeed())

					isIngressSSLPassthroughEnabled = true
				}
			}
		}
	}
}

// Set up test-proxy for external http requests
func setUpTestProxy() {

	var err error
	testProxyPort := int32(3129)
	testProxyDeploymentReplicas := int32(1)
	testProxyName := "test-proxy"
	testProxyNamespace := "default"
	testProxyHost := testProxyName + ".tests.artemiscloud.io"
	testProxyLabels := map[string]string{"app": "test-proxy"}
	testProxyScript := fmt.Sprintf("openssl req -newkey rsa:2048 -nodes -keyout %[1]s -x509 -days 365 -out %[2]s -subj '/CN=test-proxy' && "+
		"echo 'https_port %[3]d tls-cert=%[2]s tls-key=%[1]s' >> %[4]s && "+"entrypoint.sh -f %[4]s -NYC",
		"/etc/squid/key.pem", "/etc/squid/certificate.pem", 3129, "/etc/squid/squid.conf")

	testProxyDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testProxyName + "-dep",
			Namespace: testProxyNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: testProxyLabels,
			},
			Replicas: &testProxyDeploymentReplicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: testProxyLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "ningx",
							Image:   "ubuntu/squid:edge",
							Command: []string{"sh", "-c", testProxyScript},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: testProxyPort,
									Protocol:      "TCP",
								},
							},
						},
					},
				},
			},
		},
	}

	err = k8sClient.Create(ctx, &testProxyDeployment)
	Expect(err != nil || errors.IsConflict(err))

	testProxyService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testProxyName + "-dep-svc",
			Namespace: testProxyNamespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: testProxyLabels,
			Ports: []corev1.ServicePort{
				{
					Port:       testProxyPort,
					TargetPort: intstr.IntOrString{IntVal: testProxyPort},
				},
			},
		},
	}

	err = k8sClient.Create(ctx, &testProxyService)
	Expect(err != nil || errors.IsConflict(err))

	testProxyIngress := ingresses.NewIngressForCRWithSSL(
		nil, types.NamespacedName{Name: testProxyName, Namespace: testProxyNamespace},
		map[string]string{}, testProxyName+"-dep-svc", strconv.FormatInt(int64(testProxyPort), 10),
		true, "", testProxyHost, isOpenshift)

	err = k8sClient.Create(ctx, testProxyIngress)
	Expect(err != nil || errors.IsConflict(err))

	proxyUrl, err := url.Parse(fmt.Sprintf("https://%s:%d", testProxyHost, 443))
	Expect(err).NotTo(HaveOccurred())

	http.DefaultTransport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, testProxyHost) {
				addr = clusterIngressHost + ":443"
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		Proxy:           http.ProxyURL(proxyUrl),
		TLSClientConfig: &tls.Config{ServerName: testProxyHost, InsecureSkipVerify: true},
	}
}

func cleanUpTestProxy() {
	var err error

	testProxyName := "test-proxy"
	testProxyNamespace := "default"

	testProxyDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testProxyName + "-dep",
			Namespace: testProxyNamespace,
		},
	}

	err = k8sClient.Delete(ctx, &testProxyDeployment)
	Expect(err != nil || errors.IsNotFound(err))

	testProxyService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testProxyName + "-dep-svc",
			Namespace: testProxyNamespace,
		},
	}

	err = k8sClient.Delete(ctx, &testProxyService)
	Expect(err != nil || errors.IsNotFound(err))

	testProxyIngress := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testProxyName + "-dep-svc-ing",
			Namespace: testProxyNamespace,
		},
	}

	err = k8sClient.Delete(ctx, &testProxyIngress)
	Expect(err != nil || errors.IsNotFound(err))
}

func createControllerManagerForSuite() {
	createControllerManager(false, "")
}

func createControllerManager(disableMetrics bool, watchNamespace string) {

	managerCtx, managerCancel = context.WithCancel(ctx)

	mgrOptions := ctrl.Options{
		Scheme: scheme.Scheme,
	}

	isLocal, watchList := common.ResolveWatchNamespaceForManager(defaultNamespace, watchNamespace)
	if isLocal {
		ctrl.Log.Info("setting up operator to watch local namespace")
		mgrOptions.Cache.DefaultNamespaces = map[string]cache.Config{
			defaultNamespace: {}}
	} else {
		if watchList != nil {
			if len(watchList) == 1 {
				ctrl.Log.Info("setting up operator to watch single namespace")
			} else {
				ctrl.Log.Info("setting up operator to watch multiple namespaces", "namespace(s)", watchList)
			}
			nsMap := map[string]cache.Config{}
			for _, ns := range watchList {
				nsMap[ns] = cache.Config{}
			}
			mgrOptions.Cache.DefaultNamespaces = nsMap
		} else {
			ctrl.Log.Info("setting up operator to watch all namespaces")
		}
	}

	if disableMetrics {
		// if we can shutdown metrics port, we don't need disable it.
		mgrOptions.Metrics.BindAddress = "0"
	}

	waitforever := time.Duration(-1)
	mgrOptions.GracefulShutdownTimeout = &waitforever
	mgrOptions.LeaderElectionReleaseOnCancel = true

	// start our controler
	var err error
	k8Manager, err = ctrl.NewManager(restConfig, mgrOptions)
	Expect(err).ToNot(HaveOccurred())

	// Create and start a new auto detect process for this operator
	autodetect, err := common.NewAutoDetect(k8Manager)
	if err != nil {
		ctrl.Log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		autodetect.DetectOpenshift()
	}

	isOpenshift, err = common.DetectOpenshift()
	Expect(err).NotTo(HaveOccurred())

	if isOpenshift {
		kubeTool = "oc"
	}

	brokerReconciler = &ActiveMQArtemisReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
		log:    ctrl.Log,
	}

	if err = brokerReconciler.SetupWithManager(k8Manager); err != nil {
		ctrl.Log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisReconciler")
	}

	securityReconciler = &ActiveMQArtemisSecurityReconciler{
		Client:           k8Manager.GetClient(),
		Scheme:           k8Manager.GetScheme(),
		BrokerReconciler: brokerReconciler,
		log:              ctrl.Log,
	}

	err = securityReconciler.SetupWithManager(k8Manager)
	Expect(err).ToNot(HaveOccurred(), "failed to create security controller")

	addressReconciler := &ActiveMQArtemisAddressReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
		log:    ctrl.Log,
	}

	err = addressReconciler.SetupWithManager(k8Manager, managerCtx)
	Expect(err).ToNot(HaveOccurred(), "failed to create address reconciler")

	scaleDownRconciler := &ActiveMQArtemisScaledownReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
		Config: k8Manager.GetConfig(),
		log:    ctrl.Log,
	}

	err = scaleDownRconciler.SetupWithManager(k8Manager)
	Expect(err).ShouldNot(HaveOccurred(), "failed to create scale down reconciler")

	managerChannel = make(chan struct{}, 1)
	go func() {
		defer GinkgoRecover()
		err = k8Manager.Start(managerCtx)
		managerChannel <- struct{}{}
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
}

func shutdownControllerManager() {
	managerCancel()
	// wait for start routine to exit
	<-managerChannel
}

func setUpRealOperator() {
	var err error
	restConfig, err = config.GetConfig()
	Expect(err).ShouldNot(HaveOccurred(), "failed to get the config")

	setUpK8sClient()

	setUpNamespace()

	ctrl.Log.Info("Installing CRDs")
	crds := []string{"../deploy/crds"}
	options := envtest.CRDInstallOptions{
		Paths:              crds,
		ErrorIfPathMissing: false,
	}
	_, err = envtest.InstallCRDs(restConfig, options)
	Expect(err).To(Succeed())

	err = installOperator(nil, defaultNamespace)
	Expect(err).To(Succeed(), "failed to install operator")
}

// Deploy operator resources
// TODO: provide 'watch all namespaces' option
func installOperator(envMap map[string]string, namespace string) error {
	ctrl.Log.Info("#### Installing Operator ####", "ns", namespace)
	for _, res := range oprRes {
		if err := installYamlResource(res, envMap, namespace); err != nil {
			return err
		}
	}

	if namespace == defaultNamespace {
		defaultOperatorInstalled = true
	}

	return waitForOperator(namespace)
}

func uninstallOperator(deleteCrds bool, namespace string) error {
	ctrl.Log.Info("#### Uninstalling Operator ####", "ns", namespace)
	if namespace != defaultNamespace || defaultOperatorInstalled {
		for _, res := range oprRes {
			if err := uninstallYamlResource(res, namespace); err != nil {
				return err
			}
		}
		if namespace == defaultNamespace {
			defaultOperatorInstalled = false
		}
	}

	if deleteCrds {
		//uninstall CRDs
		ctrl.Log.Info("Uninstalling CRDs")
		crds := []string{"../deploy/crds"}
		options := envtest.CRDInstallOptions{
			Paths:              crds,
			ErrorIfPathMissing: false,
		}
		return envtest.UninstallCRDs(restConfig, options)
	}

	return nil
}

func waitForOperator(namespace string) error {
	podList := &corev1.PodList{}
	labelSelector, err := labels.Parse("name=activemq-artemis-operator")
	Expect(err).To(BeNil())
	opts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}

	Eventually(func(g Gomega) {
		g.Expect(k8sClient.List(ctx, podList, opts)).Should(Succeed())
		g.Expect(len(podList.Items)).Should(BeEquivalentTo(1))
		oprPod := podList.Items[0]
		g.Expect(len(oprPod.Status.ContainerStatuses)).Should(BeEquivalentTo(1))
		g.Expect(oprPod.Status.ContainerStatuses[0].Ready).Should(BeTrue())
	}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
	return nil
}

func loadYamlResource(yamlFile string) (runtime.Object, *schema.GroupVersionKind, error) {
	var info os.FileInfo
	var err error
	var filePath string

	// Return the error if ErrorIfPathMissing exists
	if info, err = os.Stat(yamlFile); os.IsNotExist(err) {
		return nil, nil, err
	}
	filePath = filepath.Dir(yamlFile)

	var b []byte
	b, err = os.ReadFile(filepath.Join(filePath, info.Name()))
	if err != nil {
		return nil, nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	robj, gKV, _ := decode(b, nil, nil)
	ctrl.Log.Info("Loaded resource", "kind", gKV.Kind)
	return robj, gKV, nil
}

func uninstallYamlResource(resPath string, namespace string) error {
	ctrl.Log.Info("Uninstalling yaml resource", "yaml", resPath)
	robj, _, err := loadYamlResource(resPath)
	if err != nil {
		return err
	}
	cobj := robj.(client.Object)
	cobj.SetNamespace(namespace)

	err = k8sClient.Delete(ctx, cobj)
	if err != nil {
		return err
	}

	ctrl.Log.Info("Successfully uninstalled", "yaml", resPath)
	return nil
}

func installYamlResource(resPath string, envMap map[string]string, namespace string) error {
	ctrl.Log.Info("Installing yaml resource", "yaml", resPath)
	robj, gkv, err := loadYamlResource(resPath)
	if err != nil {
		return err
	}
	cobj := robj.(client.Object)
	cobj.SetNamespace(namespace)

	if gkv.Kind == "Deployment" {
		oprObj := cobj.(*appsv1.Deployment)
		if oprImg := os.Getenv("IMG"); oprImg != "" {
			ctrl.Log.Info("Using custom operator image", "url", oprImg)
			oprObj.Spec.Template.Spec.Containers[0].Image = oprImg
		}
		for k, v := range envMap {
			ctrl.Log.Info("Adding new env var into operator", "name", k, "value", v)
			newEnv := corev1.EnvVar{
				Name:  k,
				Value: v,
			}
			oprObj.Spec.Template.Spec.Containers[0].Env = append(oprObj.Spec.Template.Spec.Containers[0].Env, newEnv)
		}
	}

	err = k8sClient.Create(ctx, cobj)
	if err != nil {
		return err
	}

	//make sure the create ok
	Eventually(func() bool {
		key := types.NamespacedName{Name: cobj.GetName(), Namespace: namespace}
		err := k8sClient.Get(ctx, key, cobj)
		return err == nil
	}, timeout, interval).Should(Equal(true))

	ctrl.Log.Info("Successfully installed", "yaml", resPath)
	return nil
}

func setUpK8sClient() {

	ctrl.Log.Info("Setting up k8s client")

	err := configv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = routev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv2alpha5.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv2alpha4.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv2alpha3.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv2alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = brokerv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
}

var _ = BeforeSuite(func() {
	opts := zap.Options{
		Development: true,
		DestWriter:  &TestLogWrapper,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if verbose {
		GinkgoWriter.TeeTo(os.Stderr)
	}

	GinkgoWriter.TeeTo(testWriter)

	ctx, cancel = context.WithCancel(context.TODO())

	// force isLocalOnly=false check from artemis reconciler such that scale down controller will create
	// role binding to service account for the drainer pod
	os.Setenv("OPERATOR_WATCH_NAMESPACE", "SomeValueToCauesEqualitytoFailInIsLocalSoDrainControllerSortsCreds")

	var err error
	currentDir, err = os.Getwd()
	Expect(err).To(Succeed())
	ctrl.Log.Info("#### Starting test ####", "working dir", currentDir)
	if os.Getenv("DEPLOY_OPERATOR") == "true" {
		ctrl.Log.Info("#### Setting up a real operator ####")
		setUpRealOperator()
	} else {
		ctrl.Log.Info("#### Setting up EnvTest ####")
		setUpEnvTest()
	}
})

func cleanUpPVC() {
	pvcs := &corev1.PersistentVolumeClaimList{}
	opts := []client.ListOption{}
	k8sClient.List(context.TODO(), pvcs, opts...)
	for _, pvc := range pvcs.Items {
		ctrl.Log.Info("Deleting/GC PVC: " + pvc.Name)
		k8sClient.Delete(context.TODO(), &pvc, &client.DeleteOptions{})
	}
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
		cleanUpPVC()
		cleanUpTestProxy()
	}

	os.Unsetenv("OPERATOR_WATCH_NAMESPACE")

	if os.Getenv("DEPLOY_OPERATOR") == "true" {
		err := uninstallOperator(true, defaultNamespace)
		Expect(err).NotTo(HaveOccurred())
	} else {
		shutdownControllerManager()

		if stateManager != nil {
			stateManager.Clear()
		}

		// scaledown controller lifecycle seems a little loose, it does not complete on signal hander like the others
		for _, drainController := range controllers {
			close(*drainController.GetStopCh())
		}

		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}

})

func StartCapturingLog() {
	testWriter.Buffer = bytes.NewBuffer(nil)
	TestLogWrapper.StartLogging()
}

func MatchCapturedLog(pattern string) (matched bool, err error) {
	return regexp.Match(pattern, testWriter.Buffer.Bytes())
}

func FindAllFromCapturedLog(pattern string) []string {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.FindAllString(testWriter.Buffer.String(), -1)
	}
	return nil
}

func StopCapturingLog() {
	testWriter.Buffer = nil
	TestLogWrapper.StopLogging()
}

func BeforeEachSpec() {
	specCount++

	//Print running spec
	currentSpecReport := CurrentSpecReport()
	fmt.Printf("\n\033[1m\033[32mSpec %d running: %s \033[33m[%s]\033[0m\n%s:%d",
		specCount, currentSpecReport.FullText(), CurrentSpecShortName(),
		currentSpecReport.LeafNodeLocation.FileName, currentSpecReport.LeafNodeLocation.LineNumber)
}

func AfterEachSpec() {
	//Print ran spec
	currentSpecReport := CurrentSpecReport()
	fmt.Printf("\n\033[1m\033[32mSpec %d ran in %f seconds \033[0m\n",
		specCount, currentSpecReport.RunTime.Seconds())
}
