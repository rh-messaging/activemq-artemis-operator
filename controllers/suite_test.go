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
	"context"
	"io/ioutil"
	"os"
	"time"

	routev1 "github.com/openshift/api/route/v1"

	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v1alpha1"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha1"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha3"
	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha4"
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha5"

	//+kubebuilder:scaffold:imports

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// Define utility constants for object names and testing timeouts/durations and intervals.
const (
	defaultNamespace        = "default"
	timeout                 = time.Second * 30
	duration                = time.Second * 10
	interval                = time.Millisecond * 500
	existingClusterTimeout  = time.Second * 180
	existingClusterInterval = time.Second * 2
	verobse                 = false
	namespace1              = "namespace1"
	namespace2              = "namespace2"
	namespace3              = "namespace3"
)

var (
	currentDir   string
	k8sClient    client.Client
	restConfig   *rest.Config
	testEnv      *envtest.Environment
	ctx          context.Context
	cancel       context.CancelFunc
	stateManager *common.StateManager

	brokerReconciler   *ActiveMQArtemisReconciler
	securityReconciler *ActiveMQArtemisSecurityReconciler

	oprRes = []string{
		"../deploy/service_account.yaml",
		"../deploy/role.yaml",
		"../deploy/role_binding.yaml",
		"../deploy/election_role.yaml",
		"../deploy/election_role_binding.yaml",
		"../deploy/operator_config.yaml",
		"../deploy/operator.yaml",
	}
	managerChannel = make(chan struct{}, 1)
)

func TestAPIs(t *testing.T) {

	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
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

	setUpK8sClient()

	createControllerManager(false, defaultNamespace)
}

func createControllerManager(disableMetrics bool, watchNamespace string) {
	ctx, cancel = context.WithCancel(context.TODO())

	mgrOptions := ctrl.Options{
		Scheme: scheme.Scheme,
	}

	isLocal, watchList := common.ResolveWatchNamespaceForManager(defaultNamespace, watchNamespace)
	if isLocal {
		logf.Log.Info("setting up operator to watch local namespace")
		mgrOptions.Namespace = defaultNamespace
	} else {
		mgrOptions.Namespace = ""
		if watchList != nil {
			logf.Log.Info("setting up operator to watch multiple namespaces", "namespace(s)", watchList)
			mgrOptions.NewCache = cache.MultiNamespacedCacheBuilder(watchList)
		} else {
			logf.Log.Info("setting up operator to watch all namespaces")
		}
	}

	if disableMetrics {
		// if we can shutdown metrics port, we don't need disable it.
		mgrOptions.MetricsBindAddress = "0"
	}

	// start our controler
	k8Manager, err := ctrl.NewManager(restConfig, mgrOptions)
	Expect(err).ToNot(HaveOccurred())

	stateManager = common.GetStateManager()

	// Create and start a new auto detect process for this operator
	autodetect, err := common.NewAutoDetect(k8Manager)
	if err != nil {
		logf.Log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		autodetect.DetectOpenshift()
	}

	brokerReconciler = &ActiveMQArtemisReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
	}

	if err = brokerReconciler.SetupWithManager(k8Manager); err != nil {
		logf.Log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisReconciler")
	}

	securityReconciler = &ActiveMQArtemisSecurityReconciler{
		Client:           k8Manager.GetClient(),
		Scheme:           k8Manager.GetScheme(),
		BrokerReconciler: brokerReconciler,
	}

	err = securityReconciler.SetupWithManager(k8Manager)
	Expect(err).ToNot(HaveOccurred(), "failed to create security controller")

	addressReconciler := &ActiveMQArtemisAddressReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
	}

	err = addressReconciler.SetupWithManager(k8Manager)
	Expect(err).ToNot(HaveOccurred(), "failed to create address reconciler")

	scaleDownRconciler := &ActiveMQArtemisScaledownReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
		Config: k8Manager.GetConfig(),
	}

	err = scaleDownRconciler.SetupWithManager(k8Manager)
	Expect(err).ShouldNot(HaveOccurred(), "failed to create scale down reconciler")

	go func() {
		defer GinkgoRecover()
		err = k8Manager.Start(ctx)
		managerChannel <- struct{}{}
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
}

func shutdownControllerManager() {
	cancel()
	<-managerChannel
}

func setUpRealOperator() {
	var err error
	restConfig, err = config.GetConfig()
	Expect(err).ShouldNot(HaveOccurred(), "failed to get the config")

	setUpK8sClient()

	logf.Log.Info("Installing CRDs")
	crds := []string{"../deploy/crds"}
	options := envtest.CRDInstallOptions{
		Paths:              crds,
		ErrorIfPathMissing: false,
	}
	_, err = envtest.InstallCRDs(restConfig, options)
	Expect(err).To(Succeed())

	err = installOperator()
	Expect(err).To(Succeed(), "failed to install operator")
}

//Deploy operator resources
//TODO: provide 'watch all namespaces' option
func installOperator() error {
	logf.Log.Info("#### Installing Operator ####")
	for _, res := range oprRes {
		if err := installYamlResource(res); err != nil {
			return err
		}
	}

	return waitForOperator()
}

func uninstallOperator() error {
	logf.Log.Info("#### Uninstalling Operator ####")
	for _, res := range oprRes {
		if err := uninstallYamlResource(res); err != nil {
			return err
		}
	}

	//uninstall CRDs
	logf.Log.Info("Uninstalling CRDs")
	crds := []string{"../deploy/crds"}
	options := envtest.CRDInstallOptions{
		Paths:              crds,
		ErrorIfPathMissing: false,
	}
	return envtest.UninstallCRDs(restConfig, options)
}

func waitForOperator() error {
	podList := &corev1.PodList{}
	opts := &client.ListOptions{
		Namespace: defaultNamespace,
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
	b, err = ioutil.ReadFile(filepath.Join(filePath, info.Name()))
	if err != nil {
		return nil, nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	robj, gKV, _ := decode(b, nil, nil)
	logf.Log.Info("Loaded resource", "kind", gKV.Kind)
	return robj, gKV, nil
}

func uninstallYamlResource(resPath string) error {
	logf.Log.Info("Uninstalling yaml resource", "yaml", resPath)
	robj, _, err := loadYamlResource(resPath)
	if err != nil {
		return err
	}
	cobj := robj.(client.Object)
	cobj.SetNamespace(defaultNamespace)

	err = k8sClient.Delete(ctx, cobj)
	if err != nil {
		return err
	}

	logf.Log.Info("Successfully uninstalled", "yaml", resPath)
	return nil
}

func installYamlResource(resPath string) error {
	logf.Log.Info("Installing yaml resource", "yaml", resPath)
	robj, gkv, err := loadYamlResource(resPath)
	if err != nil {
		return err
	}
	cobj := robj.(client.Object)
	cobj.SetNamespace(defaultNamespace)

	if oprImg := os.Getenv("OPERATOR_IMAGE"); oprImg != "" {
		if gkv.Kind == "Deployment" {
			logf.Log.Info("Using custom operator image", "url", oprImg)
			oprObj := cobj.(*appsv1.Deployment)
			oprObj.Spec.Template.Spec.Containers[0].Image = oprImg
		}
	}

	err = k8sClient.Create(ctx, cobj)
	if err != nil {
		return err
	}

	//make sure the create ok
	Eventually(func() bool {
		key := types.NamespacedName{Name: cobj.GetName(), Namespace: defaultNamespace}
		err := k8sClient.Get(ctx, key, cobj)
		return err == nil
	}, timeout, interval).Should(Equal(true))

	logf.Log.Info("Successfully installed", "yaml", resPath)
	return nil
}

func setUpK8sClient() {

	logf.Log.Info("Setting up k8s client")

	err := routev1.AddToScheme(scheme.Scheme)
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
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	if verobse {
		GinkgoWriter.TeeTo(os.Stderr)
	}
	// force isLocalOnly=false check from artemis reconciler such that scale down controller will create
	// role binding to service account for the drainer pod
	os.Setenv("OPERATOR_WATCH_NAMESPACE", "SomeValueToCauesEqualitytoFailInIsLocalSoDrainControllerSortsCreds")

	var err error
	currentDir, err = os.Getwd()
	Expect(err).To(Succeed())
	logf.Log.Info("#### Starting test ####", "working dir", currentDir)
	if os.Getenv("DEPLOY_OPERATOR") == "true" {
		logf.Log.Info("#### Setting up a real operator ####")
		setUpRealOperator()
	} else {
		logf.Log.Info("#### Setting up EnvTest ####")
		setUpEnvTest()
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	os.Unsetenv("OPERATOR_WATCH_NAMESPACE")

	if os.Getenv("DEPLOY_OPERATOR") == "true" {
		err := uninstallOperator()
		Expect(err).NotTo(HaveOccurred())
	}

	shutdownControllerManager()

	if stateManager != nil {
		stateManager.Clear()
	}

	if os.Getenv("DEPLOY_OPERATOR") != "true" {
		// scaledown controller lifecycle seems a little loose, it does not complete on signal hander like the others
		for _, drainController := range controllers {
			close(*drainController.GetStopCh())
		}

		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})
