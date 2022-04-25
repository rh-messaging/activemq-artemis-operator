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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
)

var _ = Describe("Scale down controller", func() {

	const (
		namespace               = "default"
		existingClusterTimeout  = time.Second * 180
		existingClusterInterval = time.Second * 10
		verobse                 = false
	)

	Context("Scale down test", func() {
		It("deploy plan 2 with user/pass", func() {

			ctx := context.Background()

			brokerName := randString()

			brokerCrd := generateOriginalArtemisSpec(defaultNamespace, brokerName)

			clustered := true
			brokerCrd.Spec.DeploymentPlan.Clustered = &clustered
			brokerCrd.Spec.DeploymentPlan.Size = 2
			brokerCrd.Spec.DeploymentPlan.PersistenceEnabled = true
			brokerCrd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
			}
			Expect(k8sClient.Create(ctx, brokerCrd)).Should(Succeed())

			createdBrokerCrd := &brokerv1beta1.ActiveMQArtemis{}
			getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("verying two started")
				Eventually(func(g Gomega) {

					getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)
					g.Expect(len(createdBrokerCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(2))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Sending a message to 1")

				podWithOrdinal := namer.CrToSS(brokerCrd.Name) + "-1"

				gvk := schema.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				}
				restClient, err := apiutil.RESTClientForGVK(gvk, false, testEnv.Config, serializer.NewCodecFactory(testEnv.Scheme))
				Expect(err).To(BeNil())

				execReq := restClient.
					Post().
					Namespace(defaultNamespace).
					Resource("pods").
					Name(podWithOrdinal).
					SubResource("exec").
					VersionedParams(&corev1.PodExecOptions{
						Container: brokerName + "-container",
						Command:   []string{"amq-broker/bin/artemis", "producer", "--user", "Jay", "--password", "activemq", "--url", "tcp://" + podWithOrdinal + ":61616", "--message-count", "1"},
						Stdin:     true,
						Stdout:    true,
						Stderr:    true,
					}, runtime.NewParameterCodec(testEnv.Scheme))

				exec, err := remotecommand.NewSPDYExecutor(testEnv.Config, "POST", execReq.URL())

				if err != nil {
					fmt.Printf("error while creating remote command executor: %v", err)
				}
				Expect(err).To(BeNil())

				var producerCapturedOut bytes.Buffer

				err = exec.Stream(remotecommand.StreamOptions{
					Stdin:  os.Stdin,
					Stdout: &producerCapturedOut,
					Stderr: os.Stderr,
					Tty:    false,
				})
				Expect(err).To(BeNil())

				Eventually(func(g Gomega) {
					By("Checking for output from producer")
					g.Expect(producerCapturedOut.Len() > 0)
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				content := producerCapturedOut.String()
				Expect(content).Should(ContainSubstring("Produced: 1 messages"))

				By("Scaling down to 0")
				Eventually(func(g Gomega) {

					getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)
					createdBrokerCrd.Spec.DeploymentPlan.Size = 1
					k8sClient.Update(ctx, createdBrokerCrd)
					//		g.Expect(len(createdBrokerCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
					By("Scale down to 0 update complete")
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Checking ready 1")
				Eventually(func(g Gomega) {
					getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCrd)
					g.Expect(len(createdBrokerCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Receiving a message from 0")
				podWithOrdinal = namer.CrToSS(brokerCrd.Name) + "-0"
				execReq = restClient.
					Post().
					Namespace(defaultNamespace).
					Resource("pods").
					Name(podWithOrdinal).
					SubResource("exec").
					VersionedParams(&corev1.PodExecOptions{
						Container: brokerName + "-container",
						Command:   []string{"amq-broker/bin/artemis", "consumer", "--user", "Jay", "--password", "activemq", "--url", "tcp://" + podWithOrdinal + ":61616", "--message-count", "1", "--receive-timeout", "10000", "--break-on-null"},
						Stdin:     true,
						Stdout:    true,
						Stderr:    true,
					}, runtime.NewParameterCodec(testEnv.Scheme))

				exec, err = remotecommand.NewSPDYExecutor(testEnv.Config, "POST", execReq.URL())

				if err != nil {
					fmt.Printf("error while creating remote command executor for consume: %v", err)
				}
				Expect(err).To(BeNil())

				var consumerCapturedOut bytes.Buffer

				err = exec.Stream(remotecommand.StreamOptions{
					Stdin:  os.Stdin,
					Stdout: &consumerCapturedOut,
					Stderr: os.Stderr,
					Tty:    false,
				})
				Expect(err).To(BeNil())

				Eventually(func(g Gomega) {
					By("Checking for output from consumer")
					g.Expect(consumerCapturedOut.Len() > 0)
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				content = consumerCapturedOut.String()

				Expect(content).Should(ContainSubstring("Consumed: 1 messages"))
			}

			Expect(k8sClient.Delete(ctx, createdBrokerCrd)).Should(Succeed())

		})
	})
})
