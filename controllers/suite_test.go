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
	"os"

	routev1 "github.com/openshift/api/route/v1"

	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v1alpha1"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha1"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha3"
	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha4"
	brokerv2alpha5 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha5"

	//+kubebuilder:scaffold:imports

	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	ctrl "sigs.k8s.io/controller-runtime"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var stateManager *common.StateManager
var autodetect *common.AutoDetector

var brokerReconciler *ActiveMQArtemisReconciler
var securityReconciler *ActiveMQArtemisSecurityReconciler

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	// force isLocalOnly=false check from artemis reconciler such that scale down controller will create
	// role binding to service account for the drainer pod
	os.Setenv("OPERATOR_WATCH_NAMESPACE", "SomeValueToCauesEqualitytoFailInIsLocalSoDrainControllerSortsCreds")

	// for run in ide
	// os.Setenv("KUBEBUILDER_ASSETS", " .. <path from makefile> /kubebuilder-envtest/k8s/1.22.1-linux-amd64")
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

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

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start our controler
	k8Manager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	stateManager = common.GetStateManager()

	// Create and start a new auto detect process for this operator
	autodetect, err := common.NewAutoDetect(k8Manager)
	if err != nil {
		logf.Log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		autodetect.DetectOpenshift()
	}

	// watch all namespaces by default
	nsoptions.SetWatchAll(true)

	brokerReconciler = &ActiveMQArtemisReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
		Result: ctrl.Result{},
	}

	if err = brokerReconciler.SetupWithManager(k8Manager); err != nil {
		logf.Log.Error(err, "unable to create controller", "controller", "ActiveMQArtemisReconciler")
	}

	securityReconciler = &ActiveMQArtemisSecurityReconciler{
		Client: k8Manager.GetClient(),
		Scheme: k8Manager.GetScheme(),
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
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	os.Unsetenv("OPERATOR_WATCH_NAMESPACE")

	cancel()
	if stateManager != nil {
		stateManager.Clear()
	}
	// scaledown controller lifecycle seems a little loose, it does not complete on signal hander like the others
	for _, drainController := range controllers {
		close(*drainController.GetStopCh())
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
