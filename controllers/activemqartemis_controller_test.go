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
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	corev1 "k8s.io/api/core/v1"
)

// Uncomment this and the "test" import if you want to debug this set of tests
//func TestArtemisController(t *testing.T) {
// 	RegisterFailHandler(Fail)
// 	RunSpecs(t, "Artemis Controller Suite")
// }

var _ = Describe("artemis controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		namespace = "default"
		timeout   = time.Second * 15
		duration  = time.Second * 10
		interval  = time.Millisecond * 250
		verobse   = false
	)

	// see what has changed from the controllers perspective
	var wi watch.Interface
	BeforeEach(func() {

		wc, _ := client.NewWithWatch(testEnv.Config, client.Options{})

		// see what changed
		var err error
		wi, err = wc.Watch(ctx, &brokerv1beta1.ActiveMQArtemisList{}, &client.ListOptions{})
		if err != nil {
			fmt.Printf("Err on watch:  %v\n", err)
		}
		go func() {
			for event := range wi.ResultChan() {
				switch co := event.Object.(type) {
				case client.Object:
					if verobse {
						fmt.Printf("%v ActiveMQArtemisList CRD: ResourceVersion: %v Generation: %v, OR: %v\n", event.Type, co.GetResourceVersion(), co.GetGeneration(), co.GetOwnerReferences())
						fmt.Printf("Object: %v\n", event.Object)
					}
				}
			}
		}()
	})

	AfterEach(func() {
		if wi != nil {
			wi.Stop()
		}
	})

	Context("Tolerations Test", func() {
		It("passing in 2 tolerations", func() {

			By("Creating a crd with 2 tolerations")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.DeploymentPlan.Tolerations = []corev1.Toleration{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: "NoSchedule",
				},
				{
					Key:    "yes",
					Value:  "No",
					Effect: "NoSchedule",
				},
			}

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the tolerations " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return len(createdSs.Spec.Template.Spec.Tolerations) == 2
			}, timeout, interval).Should(Equal(true))
			Expect(len(createdSs.Spec.Template.Spec.Tolerations) == 2).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Key == "foo").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Value == "bar").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Effect == "NoSchedule").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[1].Key == "yes").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[1].Value == "No").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[1].Effect == "NoSchedule").Should(BeTrue())

			By("Redeploying the CRD with different Tolerations")

			Eventually(func() bool {

				// fetch, modify and update (we compete with the status updates)

				ok := getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
				if !ok {
					return false
				}

				createdCrd.Spec.DeploymentPlan.Tolerations = []corev1.Toleration{
					{
						Key:    "yes",
						Value:  "No",
						Effect: "NoSchedule",
					},
				}

				err := k8sClient.Update(ctx, createdCrd)
				return err == nil
			}, timeout, interval).Should(Equal(true))

			By("and checking there is just a single Toleration")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return len(createdSs.Spec.Template.Spec.Tolerations) == 1
			}, timeout, interval).Should(Equal(true))
			Expect(len(createdSs.Spec.Template.Spec.Tolerations) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Key == "yes").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Value == "No").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Effect == "NoSchedule").Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("Liveness Probe Tests", func() {
		It("Override Liveness Probe No Exec", func() {
			By("By creating a crd with Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			livenessProbe := corev1.Probe{}
			livenessProbe.PeriodSeconds = 5
			livenessProbe.InitialDelaySeconds = 6
			livenessProbe.TimeoutSeconds = 7
			livenessProbe.SuccessThreshold = 8
			livenessProbe.FailureThreshold = 9
			crd.Spec.DeploymentPlan.LivenessProbe = livenessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Liveness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.TCPSocket.Port.String() == "8161").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 5).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds == 6).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds == 7).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold == 8).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == 9).Should(BeTrue())

			By("Checking that Stateful Set is updated with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			By("Updating the CR")
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd) }, timeout, interval).Should(BeTrue())
			original := createdCrd

			original.Spec.DeploymentPlan.LivenessProbe.PeriodSeconds = 15
			original.Spec.DeploymentPlan.LivenessProbe.InitialDelaySeconds = 16
			original.Spec.DeploymentPlan.LivenessProbe.TimeoutSeconds = 17
			original.Spec.DeploymentPlan.LivenessProbe.SuccessThreshold = 18
			original.Spec.DeploymentPlan.LivenessProbe.FailureThreshold = 19
			exec := corev1.ExecAction{
				Command: []string{"/broker/bin/artemis check node"},
			}
			original.Spec.DeploymentPlan.LivenessProbe.Exec = &exec
			By("Redeploying the modified CRD")
			Expect(k8sClient.Update(ctx, original)).Should(Succeed())

			By("Retrieving the new SS to find the modification")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 15
			}, timeout, interval).Should(Equal(true))

			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.Exec != nil).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 15).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds == 16).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds == 17).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold == 18).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == 19).Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool { return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd) }, timeout, interval).Should(BeTrue())
		})

		It("Override Liveness Probe Exec", func() {
			By("By creating a crd with Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			exec := corev1.ExecAction{
				Command: []string{"/broker/bin/artemis check node"},
			}
			livenessProbe := corev1.Probe{}
			livenessProbe.Exec = &exec
			crd.Spec.DeploymentPlan.LivenessProbe = livenessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Liveness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Default Liveness Probe", func() {
			By("By creating a crd without Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that the Liveness Probe is created")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.TCPSocket.Port.String() == "8161").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Readiness Probe Tests", func() {
		It("Override Readiness Probe No Exec", func() {
			By("By creating a crd with Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			readinessProbe := corev1.Probe{}
			readinessProbe.PeriodSeconds = 5
			readinessProbe.InitialDelaySeconds = 6
			readinessProbe.TimeoutSeconds = 7
			readinessProbe.SuccessThreshold = 8
			readinessProbe.FailureThreshold = 9
			crd.Spec.DeploymentPlan.ReadinessProbe = readinessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Readiness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[0] == "/bin/bash").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[1] == "-c").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[2] == "/opt/amq/bin/readinessProbe.sh").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Override Readiness Probe Exec", func() {
			By("By creating a crd with Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			exec := corev1.ExecAction{
				Command: []string{"/broker/bin/artemis check node"},
			}
			readinessProbe := corev1.Probe{}
			readinessProbe.Exec = &exec
			crd.Spec.DeploymentPlan.ReadinessProbe = readinessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Readiness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Default Readiness Probe", func() {
			By("By creating a crd without Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that the Readiness Probe is created")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[0] == "/bin/bash").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[1] == "-c").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.Exec.Command[2] == "/opt/amq/bin/readinessProbe.sh").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Status", func() {
		It("Expect pod desc", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			// would like more status updates on createdCrd

			By("By checking the status of stateful set")
			Eventually(func() (int, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return -1, err
				}

				// presence is good enough... check on this status just for kicks
				return int(createdSs.Status.Replicas), err
			}, duration, interval).Should(Equal(0))

			By("Checking stopped status of CR because we expect it to fail to deploy")
			Eventually(func() (int, error) {
				key := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: namespace}
				err := k8sClient.Get(ctx, key, createdCrd)

				if err != nil {
					return -1, err
				}

				return len(createdCrd.Status.PodStatus.Stopped), nil
			}, timeout, interval).Should(Equal(1))
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("BrokerProperties", func() {
		It("Expect vol mount via config map", func() {
			By("By creating a new crd with BrokerProperties in the spec")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)

			crd.Spec.BrokerProperties = map[string]string{
				"globalMaxSize": "512m",
			}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, namespace, createdCrd) }, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			hexShaOriginal := HexShaHashOfMap(crd.Spec.BrokerProperties)

			By("By finding a new config map with broker props")
			configMap := &corev1.ConfigMap{}
			key := types.NamespacedName{Name: "broker-properties-" + hexShaOriginal, Namespace: crd.ObjectMeta.Namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, configMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By checking the container stateful set for java opts")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.InitContainers {
					for _, env := range container.Env {
						if env.Name == "JAVA_OPTS" {
							if strings.Contains(env.Value, "broker.properties") {
								found = true
							}
						}
					}
				}

				return found, err
			}, duration, interval).Should(Equal(true))

			By("By checking the stateful set for volume mount path")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: namespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.Containers {
					for _, vm := range container.VolumeMounts {
						// mount path can't have a .
						if strings.Contains(vm.MountPath, "broker-properties") {
							found = true
						}
					}
				}

				return found, err
			}, duration, interval).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})

		It("Expect new config map on update to BrokerProperties", func() {
			By("By creating a crd with BrokerProperties in the spec")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)

			crd.Spec.BrokerProperties = map[string]string{
				"globalMaxSize": "64g",
			}
			hexShaOriginal := HexShaHashOfMap(crd.Spec.BrokerProperties)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("By eventualy finding a matching config map with broker props")
			configMapList := &corev1.ConfigMapList{}
			opts := &client.ListOptions{
				Namespace: namespace,
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, configMapList, opts)
				if err != nil {
					fmt.Printf("error getting list of configopts map! %v", err)
				}
				for _, cm := range configMapList.Items {
					if strings.Contains(cm.ObjectMeta.Name, hexShaOriginal) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("updating the crd, expect new ConfigMap name")
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			By("pushing the update on the current version...")
			Eventually(func() bool {

				err := k8sClient.Get(ctx, types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: crd.ObjectMeta.Namespace}, createdCrd)
				if err == nil {

					// add a new property
					createdCrd.Spec.BrokerProperties["gen"] = strconv.FormatInt(createdCrd.ObjectMeta.Generation, 10)

					err = k8sClient.Update(ctx, createdCrd)
					if err != nil {
						fmt.Printf("error on update! %v\n", err)
					}
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			hexShaModified := HexShaHashOfMap(createdCrd.Spec.BrokerProperties)

			By("finding the updated config map using the sha")
			Eventually(func() bool {
				err := k8sClient.List(ctx, configMapList, opts)

				if err == nil && len(configMapList.Items) > 0 {

					for _, cm := range configMapList.Items {
						if strings.Contains(cm.ObjectMeta.Name, hexShaModified) {
							return true
						}
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// cleanup
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, namespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

			By("verifying no config maps leaked")
			Eventually(func() bool {
				k8sClient.List(ctx, configMapList, opts)
				return len(configMapList.Items) == 0
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("With delopyed controller", func() {
		It("Checking acceptor service while expose is false", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(namespace)

			crd.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:   "new-acceptor",
					Port:   61616,
					Expose: false,
				},
			}
			crd.Spec.Connectors = []brokerv1beta1.ConnectorType{
				{
					Name:   "new-connector",
					Port:   61616,
					Expose: false,
				},
			}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: namespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-acceptor-0-svc", Namespace: namespace}
				acceptorService := &corev1.Service{}
				err := k8sClient.Get(context.Background(), key, acceptorService)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-connector-0-svc", Namespace: namespace}
				connectorService := &corev1.Service{}
				err := k8sClient.Get(context.Background(), key, connectorService)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, namespace, &crd), timeout, interval).Should(BeTrue())
		})
	})
})

func generateArtemisSpec(namespace string) brokerv1beta1.ActiveMQArtemis {

	spec := brokerv1beta1.ActiveMQArtemisSpec{}

	toCreate := brokerv1beta1.ActiveMQArtemis{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      randString(),
			Namespace: namespace,
		},
		Spec: spec,
	}

	return toCreate
}

func generateOriginalArtemisSpec(namespace string, name string) *brokerv1beta1.ActiveMQArtemis {

	spec := brokerv1beta1.ActiveMQArtemisSpec{}

	toCreate := brokerv1beta1.ActiveMQArtemis{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	return &toCreate
}

func randString() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	length := 6
	var b strings.Builder
	b.WriteString("broker-")
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func getPersistedVersionedCrd(name string, nameSpace string, crd *brokerv1beta1.ActiveMQArtemis) bool {
	key := types.NamespacedName{Name: name, Namespace: nameSpace}
	err := k8sClient.Get(ctx, key, crd)
	return err == nil
}

func checkCrdDeleted(name string, namespace string, crd *brokerv1beta1.ActiveMQArtemis) bool {
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, crd)
	return errors.IsNotFound(err)
}
