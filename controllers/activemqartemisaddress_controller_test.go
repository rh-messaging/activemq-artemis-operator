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
// +kubebuilder:docs-gen:collapse=Apache License

/*
As usual, we start with the necessary imports. We also define some utility variables.
*/
package controllers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// To run this test using the following command
// export OPERATOR_IMAGE="<the test operator image>";export DEPLOY_OPERATOR="true";export TEST_ARGS="-ginkgo.focus \"Address controller\" -ginkgo.v"; make -e test-mk
// if OPERATOR_IMAGE is not defined the test will use the latest dev tag
var _ = Describe("Address controller", Label("do"), func() {

	Context("Address test", func() {

		It("Deploy CR with size 5 (pods)", func() {

			ctx := context.Background()

			brokerCrd := generateArtemisSpec(defaultNamespace)

			brokerName := brokerCrd.Name

			brokerCrd.Spec.DeploymentPlan.Size = 5

			brokerCrd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
			}

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" && os.Getenv("DEPLOY_OPERATOR") == "true" {

				Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())
				createdBrokerCrd := &brokerv1beta1.ActiveMQArtemis{}

				By("Waiting for all pods to be started and ready")
				Eventually(func(g Gomega) {

					getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)
					g.Expect(len(createdBrokerCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(5))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("creating 5 queue resources and 1 security")
				addressCrs := make([]*brokerv1beta1.ActiveMQArtemisAddress, 5)
				for i := 0; i < 5; i++ {
					ordinal := strconv.FormatInt(int64(i), 10)
					addressCrs[i] = generateAddressSpec("ex-aaoaddress"+ordinal, defaultNamespace, "myAddress"+ordinal, "myQueue"+ordinal, true, true)
				}

				// This will trigger another issue where some secrets are deleted during pod restart due to security and artemis controller thread
				// contention. Fixing by having security controller fire a reconcile event rather than doing reconcile in line
				propLoginModules := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
				pwd := "geezrick"
				moduleName := "prop-module"
				flag := "sufficient"
				propLoginModules[0] = brokerv1beta1.PropertiesLoginModuleType{
					Name: moduleName,
					Users: []brokerv1beta1.UserType{
						{Name: "morty",
							Password: &pwd,
							Roles:    []string{"admin", "random"}},
					},
				}

				brokerDomainName := "activemq"
				loginModules := make([]brokerv1beta1.LoginModuleReferenceType, 1)
				loginModules[0] = brokerv1beta1.LoginModuleReferenceType{
					Name: &moduleName,
					Flag: &flag,
				}
				brokerDomain := brokerv1beta1.BrokerDomainType{
					Name:         &brokerDomainName,
					LoginModules: loginModules,
				}

				By("Deploying all resources at once")
				_, deployedSecCrd := DeploySecurity("ex-proper", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {
					secCrdToDeploy.Spec.LoginModules.PropertiesLoginModules = propLoginModules
					secCrdToDeploy.Spec.SecurityDomains.BrokerDomain = brokerDomain
				})

				for _, addr := range addressCrs {
					DeployAddress(addr)
				}

				By("Waiting for all pods to be restarted and ready")
				Eventually(func(g Gomega) {

					getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)
					g.Expect(len(createdBrokerCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(5))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Checking all addresses are created on all pods")

				gvk := schema.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				}
				restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme))
				Expect(err).To(BeNil())

				for ipod := 4; ipod >= 0; ipod-- {
					podOrdinal := strconv.FormatInt(int64(ipod), 10)
					podName := namer.CrToSS(brokerCrd.Name) + "-" + podOrdinal

					Eventually(func(g Gomega) {
						execReq := restClient.
							Post().
							Namespace(defaultNamespace).
							Resource("pods").
							Name(podName).
							SubResource("exec").
							VersionedParams(&corev1.PodExecOptions{
								Container: brokerName + "-container",
								Command:   []string{"amq-broker/bin/artemis", "queue", "stat", "--user", "morty", "--password", "geezrick", "--url", "tcp://" + podName + ":61616"},
								Stdin:     true,
								Stdout:    true,
								Stderr:    true,
							}, runtime.NewParameterCodec(scheme.Scheme))

						exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", execReq.URL())

						if err != nil {
							fmt.Printf("error while creating remote command executor: %v", err)
						}
						Expect(err).To(BeNil())
						var capturedOut bytes.Buffer

						err = exec.Stream(remotecommand.StreamOptions{
							Stdin:  os.Stdin,
							Stdout: &capturedOut,
							Stderr: os.Stderr,
							Tty:    false,
						})
						g.Expect(err).To(BeNil())

						By("Checking for output pod")
						g.Expect(capturedOut.Len() > 0)
						content := capturedOut.String()
						g.Expect(content).Should(ContainSubstring("myQueue0"))
						g.Expect(content).Should(ContainSubstring("myQueue1"))
						g.Expect(content).Should(ContainSubstring("myQueue2"))
						g.Expect(content).Should(ContainSubstring("myQueue3"))
						g.Expect(content).Should(ContainSubstring("myQueue4"))
					}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
				}

				//clean up all resources
				Expect(k8sClient.Delete(ctx, createdBrokerCrd)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, deployedSecCrd)).Should(Succeed())
				for _, addr := range addressCrs {
					Expect(k8sClient.Delete(ctx, addr)).Should((Succeed()))
				}
			}
		})
	})

	Context("Address delete and scale down", func() {

		It("Scale down, verify RemoveFromBrokerOnDelete", func() {

			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
			}
			crd.Spec.DeploymentPlan.Size = 2

			By("By deploying address cr for a2 for this broker in advance")

			addressName := "A2"
			addressCrd := brokerv1beta1.ActiveMQArtemisAddress{}
			addressCrd.SetName("address-" + randString())
			addressCrd.SetNamespace(defaultNamespace)
			addressCrd.Spec.AddressName = addressName // note no queue
			routingTypeShouldBeOptional := "multicast"
			addressCrd.Spec.RoutingType = &routingTypeShouldBeOptional
			addressCrd.Spec.RemoveFromBrokerOnDelete = true

			Expect(k8sClient.Create(ctx, &addressCrd)).Should(Succeed())

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" && os.Getenv("DEPLOY_OPERATOR") == "true" {

				By("Deploying a broker pair")
				Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

				By("Checking ready on SS")
				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
					sfsFound := &appsv1.StatefulSet{}

					g.Expect(k8sClient.Get(ctx, key, sfsFound)).Should(Succeed())
					g.Expect(sfsFound.Status.ReadyReplicas).Should(BeEquivalentTo(2))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Verfying address is present")
				ordinals := []string{"0", "1"}
				for _, ordinal := range ordinals {

					podWithOrdinal := namer.CrToSS(crd.Name) + "-" + ordinal
					command := []string{"amq-broker/bin/artemis", "address", "show", "--url", "tcp://" + podWithOrdinal + ":61616"}

					Eventually(func(g Gomega) {
						stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, gomega.Default)
						g.Expect(stdOutContent).Should(ContainSubstring(addressName))
					}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
				}

				By("Deleting address CR")
				Expect(k8sClient.Delete(ctx, &addressCrd)).Should(Succeed())

				By("Verifying address gone from both brokers")
				for _, ordinal := range ordinals {
					podWithOrdinal := namer.CrToSS(crd.Name) + "-" + ordinal
					command := []string{"amq-broker/bin/artemis", "address", "show", "--url", "tcp://" + podWithOrdinal + ":61616"}

					Eventually(func(g Gomega) {
						stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, gomega.Default)
						g.Expect(stdOutContent).ShouldNot(ContainSubstring(addressName))
					}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
				}
			}

			// cleanup
			k8sClient.Delete(ctx, &addressCrd)
			k8sClient.Delete(ctx, &crd)
		})
	})
})

func execOnPod(podWithOrdinal string, brokerName string, namespace string, command []string, g Gomega) string {

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme))
	g.Expect(err).To(BeNil())

	execReq := restClient.
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podWithOrdinal).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: brokerName + "-container",
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(scheme.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", execReq.URL())
	g.Expect(err).To(BeNil())

	var outPutbuffer bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &outPutbuffer,
		Stderr: os.Stderr,
		Tty:    false,
	})
	g.Expect(err).To(BeNil())

	g.Eventually(func(g Gomega) {
		By("Checking for output from exec")
		g.Expect(outPutbuffer.Len() > 0)
	}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

	return outPutbuffer.String()
}

func generateAddressSpec(name string, ns string, address string, queue string, isMulticast bool, autoDelete bool) *brokerv1beta1.ActiveMQArtemisAddress {

	spec := brokerv1beta1.ActiveMQArtemisAddressSpec{}

	spec.AddressName = address
	spec.QueueName = &queue

	routingType := "anycast"
	if isMulticast {
		routingType = "multicast"
	}
	spec.RoutingType = &routingType

	toCreate := &brokerv1beta1.ActiveMQArtemisAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemisAddress",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: spec,
	}

	return toCreate
}

func DeployAddress(candidate *brokerv1beta1.ActiveMQArtemisAddress) {
	ctx := context.Background()

	Expect(k8sClient.Create(ctx, candidate)).Should(Succeed())

	createdAddressCrd := &brokerv1beta1.ActiveMQArtemisAddress{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(candidate.ObjectMeta.Name, candidate.Namespace, createdAddressCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdAddressCrd.Name).Should(Equal(candidate.ObjectMeta.Name))
}
