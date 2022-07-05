/*
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

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("tests regarding controller manager", func() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("operator namespaces test", func() {

		It("test resolving watching namespace", Label("test-resolving-namespace"), func() {

			operatorNamespace := "default"
			isLocal, watchList := common.ResolveWatchNamespaceForManager(operatorNamespace, operatorNamespace)
			Expect(isLocal).To(BeTrue())
			Expect(watchList).To(BeNil())

			for _, wn := range []string{"", "*"} {
				isLocal, watchList = common.ResolveWatchNamespaceForManager(operatorNamespace, wn)
				Expect(isLocal).To(BeFalse())
				Expect(watchList).To(BeNil())
			}

			isLocal, watchList = common.ResolveWatchNamespaceForManager(operatorNamespace, "namespace1,namespace2")
			Expect(isLocal).To(BeFalse())
			Expect(len(watchList)).To(Equal(2))
			Expect(watchList[0]).To(Equal("namespace1"))
			Expect(watchList[1]).To(Equal("namespace2"))
		})

		It("test watching single(local) namespace", Label("test-watching-namespace"), func() {
			testWatchNamespace("single", false, func() {
				By("deploying broker in to target namespace")
				cr, createdCr := DeployCustomBroker("", defaultNamespace, nil)

				By("check statefulset get created")
				createdSs := &appsv1.StatefulSet{}
				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(cr.Name), Namespace: defaultNamespace}
					err := k8sClient.Get(ctx, key, createdSs)
					g.Expect(err).To(Succeed(), "expect to get ss for cr "+cr.Name)
				}, timeout, interval).Should(Succeed())

				By("deploying broker in " + namespace1)
				cr1, createdCr1 := DeployCustomBroker("", namespace1, nil)

				By("check statefulset should not be created")
				createdSs1 := &appsv1.StatefulSet{}
				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(cr1.Name), Namespace: namespace1}
					err := k8sClient.Get(ctx, key, createdSs1)
					g.Expect(err).NotTo(Succeed(), "no ss should be created for cr "+cr1.Name+" in namespace "+namespace1)
				}, timeout, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(cr1.Name), Namespace: defaultNamespace}
					err := k8sClient.Get(ctx, key, createdSs1)
					g.Expect(err).NotTo(Succeed(), "no ss should be created for cr "+cr1.Name+" in namespace "+defaultNamespace)
				}, timeout, interval).Should(Succeed())

				DeleteCr(createdCr, cr.Name, defaultNamespace)
				DeleteCr(createdCr1, cr1.Name, namespace1)
			})
		})

		It("test watching all namespaces", Label("test-watching-namespace"), func() {
			testWatchNamespace("all", false, func() {
				By("deploying broker in to all namespaces")
				var createdCrs []*brokerv1beta1.ActiveMQArtemis
				for _, ns := range []string{defaultNamespace, namespace1, namespace2, namespace3} {
					_, createdCr := DeployCustomBroker("", ns, nil)
					createdCrs = append(createdCrs, createdCr)
				}

				By("check statefulset get created in each namespace")
				for _, createdCr := range createdCrs {
					createdSs := &appsv1.StatefulSet{}
					key := types.NamespacedName{Name: namer.CrToSS(createdCr.Name), Namespace: createdCr.Namespace}
					Eventually(func(g Gomega) {
						err := k8sClient.Get(ctx, key, createdSs)
						g.Expect(err).To(Succeed(), "expect to get ss "+key.Name+" in namespace "+key.Namespace)
					}, timeout, interval).Should(Succeed())
				}

				By("clean up")
				for _, createdCr := range createdCrs {
					DeleteCr(createdCr, createdCr.Name, createdCr.Namespace)
				}
			})
		})

		It("test watching multiple namespaces", Label("test-watching-namespace"), func() {
			testWatchNamespace("multiple", true, func() {
				//only namespace2 and namespace3 is watched
				By("deploying broker in to all namespaces")
				var createdCrs []*brokerv1beta1.ActiveMQArtemis
				for _, ns := range []string{defaultNamespace, namespace1, namespace2, namespace3} {
					_, createdCr := DeployCustomBroker("", ns, nil)
					createdCrs = append(createdCrs, createdCr)
				}

				By("check statefulset get created only in " + namespace2 + " and " + namespace3)
				for _, createdCr := range createdCrs {
					createdSs := &appsv1.StatefulSet{}
					key := types.NamespacedName{Name: namer.CrToSS(createdCr.Name), Namespace: createdCr.Namespace}
					if createdCr.Name == namespace2 || createdCr.Name == namespace3 {
						Eventually(func(g Gomega) {
							err := k8sClient.Get(ctx, key, createdSs)
							g.Expect(err).To(Succeed(), "expect to get ss "+key.Name+" in namespace "+key.Namespace)
						}, timeout, interval).Should(Succeed())
					} else {
						Eventually(func() bool {
							err := k8sClient.Get(ctx, key, createdSs)
							return errors.IsNotFound(err)
						}, timeout, interval).Should(BeTrue(), "statefulset shouldn't be in namespace "+createdCr.Namespace)
					}
				}

				By("clean up")
				for _, createdCr := range createdCrs {
					DeleteCr(createdCr, createdCr.Name, createdCr.Namespace)
				}
			})
		})
	})
})

func DeleteCr(cr client.Object, name string, targetNs string) {
	ctx := context.Background()
	Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())

	Eventually(func() bool {
		return checkCrdDeleted(name, targetNs, cr)
	}, timeout, interval).Should(BeTrue())

}

func createNamespace(namespace string) {
	ns := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	Eventually(func() bool {
		err := k8sClient.Create(ctx, &ns, &client.CreateOptions{})
		return err == nil || errors.IsAlreadyExists(err)
	}, timeout, interval).Should(BeTrue())
}

func testWatchNamespace(kind string, last bool, testFunc func()) {

	shutdownControllerManager()

	if kind == "single" {
		createControllerManager(true, defaultNamespace)
	} else if kind == "all" {
		createControllerManager(true, "")
	} else {
		createControllerManager(true, namespace2+","+namespace3)
	}

	createNamespace(namespace1)
	createNamespace(namespace2)
	createNamespace(namespace3)

	testFunc()

	if last {
		shutdownControllerManager()
		createControllerManager(true, defaultNamespace)
	}
}
