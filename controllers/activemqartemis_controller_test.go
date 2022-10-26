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
	"container/list"
	"context"
	"fmt"

	"net"
	"net/url"
	"os"

	"github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/api/v2alpha4"
	"github.com/artemiscloud/activemq-artemis-operator/api/v2alpha5"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/pods"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"math/rand"
	"reflect"
	"strconv"
	"strings"

	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"

	"github.com/Azure/go-amqp"
	netv1 "k8s.io/api/networking/v1"
)

//Uncomment this and the "test" import if you want to debug this set of tests
//func TestArtemisController(t *testing.T) {
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Artemis Controller Suite")
//}

var _ = Describe("artemis controller", func() {

	// see what has changed from the controllers perspective, what we watch
	toWatch := []client.ObjectList{&brokerv1beta1.ActiveMQArtemisList{}, &appsv1.StatefulSetList{}, &corev1.PodList{}}
	wis := list.New()
	BeforeEach(func() {

		if verobse {
			fmt.Println("Time with MicroSeconds: ", time.Now().Format("2006-01-02 15:04:05.000000"), " test:", CurrentGinkgoTestDescription())
		}

		for _, li := range toWatch {

			wc, err := client.NewWithWatch(testEnv.Config, client.Options{})
			if err != nil {
				fmt.Printf("Err on watch client:  %v\n", err)
				return
			}

			// see what changed
			wi, err := wc.Watch(ctx, li, &client.ListOptions{})
			if err != nil {
				fmt.Printf("Err on watch:  %v\n", err)
			}
			wis.PushBack(wi)

			go func() {
				for event := range wi.ResultChan() {
					switch co := event.Object.(type) {
					case client.Object:
						if verobse {
							fmt.Printf("%v : ResourceVersion: %v Generation: %v, OR: %v\n", event.Type, co.GetResourceVersion(), co.GetGeneration(), co.GetOwnerReferences())
							fmt.Printf("%v : Object: %v\n", event.Type, event.Object)
						}
					default:
						if verobse {
							fmt.Printf("%v : type: %v\n", event.Type, co)
							fmt.Printf("%v : Object: %v\n", event.Type, event.Object)
						}
					}
				}
			}()
		}
	})

	AfterEach(func() {
		for e := wis.Front(); e != nil; e = e.Next() {
			e.Value.(watch.Interface).Stop()
		}
	})

	Context("New address settings options", func() {
		It("Deploy broker with new address settings", func() {

			By("By creating a crd with address settings in spec")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			// add address settings, to an existing crd
			ma := "merge_all"
			dlqabc := "dlqabc"
			maxSize := "10m"
			maxMessages := int64(5000)
			redistributionDelay := int32(5000)
			configDeleteDiverts := "OFF"

			crd.Spec.AddressSettings = brokerv1beta1.AddressSettingsType{
				ApplyRule: &ma,
				AddressSetting: []brokerv1beta1.AddressSettingType{
					{
						Match:               "abc#",
						DeadLetterAddress:   &dlqabc,
						MaxSizeBytes:        &maxSize,
						MaxSizeMessages:     &maxMessages,
						ConfigDeleteDiverts: &configDeleteDiverts,
						RedistributionDelay: &redistributionDelay,
					},
					{
						Match:               "#",
						DeadLetterAddress:   &dlqabc,
						MaxSizeBytes:        &maxSize,
						MaxSizeMessages:     &maxMessages,
						ConfigDeleteDiverts: &configDeleteDiverts,
						RedistributionDelay: &redistributionDelay,
					},
				},
			}
			crd.Spec.DeploymentPlan.Size = 1

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

			By("tracking the yaconfig init command with user_address_settings and verifying new options are in")
			key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
			var initArgsString string
			Eventually(func() bool {
				createdSs := &appsv1.StatefulSet{}

				if k8sClient.Get(ctx, key, createdSs) != nil {
					return false
				}

				initArgsString = strings.Join(createdSs.Spec.Template.Spec.InitContainers[0].Args, ",")
				if !strings.Contains(initArgsString, "max_size_messages: 5000") {
					return false
				}

				value := cr2jinja2.GetUniqueShellSafeSubstution(configDeleteDiverts)
				fullString := "config_delete_diverts: " + value
				return strings.Contains(initArgsString, fullString)

			}, timeout, interval).Should(BeTrue())

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("verifying started")
				brokerKey := types.NamespacedName{Name: createdCrd.Name, Namespace: createdCrd.Namespace}
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}

			// cleanup
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("Versions Test", func() {
		It("default image to use latest", func() {
			crd := generateArtemisSpec(defaultNamespace)
			imageToUse := determineImageToUse(&crd, "Kubernetes")
			Expect(imageToUse).To(Equal(version.LatestKubeImage), "actual", imageToUse)

			imageToUse = determineImageToUse(&crd, "Init")
			Expect(imageToUse).To(Equal(version.LatestInitImage), "actual", imageToUse)
			brokerCr := generateArtemisSpec(defaultNamespace)
			compactVersionToUse := determineCompactVersionToUse(&brokerCr)
			yacfgProfileVersion = version.YacfgProfileVersionFromFullVersion[version.FullVersionFromCompactVersion[compactVersionToUse]]
			Expect(yacfgProfileVersion).To(Equal("7.10.0"))
		})
	})

	Context("CrVersionConversionTest", func() {
		It("can reconcile different version", func() {

			var float32Var = float32(2.3)
			var ma = "all"
			toCreate := v2alpha5.ActiveMQArtemis{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ActiveMQArtemis",
					APIVersion: v2alpha5.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      randString(),
					Namespace: defaultNamespace,
				},
				Spec: v2alpha5.ActiveMQArtemisSpec{
					AddressSettings: v2alpha5.AddressSettingsType{
						ApplyRule: &ma,
						AddressSetting: []v2alpha5.AddressSettingType{
							{
								Match:                              "#",
								RedeliveryDelayMultiplier:          &float32Var,
								RedeliveryCollisionAvoidanceFactor: &float32Var,
							},
						},
					},
				},
			}

			By("deploying v2lpha5 with float32, ignored")
			Expect(k8sClient.Create(context.TODO(), &toCreate)).Should(Succeed())

			key := types.NamespacedName{Name: namer.CrToSS(toCreate.Name), Namespace: defaultNamespace}
			createdSs := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, createdSs)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			By("checking status - using original version - exercise in marshalling")
			key = types.NamespacedName{Name: toCreate.Name, Namespace: toCreate.Namespace}
			createdCrdv2alpha5 := &v2alpha5.ActiveMQArtemis{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, createdCrdv2alpha5)).Should(Succeed())
				g.Expect(len(createdCrdv2alpha5.Status.PodStatus.Stopped)).Should(BeEquivalentTo(1))
			}, timeout, interval).Should(Succeed())

			By("checking status - using served version - with conditions")
			key = types.NamespacedName{Name: toCreate.Name, Namespace: toCreate.Namespace}
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, createdCrd)).Should(Succeed())
				g.Expect(len(createdCrd.Status.PodStatus.Stopped)).Should(BeEquivalentTo(1))
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
		})

		It("can reconcile latest version with config in brokerProperties", func() {

			toCreate := brokerv1beta1.ActiveMQArtemis{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ActiveMQArtemis",
					APIVersion: brokerv1beta1.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      randString(),
					Namespace: defaultNamespace,
				},
				Spec: brokerv1beta1.ActiveMQArtemisSpec{
					BrokerProperties: []string{
						"addressesSettings.#.redeliveryDelayMultiplier=2.3",
						"addressesSettings.#.redeliveryCollisionAvoidanceFactor=1.2",
					},
				},
			}

			Expect(k8sClient.Create(context.TODO(), &toCreate)).Should(Succeed())

			key := types.NamespacedName{Name: namer.CrToSS(toCreate.Name), Namespace: defaultNamespace}
			createdSs := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, createdSs)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			key = types.NamespacedName{Name: toCreate.Name, Namespace: toCreate.Namespace}
			createdCrd := &v1beta1.ActiveMQArtemis{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, createdCrd)).Should(Succeed())
				g.Expect(len(createdCrd.Status.PodStatus.Stopped)).Should(BeEquivalentTo(1))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

		})

		It("testing broker image resolve", Label("amq-versions-test"), func() {
			manager := "../deploy/operator.yaml"
			robj, _, err := loadYamlResource(manager)
			Expect(err).To(Succeed())
			oprObj := robj.(*appsv1.Deployment)

			envs := oprObj.Spec.Template.Spec.Containers[0].Env
			for _, entry := range envs {
				os.Setenv(entry.Name, entry.Value)
			}
			defer func() {
				for _, entry := range envs {
					os.Unsetenv(entry.Name)
				}
			}()

			latestBrokerImageEnvKey := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_Kubernetes_" + version.CompactLatestVersion
			latestBrokerInitImageEnvKey := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_Init_" + version.CompactLatestVersion

			latestBrokerImage := os.Getenv(latestBrokerImageEnvKey)
			latestInitImage := os.Getenv(latestBrokerInitImageEnvKey)

			By("checking default images are always the latest defined in env")
			brokerCrd := generateArtemisSpec(defaultNamespace)

			imageToUse := determineImageToUse(&brokerCrd, "Kubernetes")
			initImageToUse := determineImageToUse(&brokerCrd, "Init")
			Expect(imageToUse).To(Equal(latestBrokerImage))
			Expect(initImageToUse).To(Equal(latestInitImage))

			brokerCrd.Spec.Version = "7.9.4"
			brokerCrd.Spec.Upgrades.Enabled = true

			brokerImageEnvKey := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_Kubernetes_" + version.CompactVersionFromVersion["7.9.4"]
			initImageEnvKey := "RELATED_IMAGE_ActiveMQ_Artemis_Broker_Init_" + version.CompactVersionFromVersion["7.9.4"]

			expectedImage := os.Getenv(brokerImageEnvKey)
			expectedInitImage := os.Getenv(initImageEnvKey)

			imageToUse = determineImageToUse(&brokerCrd, "Kubernetes")
			initImageToUse = determineImageToUse(&brokerCrd, "Init")

			Expect(imageToUse).To(Equal(expectedImage))
			Expect(initImageToUse).To(Equal(expectedInitImage))
		})
	})

	Context("ClientId autoshard Test", func() {

		produceMessage := func(url string, clientId string, linkAddress string, messageId int, g Gomega) {

			client, err := amqp.Dial(url, amqp.ConnContainerID(clientId), amqp.ConnSASLPlain("dummy-user", "dummy-pass"))

			g.Expect(err).Should(BeNil())
			g.Expect(client).ShouldNot(BeNil())
			defer client.Close()

			session, err := client.NewSession()

			g.Expect(err).Should(BeNil())
			g.Expect(session).ShouldNot(BeNil())

			sender, err := session.NewSender(
				amqp.LinkTargetAddress(linkAddress),
			)
			g.Expect(err).Should(BeNil())
			ctx, cancel := context.WithTimeout(ctx, 4*time.Second)

			defer sender.Close(ctx)
			defer cancel()

			msg := amqp.NewMessage([]byte("Hello! from:" + clientId))
			msg.ApplicationProperties = make(map[string]interface{})
			msg.ApplicationProperties["MessageID"] = messageId
			msg.ApplicationProperties["ClientID"] = clientId

			err = sender.Send(ctx, msg)
			g.Expect(err).Should(BeNil())
		}

		consumeMatchingMessage := func(url string, linkAddress string, receivedTracker map[string]*list.List, g Gomega) {

			client, err := amqp.Dial(url, amqp.ConnSASLPlain("dummy-user", "dummy-pass"))
			g.Expect(err).Should(BeNil())
			g.Expect(client).ShouldNot(BeNil())

			defer client.Close()

			session, err := client.NewSession()
			g.Expect(err).Should(BeNil())
			g.Expect(session).ShouldNot(BeNil())

			receiver, err := session.NewReceiver(
				amqp.LinkSourceAddress(linkAddress),
				amqp.LinkSourceCapabilities("queue"),
				amqp.LinkCredit(50),
			)
			g.Expect(err).Should(BeNil())
			g.Expect(receiver).ShouldNot(BeNil())

			ctx, cancel := context.WithTimeout(ctx, 600*time.Millisecond)
			defer receiver.Close(ctx)
			defer cancel()

			// Receive messages till error or nil
			for {
				msg, err := receiver.Receive(ctx)

				if err != nil || msg == nil {
					break
				}

				g.Expect(err).Should(BeNil())
				g.Expect(msg).ShouldNot(BeNil())

				var senderClientId = msg.ApplicationProperties["ClientID"].(string)
				receivedTracker[senderClientId].PushBack(msg.ApplicationProperties["MessageID"])

				err = receiver.AcceptMessage(ctx, msg)
				g.Expect(err).Should(BeNil())

			}
		}

		It("deploy 2 with clientID auto sharding", func() {

			isClusteredBoolean := false
			NOT := false
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
				TimeoutSeconds:      5,
			}
			crd.Spec.DeploymentPlan.LivenessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
				TimeoutSeconds:      5,
			}

			crd.Spec.DeploymentPlan.Size = 2
			crd.Spec.DeploymentPlan.Clustered = &isClusteredBoolean
			crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{{
				Name:                "tcp",
				Port:                62616,
				Expose:              false,
				BindToAllInterfaces: &NOT,
			}}

			linkAddress := "LB.TESTQ"

			crd.Spec.BrokerProperties = []string{
				"connectionRouters.autoShard.keyType=CLIENT_ID",
				"connectionRouters.autoShard.localTargetFilter=NULL|${STATEFUL_SET_ORDINAL}|-${STATEFUL_SET_ORDINAL}",
				"connectionRouters.autoShard.policyConfiguration=CONSISTENT_HASH_MODULO",
				"connectionRouters.autoShard.policyConfiguration.properties.MODULO=2",
				"acceptorConfigurations.tcp.params.router=autoShard",           // matching spec.acceptor
				"addressesSettings.\"LB.#\".defaultAddressRoutingType=ANYCAST", // b/c cannot set linkTargetCapabilities from amqp client
			}

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("Deploying the CRD " + crd.ObjectMeta.Name)
				Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

				crdNsName := types.NamespacedName{
					Name:      crd.Name,
					Namespace: defaultNamespace,
				}

				deployedCrd := &brokerv1beta1.ActiveMQArtemis{}

				By("Finding cluster host")
				baseUrl, err := url.Parse(testEnv.Config.Host)
				Expect(err).Should(BeNil())

				ipAddressNet, err := net.LookupIP(baseUrl.Hostname())
				Expect(err).Should(BeNil())
				ipAddress := ipAddressNet[0].String()

				By("verifying started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, crdNsName, deployedCrd)).Should(Succeed())
					g.Expect(len(deployedCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(2))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("exposing ss-0 via NodePort")
				ss0Service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: crd.Name + "-nodeport-0", Namespace: defaultNamespace},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"ActiveMQArtemis":                    crd.Name,
							"statefulset.kubernetes.io/pod-name": namer.CrToSS(crd.Name) + "-0",
						},
						Type: "NodePort",
						Ports: []corev1.ServicePort{
							{
								Port:       61617,
								TargetPort: intstr.FromInt(62616),
							},
						},
						ExternalIPs: []string{ipAddress},
					},
				}
				Expect(k8sClient.Create(ctx, ss0Service)).Should(Succeed())

				By("exposing ss-1 via NodePort")
				ss1Service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: crd.Name + "-nodeport-1", Namespace: defaultNamespace},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"ActiveMQArtemis":                    crd.Name,
							"statefulset.kubernetes.io/pod-name": namer.CrToSS(crd.Name) + "-1",
						},
						Type: "NodePort",
						Ports: []corev1.ServicePort{
							{
								Port:       61618,
								TargetPort: intstr.FromInt(62616),
							},
						},
						ExternalIPs: []string{string(ipAddress)},
					},
				}
				Expect(k8sClient.Create(ctx, ss1Service)).Should(Succeed())

				urls := []string{"amqp://" + baseUrl.Hostname() + ":61617", "amqp://" + baseUrl.Hostname() + ":61618"}

				By("verify partition by sending eventually... expect auto-shard error if we get the wrong broker")
				numberOfMessagesToSendPerClientId := 5
				sentMessageSequenceId := 0
				urlBalancerCounter := 1 // round robin over urls if len(urls) > 0
				clientIds := []string{"W", "ONE", "TWO", "THREE", "FOUR", "0299s-99", "D-0301-c-e-SomeHostBla"}
				for _, id := range clientIds {
					// send messages on new connection with each clientId
					for i := 0; i < numberOfMessagesToSendPerClientId; i++ {
						Eventually(func(g Gomega) {
							urlBalancerCounter++
							produceMessage(urls[urlBalancerCounter%len(urls)], id, linkAddress, sentMessageSequenceId+1, g)
							sentMessageSequenceId++ // on success
						}, timeout*4, interval).Should(Succeed())
					}
				}

				Expect(len(clientIds) * numberOfMessagesToSendPerClientId).Should(BeEquivalentTo(sentMessageSequenceId))

				By("verify partition by consuming messages from each broker")
				receivedIdTracker := map[string]*list.List{}
				for _, id := range clientIds {
					receivedIdTracker[id] = list.New()
				}
				for _, url := range urls {
					Eventually(func(g Gomega) {
						consumeMatchingMessage(url, linkAddress, receivedIdTracker, g)
					}).Should(Succeed())
				}

				for _, list := range receivedIdTracker {
					Expect(list.Len()).Should(BeEquivalentTo(5))

					By("verifying received in order per clientId, ie: producer was sharded nicely")
					receivedIdTracker := list.Front().Value.(int64)
					for e := list.Front(); e != nil; e = e.Next() {
						if e.Value == receivedIdTracker {
							receivedIdTracker++
						}
					}

					By("verifying all in order")
					Expect(receivedIdTracker - list.Front().Value.(int64)).Should(BeEquivalentTo(numberOfMessagesToSendPerClientId))
				}

				Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, ss0Service)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, ss1Service)).Should(Succeed())
			}

		})
	})

	Context("Probe defaults reconcile", func() {
		It("deploy", func() {
			boolTrueVal := true
			boolFalseVal := false

			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1
			crd.Spec.DeploymentPlan.MessageMigration = &boolTrueVal

			crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       1,
				TimeoutSeconds:      5,
				// default values are server-side applied - the ones that we set that are > 0 get tracked as owned by us
				// omitted values have default nil,false,0
				// SuccessThreshod defaults to 1 and is applied on write, our 0 default is ignored
			}

			By("Deploying Cr with Probe and defaults " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			brokerKey := types.NamespacedName{Name: crd.Name, Namespace: crd.Namespace}
			deployedCrd := &brokerv1beta1.ActiveMQArtemis{}

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("verifying started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, deployedCrd)).Should(Succeed())
					g.Expect(len(deployedCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, timeout*5, interval).Should(Succeed())

				key := types.NamespacedName{
					Name:      namer.CrToSS(crd.Name),
					Namespace: defaultNamespace,
				}
				currentSS := &appsv1.StatefulSet{}
				var ssVersion string
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
					ssVersion = currentSS.ResourceVersion
					g.Expect(ssVersion).ShouldNot(BeEmpty())

					By("verifying default filled in and respected")
					g.Expect(currentSS.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold).Should(BeEquivalentTo(1))
					g.Expect(currentSS.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).Should(BeEquivalentTo(3))

					By("verifying what was configured")
					g.Expect(currentSS.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).Should(BeEquivalentTo(1))
				}, timeout, interval).Should(Succeed())

				Expect(k8sClient.Get(ctx, brokerKey, deployedCrd)).Should(Succeed())
				crdVer := deployedCrd.ResourceVersion
				deployedCrd.Spec.DeploymentPlan.MessageMigration = &boolFalseVal
				By("force reconcile via CR update of Ver:" + crdVer)
				Expect(k8sClient.Update(ctx, deployedCrd)).Should(Succeed())

				By("verify no change in ssVersion but change in brokerCr")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, brokerKey, deployedCrd)).Should(Succeed())
					By("verify crd new revision: " + deployedCrd.ResourceVersion)
					g.Expect(deployedCrd.ResourceVersion).ShouldNot(Equal(crdVer))

					By("Verify second generation...")
					g.Expect(deployedCrd.Generation).Should(BeEquivalentTo(2))

					g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
					By("verify no new ss revision: " + currentSS.ResourceVersion)
					g.Expect(currentSS.ResourceVersion).Should(Equal(ssVersion))
					By("Verify first generation...")
					g.Expect(currentSS.Generation).Should(BeEquivalentTo(1))

				}, timeout, interval).Should(Succeed())

				By("Force SS update via CR update to Probe")
				Expect(k8sClient.Get(ctx, brokerKey, deployedCrd)).Should(Succeed())
				deployedCrd.Spec.DeploymentPlan.ReadinessProbe.InitialDelaySeconds = 2
				Expect(k8sClient.Update(ctx, deployedCrd)).Should(Succeed())

				By("verifying update to SS")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
					By("verify no new ss revision: " + currentSS.ResourceVersion)
					g.Expect(currentSS.ResourceVersion).ShouldNot(Equal(ssVersion))
					By("Verify second generation...")
					g.Expect(currentSS.Generation).Should(BeEquivalentTo(2))

				}, timeout*2, interval).Should(Succeed())

				By("Deleting the ready ss pod")
				key = types.NamespacedName{Name: namer.CrToSS(crd.Name) + "-0", Namespace: defaultNamespace}
				zeroGracePeriodSeconds := int64(0) // immediate delete
				var podResourceVersion string
				Eventually(func(g Gomega) {
					pod := &corev1.Pod{}
					g.Expect(k8sClient.Get(ctx, key, pod)).Should(Succeed())
					podResourceVersion = pod.ResourceVersion
					g.Expect(podResourceVersion).ShouldNot(BeEmpty())
					g.Expect(k8sClient.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: &zeroGracePeriodSeconds})).Should(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Again finding pod instance with new resource version")
				Eventually(func(g Gomega) {
					pod := &corev1.Pod{}
					g.Expect(k8sClient.Get(ctx, key, pod)).Should(Succeed())
					g.Expect(pod.ResourceVersion).ShouldNot(Equal(podResourceVersion))
				}, timeout, interval).Should(Succeed())

				By("Verying new pod restarted via CR Status")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, brokerKey, deployedCrd)).Should(Succeed())
					g.Expect(len(deployedCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
				}, timeout*5, interval).Should(Succeed())

			}

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())
		})
	})

	Context("SS delete recreate Test", func() {
		It("deploy, delete ss, verify", func() {

			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			key := types.NamespacedName{
				Name:      namer.CrToSS(crd.Name),
				Namespace: defaultNamespace,
			}

			currentSS := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			brokerKey := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: defaultNamespace}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			By("manually deleting SS quickly")
			ssVersion := currentSS.ResourceVersion
			Expect(k8sClient.Delete(ctx, currentSS)).Should(Succeed())

			By("checking new version created")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
				g.Expect(currentSS.ResourceVersion).ShouldNot(Equal(ssVersion))
				ssVersion = currentSS.ResourceVersion
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("verifying started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("manually deleting SS again once ready")
				Expect(k8sClient.Delete(ctx, currentSS)).Should(Succeed())

				By("checking new version created again")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, key, currentSS)).Should(Succeed())
					g.Expect(currentSS.ResourceVersion).ShouldNot(Equal(ssVersion))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
		})
	})

	Context("PVC no gc test", func() {
		It("deploy, verify, undeploy, verify", func() {

			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1
			crd.Spec.DeploymentPlan.PersistenceEnabled = true

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("Deploying the CRD " + crd.ObjectMeta.Name)
				Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

				createdCrd := &brokerv1beta1.ActiveMQArtemis{}
				brokerKey := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: defaultNamespace}

				By("verifing started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("finding PVC")
				pvcKey := types.NamespacedName{Namespace: defaultNamespace, Name: crd.Name + "-" + namer.CrToSS(crd.Name) + "-0"}
				pvc := &corev1.PersistentVolumeClaim{}
				Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
				// at some stage, there should/could be an owner or SS controller does this gc management
				Expect(len(pvc.OwnerReferences)).Should(BeEquivalentTo(0))

				By("undeploying CR")
				Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

				Eventually(func(g Gomega) {
					By("again finding PVC b/c it has been gc'ed - " + pvcKey.Name)
					g.Expect(k8sClient.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})).Should(Succeed())
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}
		})
	})

	Context("PVC upgrade owner reference test", Label("pvc-owner-reference-upgrade"), func() {
		It("faking a broker deployment with owned pvc", func() {
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1
			crd.Spec.DeploymentPlan.PersistenceEnabled = true

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("Deploying the CRD " + crd.ObjectMeta.Name)
				Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

				createdCrd := &brokerv1beta1.ActiveMQArtemis{}
				brokerKey := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: defaultNamespace}

				By("verifing started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("finding PVC")
				pvcKey := types.NamespacedName{Namespace: defaultNamespace, Name: crd.Name + "-" + namer.CrToSS(crd.Name) + "-0"}
				pvc := &corev1.PersistentVolumeClaim{}
				Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
				Expect(len(pvc.OwnerReferences)).Should(BeEquivalentTo(0))

				// added back owner reference
				pvc.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: v1beta1.GroupVersion.String(),
					Kind:       "ActiveMQArtemis",
					Name:       createdCrd.Name,
					UID:        createdCrd.GetUID()}}
				Expect(k8sClient.Update(ctx, pvc)).Should(Succeed())

				Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
				Expect(len(pvc.OwnerReferences)).To(BeEquivalentTo(1))

				shutdownControllerManager()

				Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
				Expect(len(pvc.OwnerReferences)).To(BeEquivalentTo(1))

				createControllerManager(true, defaultNamespace)

				// Expect the owner reference gets removed
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
					g.Expect(len(pvc.OwnerReferences)).Should(BeEquivalentTo(0))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("undeploying CR")
				Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

				Eventually(func(g Gomega) {
					By("again finding PVC should succeed - " + pvcKey.Name)
					g.Expect(k8sClient.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})).Should(Succeed())
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			} else {
				fmt.Println("The test is skipped because it requires existing cluster")
			}
		})
	})

	Context("Tolerations Existing Cluster", func() {
		It("Toleration of artemis", func() {

			By("Creating a crd with tolerations")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1
			crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
			}
			crd.Spec.DeploymentPlan.Tolerations = []corev1.Toleration{
				{
					Key:    "artemis",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			}

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			brokerKey := types.NamespacedName{Name: crd.Name, Namespace: crd.Namespace}
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			// some required services on crc get evicted which invalidates this test of taints
			isOpenshift, err := environments.DetectOpenshift()
			Expect(err).Should(BeNil())
			if !isOpenshift && os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("veryify pod started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, timeout*5, interval).Should(Succeed())

				By("veryify no taints on node")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(0))
				}, timeout*2, interval).Should(Succeed())

				By("veryify pod status still started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, timeout*2, interval).Should(Succeed())

				By("applying taints to node")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(0))
					node.Spec.Taints = []corev1.Taint{{Key: "artemis", Effect: corev1.TaintEffectPreferNoSchedule}}

					g.Expect(k8sClient.Update(ctx, &node)).Should(Succeed())
				}, timeout*2, interval).Should(Succeed())

				By("veryify pod status still started")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, timeout*2, interval).Should(Succeed())

				By("reverting taints on node")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(1))
					node.Spec.Taints = []corev1.Taint{}
					g.Expect(k8sClient.Update(ctx, &node)).Should(Succeed())
				}, timeout*2, interval).Should(Succeed())

			}

			Expect(k8sClient.Delete(ctx, createdCrd))

		})

		It("Toleration of artemis required add/remove verify status", func() {

			By("Creating a crd with tolerations")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Size = 1
			crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       5,
			}

			crd.Spec.DeploymentPlan.Tolerations = []corev1.Toleration{
				{
					Key:      "artemis",
					Value:    "ok",
					Operator: corev1.TolerationOpEqual,
					Effect:   corev1.TaintEffectNoExecute,
				},
			}

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			brokerKey := types.NamespacedName{Name: crd.Name, Namespace: crd.Namespace}
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			// some required services get evicted which invalidates this test of taints
			if false && os.Getenv("USE_EXISTING_CLUSTER") == "true" {

				By("veryify pod started as no taints in play")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, timeout*5, interval).Should(Succeed())

				By("apply matching taint with wrong value, force eviction")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(0))

					node.Spec.Taints = []corev1.Taint{{Key: "artemis", Value: "no", Effect: corev1.TaintEffectNoExecute}}
					g.Expect(k8sClient.Update(ctx, &node)).Should(Succeed())

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("veryify pod status now starting, evicted!")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Starting)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("updating taint to match key and value")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(1))
					node.Spec.Taints = []corev1.Taint{{Key: "artemis", Value: "ok", Effect: corev1.TaintEffectNoExecute}}
					g.Expect(k8sClient.Update(ctx, &node)).Should(Succeed())
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("veryify ready status on CR, started again")
				Eventually(func(g Gomega) {

					g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
					g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("reverting taints on node")
				Eventually(func(g Gomega) {

					// find our node, take the first one...
					nodes := &corev1.NodeList{}
					g.Expect(k8sClient.List(ctx, nodes, &client.ListOptions{})).Should(Succeed())
					g.Expect(len(nodes.Items) > 0).Should(BeTrue())

					node := nodes.Items[0]
					g.Expect(len(node.Spec.Taints)).Should(BeEquivalentTo(1))
					node.Spec.Taints = []corev1.Taint{}
					g.Expect(k8sClient.Update(ctx, &node)).Should(Succeed())
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			}

			Expect(k8sClient.Delete(ctx, createdCrd))

		})

	})

	Context("Console secret Test", func() {

		crd := generateArtemisSpec(defaultNamespace)
		crd.Spec.Console.Expose = true
		crd.Spec.Console.SSLEnabled = true

		reconcilerImpl := &ActiveMQArtemisReconcilerImpl{}

		It("deploy broker with ssl enabled console", func() {
			os.Setenv("OPERATOR_OPENSHIFT", "true")
			defer os.Unsetenv("OPERATOR_OPENSHIFT")

			defaultConsoleSecretName := crd.Name + "-console-secret"
			currentSS := &appsv1.StatefulSet{}
			currentSS.Name = namer.CrToSS(crd.Name)
			currentSS.Namespace = defaultNamespace
			currentSS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name: "main-container",
			}}
			currentSS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "init-container",
			}}

			namer := MakeNamers(&crd)
			reconcilerImpl.ProcessConsole(&crd, *namer, brokerReconciler.Client, brokerReconciler.Scheme, currentSS)

			secretName := namer.SecretsConsoleNameBuilder.Name()
			internalSecretName := secretName + "-internal"
			consoleArgs := "AMQ_CONSOLE_ARGS"

			foundSecret := false
			foundInternalSecret := false
			defaultSecretPath := "/etc/" + defaultConsoleSecretName + "-volume/"
			defaultSslArgs := " --ssl-key " + defaultSecretPath + "broker.ks --ssl-key-password password --ssl-trust " + defaultSecretPath + "client.ts --ssl-trust-password password"
			for _, reqres := range reconcilerImpl.requestedResources {
				if reqres.GetObjectKind().GroupVersionKind().Kind == "Secret" {
					secret := reqres.(*corev1.Secret)
					if secret.Name == secretName {
						foundSecret = true
					}
					if secret.Name == internalSecretName {
						foundInternalSecret = true
						consoleSslValue := secret.StringData[consoleArgs]
						Expect(consoleSslValue).To(Equal(defaultSslArgs))
					}
				}
			}
			Expect(foundSecret).To(BeTrue())
			Expect(foundInternalSecret).To(BeTrue())

			foundSecretRef := false
			foundSecretKey := false
			for _, evar := range currentSS.Spec.Template.Spec.InitContainers[0].Env {
				if evar.Name == consoleArgs {
					if evar.ValueFrom.SecretKeyRef.Name == internalSecretName {
						foundSecretRef = true
					}
					if evar.ValueFrom.SecretKeyRef.Key == consoleArgs {
						foundSecretKey = true
					}
				}
			}
			Expect(foundSecretRef).To(BeTrue())
			Expect(foundSecretKey).To(BeTrue())
		})
	})

	Context("PodSecurityContext Test", func() {
		It("Setting the pods PodSecurityContext", func() {
			By("Creating a CR instance with PodSecurityContext configured")

			podSecurityContext := corev1.PodSecurityContext{}
			retrievedCR := &brokerv1beta1.ActiveMQArtemis{}
			createdSS := &appsv1.StatefulSet{}

			context := context.Background()
			defaultCR := generateArtemisSpec(defaultNamespace)
			defaultCR.Spec.DeploymentPlan.PodSecurityContext = &podSecurityContext

			By("Deploying the CR " + defaultCR.ObjectMeta.Name)
			Expect(k8sClient.Create(context, &defaultCR)).Should(Succeed())

			By("Making sure that the CR gets deployed " + defaultCR.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(defaultCR.ObjectMeta.Name, defaultNamespace, retrievedCR)
			}, timeout, interval).Should(BeTrue())
			Expect(retrievedCR.Name).Should(Equal(defaultCR.ObjectMeta.Name))

			By("Checking that the StatefulSet has been created with a PodSecurityContext field " + namer.CrToSS(retrievedCR.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(retrievedCR.Name), Namespace: defaultNamespace}
				if err := k8sClient.Get(context, key, createdSS); nil != err {
					return false
				}
				return nil != createdSS.Spec.Template.Spec.SecurityContext
			})

			By("Check for default CR instance deletion")
			Expect(k8sClient.Delete(context, retrievedCR)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(defaultCR.ObjectMeta.Name, defaultNamespace, retrievedCR)
			}, timeout, interval).Should(BeTrue())

			By("Creating a non-default SELinuxOptions CR instance")
			nonDefaultCR := generateArtemisSpec(defaultNamespace)

			credentialSpecName := "CredentialSpecName0"
			credentialSpec := "CredentialSpec0"
			runAsUserName := "RunAsUserName0"
			hostProcess := false
			var runAsUser int64 = 1000
			var runAsGroup int64 = 1001
			runAsNonRoot := true
			var supplementalGroupA int64 = 2000
			var supplementalGroupB int64 = 2001
			supplementalGroups := []int64{supplementalGroupA, supplementalGroupB}
			var fsGroup int64 = 3000
			sysctlA := corev1.Sysctl{
				Name:  "NameA",
				Value: "ValueA",
			}
			sysctlB := corev1.Sysctl{
				Name:  "NameB",
				Value: "ValueB",
			}
			sysctls := []corev1.Sysctl{sysctlA, sysctlB}

			fsGCPString := "GroupChangePolicy0"
			fsGCP := corev1.PodFSGroupChangePolicy(fsGCPString)
			localhostProfile := "LocalhostProfile0"
			seccompProfile := corev1.SeccompProfile{
				Type:             corev1.SeccompProfileTypeUnconfined,
				LocalhostProfile: &localhostProfile,
			}

			nonDefaultCR.Spec.DeploymentPlan.PodSecurityContext = &corev1.PodSecurityContext{
				SELinuxOptions: &corev1.SELinuxOptions{
					User:  "TestUser0",
					Role:  "TestRole0",
					Type:  "TestType0",
					Level: "TestLevel0",
				},
				WindowsOptions: &corev1.WindowsSecurityContextOptions{
					GMSACredentialSpecName: &credentialSpecName,
					GMSACredentialSpec:     &credentialSpec,
					RunAsUserName:          &runAsUserName,
					HostProcess:            &hostProcess,
				},
				RunAsUser:           &runAsUser,
				RunAsGroup:          &runAsGroup,
				RunAsNonRoot:        &runAsNonRoot,
				SupplementalGroups:  supplementalGroups,
				FSGroup:             &fsGroup,
				Sysctls:             sysctls,
				FSGroupChangePolicy: &fsGCP,
				SeccompProfile:      &seccompProfile,
			}

			By("Deploying the non-default CR instance named " + nonDefaultCR.Name)
			Expect(k8sClient.Create(context, &nonDefaultCR)).Should(Succeed())

			retrievednonDefaultCR := brokerv1beta1.ActiveMQArtemis{}
			By("Checking to ensure that the non-default CR was created with the right values " + nonDefaultCR.Name)
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: nonDefaultCR.Name, Namespace: defaultNamespace}
				g.Expect(k8sClient.Get(context, key, &retrievednonDefaultCR)).Should(Succeed())
				g.Expect(reflect.DeepEqual(nonDefaultCR.Spec.DeploymentPlan.PodSecurityContext,
					retrievednonDefaultCR.Spec.DeploymentPlan.PodSecurityContext)).Should(BeTrue())

			}, timeout, interval).Should(Succeed())

			By("Checking that the StatefulSet has been created with the non-default PodSecurityContext " + namer.CrToSS(nonDefaultCR.Name))
			Eventually(func(g Gomega) {
				nonDefaultSS := &appsv1.StatefulSet{}
				key := types.NamespacedName{Name: namer.CrToSS(nonDefaultCR.Name), Namespace: defaultNamespace}
				g.Expect(k8sClient.Get(context, key, nonDefaultSS)).Should(Succeed())

				g.Expect(nonDefaultSS.Spec.Template.Spec.SecurityContext).ShouldNot(BeNil())
				g.Expect(reflect.DeepEqual(nonDefaultCR.Spec.DeploymentPlan.PodSecurityContext, nonDefaultSS.Spec.Template.Spec.SecurityContext)).Should(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Check for non-default CR instance deletion")
			Expect(k8sClient.Delete(context, &retrievednonDefaultCR)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(retrievednonDefaultCR.ObjectMeta.Name, defaultNamespace, &retrievednonDefaultCR)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Affinity Test", func() {
		It("setting Pod Affinity", func() {
			By("Creating a crd with pod affinity")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			labelSelector := metav1.LabelSelector{}
			labelSelector.MatchLabels = make(map[string]string)
			labelSelector.MatchLabels["key"] = "value"

			podAffinityTerm := corev1.PodAffinityTerm{}
			podAffinityTerm.LabelSelector = &labelSelector

			podAffinity := corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					podAffinityTerm,
				},
			}
			crd.Spec.DeploymentPlan.Affinity.PodAffinity = &podAffinity

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))
			By("Checking that Stateful Set is Created with the node selectors " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Affinity.PodAffinity != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the pd affinity are correct")
			Expect(createdSs.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["key"] == "value").Should(BeTrue())

			By("Updating the CR")
			Eventually(func(g Gomega) {

				// we need to update the latest version and deal with update failures
				g.Expect(getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)).Should(BeTrue())
				original := createdCrd

				labelSelector = metav1.LabelSelector{}
				labelSelector.MatchLabels = make(map[string]string)
				labelSelector.MatchLabels["key"] = "differentvalue"

				podAffinityTerm = corev1.PodAffinityTerm{}
				podAffinityTerm.LabelSelector = &labelSelector
				podAffinity = corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						podAffinityTerm,
					},
				}
				original.Spec.DeploymentPlan.Affinity.PodAffinity = &podAffinity
				By("Redeploying the CRD")
				g.Expect(k8sClient.Update(ctx, original)).Should(Succeed())

				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				g.Expect(k8sClient.Get(ctx, key, createdSs))

				g.Expect(createdSs.Spec.Template.Spec.Affinity.PodAffinity).ShouldNot(BeNil())

				g.Expect(len(createdSs.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution)).Should(BeEquivalentTo(1))

				By("Making sure the pd affinity are correct")
				g.Expect(createdSs.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["key"]).Should(Equal("differentvalue"))

			}, timeout, interval).Should(Succeed())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
		It("setting Pod AntiAffinity", func() {
			By("Creating a crd with pod anti affinity")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			labelSelector := metav1.LabelSelector{}
			labelSelector.MatchLabels = make(map[string]string)
			labelSelector.MatchLabels["key"] = "value"

			podAffinityTerm := corev1.PodAffinityTerm{}
			podAffinityTerm.LabelSelector = &labelSelector
			podAntiAffinity := corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					podAffinityTerm,
				},
			}
			crd.Spec.DeploymentPlan.Affinity.PodAntiAffinity = &podAntiAffinity

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))
			By("Checking that Stateful Set is Created with the node selectors " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Affinity.PodAntiAffinity != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the pd affinity are correct")
			Expect(createdSs.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["key"]).Should(Equal("value"))

			By("Updating the CR")
			Eventually(func(g Gomega) {

				g.Expect(getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)).Should(BeTrue())
				original := createdCrd

				labelSelector = metav1.LabelSelector{}
				labelSelector.MatchLabels = make(map[string]string)
				labelSelector.MatchLabels["key"] = "differentvalue"

				podAffinityTerm = corev1.PodAffinityTerm{}
				podAffinityTerm.LabelSelector = &labelSelector
				podAntiAffinity = corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						podAffinityTerm,
					},
				}
				original.Spec.DeploymentPlan.Affinity.PodAntiAffinity = &podAntiAffinity
				By("Redeploying the CRD")
				k8sClient.Update(ctx, original)

				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				g.Expect(k8sClient.Get(ctx, key, createdSs)).Should(Succeed())

				g.Expect(createdSs.Spec.Template.Spec.Affinity.PodAntiAffinity).ShouldNot(BeNil())

				g.Expect(len(createdSs.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)).Should(Equal(1))

				By("Making sure the pd affinity are correct")
				g.Expect(createdSs.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["key"]).Should(Equal("differentvalue"))

			}, timeout, interval).Should(Succeed())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
		It("setting Node AntiAffinity", func() {
			By("Creating a crd with node affinity")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			nodeSelectorRequirement := corev1.NodeSelectorRequirement{
				Key:    "foo",
				Values: make([]string, 1),
			}
			nodeSelectorRequirements := [1]corev1.NodeSelectorRequirement{nodeSelectorRequirement}
			nodeSelectorRequirements[0] = nodeSelectorRequirement
			nodeSelectorTerm := corev1.NodeSelectorTerm{MatchExpressions: nodeSelectorRequirements[:]}
			nodeSelectorTerms := [1]corev1.NodeSelectorTerm{nodeSelectorTerm}
			nodeSelector := corev1.NodeSelector{
				NodeSelectorTerms: nodeSelectorTerms[:],
			}
			nodeAffinity := corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &nodeSelector,
			}
			crd.Spec.DeploymentPlan.Affinity.NodeAffinity = &nodeAffinity

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))
			By("Checking that Stateful Set is Created with the node selectors " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Affinity.NodeAffinity != nil
			}, timeout, interval).Should(BeTrue())

			By("Making sure the node affinity are correct")
			Expect(createdSs.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal("foo"))

			Eventually(func(g Gomega) {
				By("Updating the CR")
				g.Expect(getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)).Should(BeTrue())
				original := createdCrd

				nodeSelectorRequirement = corev1.NodeSelectorRequirement{
					Key:    "bar",
					Values: make([]string, 2),
				}
				nodeSelectorRequirements = [1]corev1.NodeSelectorRequirement{nodeSelectorRequirement}
				nodeSelectorRequirements[0] = nodeSelectorRequirement
				nodeSelectorTerm = corev1.NodeSelectorTerm{MatchExpressions: nodeSelectorRequirements[:]}
				nodeSelectorTerms = [1]corev1.NodeSelectorTerm{nodeSelectorTerm}
				nodeSelector = corev1.NodeSelector{
					NodeSelectorTerms: nodeSelectorTerms[:],
				}
				nodeAffinity = corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &nodeSelector,
				}
				original.Spec.DeploymentPlan.Affinity.NodeAffinity = &nodeAffinity
				By("Redeploying the CRD")
				k8sClient.Update(ctx, original)

				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				g.Expect(k8sClient.Get(ctx, key, createdSs)).Should(Succeed())

				g.Expect(len(createdSs.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)).Should(BeEquivalentTo(1))

				By("Making sure the pod affinity is correct")
				g.Expect(createdSs.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal("bar"))

			}, timeout, interval).Should(Succeed())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Node Selector Test", func() {
		It("passing in 2 labels", func() {
			By("Creating a crd with 2 selectors")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			nodeSelector := map[string]string{
				"location": "production",
				"type":     "foo",
			}
			crd.Spec.DeploymentPlan.NodeSelector = nodeSelector

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))
			By("Checking that Stateful Set is Created with the node selectors " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return len(createdSs.Spec.Template.Spec.NodeSelector) == 2
			}, timeout, interval).Should(Equal(true))

			By("Making sure the node selectors are correct")
			Expect(createdSs.Spec.Template.Spec.NodeSelector["location"] == "production").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.NodeSelector["type"] == "foo").Should(BeTrue())

			By("Updating the CR")
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd) }, timeout, interval).Should(BeTrue())
			original := createdCrd

			nodeSelector = map[string]string{
				"type": "foo",
			}
			original.Spec.DeploymentPlan.NodeSelector = nodeSelector
			By("Redeploying the CRD")
			Expect(k8sClient.Update(ctx, original)).Should(Succeed())

			Eventually(func() int {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return -1
				}
				return len(createdSs.Spec.Template.Spec.NodeSelector)
			}, timeout, interval).Should(Equal(1))

			By("Making sure the node selectors are correct")
			Expect(createdSs.Spec.Template.Spec.NodeSelector["location"] == "production").Should(BeFalse())
			Expect(createdSs.Spec.Template.Spec.NodeSelector["type"] == "foo").Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Labels Test", func() {
		It("passing in 2 labels", func() {
			By("Creating a crd with 2 labels, verifying only on pod template")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.DeploymentPlan.Labels = make(map[string]string)
			crd.Spec.DeploymentPlan.Labels["key1"] = "val1"
			crd.Spec.DeploymentPlan.Labels["key2"] = "val2"

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))
			By("Checking that Stateful Set is Created with the labels " + namer.CrToSS(createdCrd.Name))
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				g.Expect(k8sClient.Get(ctx, key, createdSs)).Should(Succeed())

				g.Expect(len(createdSs.ObjectMeta.Labels)).Should(BeNumerically("==", 2))
				g.Expect(len(createdSs.Spec.Selector.MatchLabels)).Should(BeNumerically("==", 2))

				g.Expect(len(createdSs.Spec.Template.Labels)).Should(BeNumerically("==", 4+len(pods.GetAdditionalLabels(version.Version))))

			}, timeout, interval).Should(Succeed())

			By("Making sure the labels are correct")
			Expect(createdSs.Spec.Template.Labels["key1"]).Should(Equal("val1"))
			Expect(createdSs.Spec.Template.Labels["key2"]).Should(Equal("val2"))

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))

		})
	})

	Context("Different namespace, deployed before start", func() {
		It("Expect pod desc", func() {
			By("By creating a new crd")
			ctx := context.Background()

			nonDefaultNamespace := "non-default"

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nonDefaultNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(errors.IsConflict(err))
			}

			crd := generateArtemisSpec(nonDefaultNamespace)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, nonDefaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			// would like more status updates on createdCrd

			By("By checking absence of stateful set with no matching controller")
			key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: nonDefaultNamespace}
			createdSs := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, key, createdSs)).ShouldNot(Succeed())

			By("By starting reconciler for this namespace")

			shutdownControllerManager()
			createControllerManager(true, nonDefaultNamespace)

			key = types.NamespacedName{Name: createdCrd.Name, Namespace: nonDefaultNamespace}

			Eventually(func(g Gomega) {
				By("Checking stopped status of CR, deployed with replica count 0")
				g.Expect(k8sClient.Get(ctx, key, createdCrd)).Should(Succeed())
				g.Expect(len(createdCrd.Status.PodStatus.Stopped)).Should(BeEquivalentTo(1))
			}, timeout, interval).Should(Succeed())

			By("deleting crd")
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

			By("restoring default manager")
			shutdownControllerManager()
			createControllerManager(true, defaultNamespace)
		})
	})

	Context("Tolerations Test", func() {
		It("passing in 2 tolerations", func() {

			By("Creating a crd with 2 tolerations")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
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
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the tolerations " + namer.CrToSS(createdCrd.Name))
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

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

				ok := getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
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
			Eventually(func() (int, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return 0, err
				}
				return len(createdSs.Spec.Template.Spec.Tolerations), err
			}, timeout, interval).Should(Equal(1))
			Expect(len(createdSs.Spec.Template.Spec.Tolerations) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Key == "yes").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Value == "No").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Tolerations[0].Effect == "NoSchedule").Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("Liveness Probe Tests", func() {
		It("Override Liveness Probe No Exec", func() {
			By("By creating a crd with Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			livenessProbe := corev1.Probe{}
			livenessProbe.PeriodSeconds = 5
			livenessProbe.InitialDelaySeconds = 6
			livenessProbe.TimeoutSeconds = 7
			livenessProbe.SuccessThreshold = 8
			livenessProbe.FailureThreshold = 9
			crd.Spec.DeploymentPlan.LivenessProbe = &livenessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}

			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Liveness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.TCPSocket.Port.String() == "8161").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 5).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds == 6).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds == 7).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold == 8).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == 9).Should(BeTrue())

			By("Updating the CR")
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd) }, timeout, interval).Should(BeTrue())
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
			By("Redeploying the CRD")
			By("Redeploying the modified CRD")
			Expect(k8sClient.Update(ctx, original)).Should(Succeed())

			By("Retrieving the new SS to find the modification")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 15
			}, timeout, interval).Should(Equal(true))

			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.Exec != nil).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 15).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds == 16).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds == 17).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold == 18).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == 19).Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool { return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd) }, timeout, interval).Should(BeTrue())
		})

		It("Override Liveness Probe Exec", func() {
			By("By creating a crd with Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			exec := corev1.ExecAction{
				Command: []string{"/broker/bin/artemis check node"},
			}
			livenessProbe := corev1.Probe{}
			livenessProbe.PeriodSeconds = 5
			livenessProbe.InitialDelaySeconds = 6
			livenessProbe.TimeoutSeconds = 7
			livenessProbe.SuccessThreshold = 8
			livenessProbe.FailureThreshold = 9
			livenessProbe.Exec = &exec
			crd.Spec.DeploymentPlan.LivenessProbe = &livenessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Liveness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds == 5).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds == 6).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds == 7).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold == 8).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == 9).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Default Liveness Probe", func() {
			By("By creating a crd without Liveness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that the Liveness Probe is created")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.TCPSocket.Port.String() == "8161").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).Should(BeEquivalentTo(5))

			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Override Liveness Probe Default TCPSocket", func() {
			By("By creating a crd with Liveness Probe")
			ctx := context.Background()
			crd, createdCrd := DeployCustomBroker("", defaultNamespace, func(candidate *brokerv1beta1.ActiveMQArtemis) {
				tcpSocketAction := corev1.TCPSocketAction{
					Port: intstr.FromInt(8161),
				}
				livenessProbe := corev1.Probe{}
				livenessProbe.FailureThreshold = 3
				livenessProbe.InitialDelaySeconds = 60
				livenessProbe.PeriodSeconds = 10
				livenessProbe.SuccessThreshold = 1
				livenessProbe.TCPSocket = &tcpSocketAction
				livenessProbe.TimeoutSeconds = 5

				candidate.Spec.DeploymentPlan.LivenessProbe = &livenessProbe
			})

			createdSs := &appsv1.StatefulSet{}

			By("Checking that Stateful Set is Created with the Liveness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Liveness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.TCPSocket.Port.String() == "8161").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).Should(BeEquivalentTo(5))
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).Should(BeEquivalentTo(3))
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).Should(BeEquivalentTo(60))
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).Should(BeEquivalentTo(10))
			Expect(createdSs.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).Should(BeEquivalentTo(1))

			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Readiness Probe Tests", func() {
		It("Override Readiness Probe No Exec", func() {
			By("By creating a crd with Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			readinessProbe := corev1.Probe{}
			readinessProbe.PeriodSeconds = 5
			readinessProbe.InitialDelaySeconds = 6
			readinessProbe.TimeoutSeconds = 7
			readinessProbe.SuccessThreshold = 8
			readinessProbe.FailureThreshold = 9
			crd.Spec.DeploymentPlan.ReadinessProbe = &readinessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Readiness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[0] == "/bin/bash").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[1] == "-c").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[2] == "/opt/amq/bin/readinessProbe.sh").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Override Readiness Probe Exec", func() {
			By("By creating a crd with Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			exec := corev1.ExecAction{
				Command: []string{"/broker/bin/artemis check node"},
			}
			readinessProbe := corev1.Probe{}
			readinessProbe.Exec = &exec
			crd.Spec.DeploymentPlan.ReadinessProbe = &readinessProbe
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			By("Deploying the CRD")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Making sure that the CRD gets deployed")
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that Stateful Set is Created with the Readiness Probe")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[0] == "/broker/bin/artemis check node").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})

		It("Default Readiness Probe", func() {
			By("By creating a crd without Readiness Probe")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			crd.Spec.AdminUser = "admin"
			crd.Spec.AdminPassword = "password"
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			createdSs := &appsv1.StatefulSet{}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("Checking that the Readiness Probe is created")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err != nil {
					return false
				}
				return createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec != nil
			}, timeout, interval).Should(Equal(true))

			By("Making sure the Readiness probe is correct")
			Expect(len(createdSs.Spec.Template.Spec.Containers) == 1).Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[0] == "/bin/bash").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[1] == "-c").Should(BeTrue())
			Expect(createdSs.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command[2] == "/opt/amq/bin/readinessProbe.sh").Should(BeTrue())
			Expect(k8sClient.Delete(ctx, createdCrd))

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Status", func() {
		It("Expect pod desc", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			// would like more status updates on createdCrd

			By("By checking the status of stateful set")
			Eventually(func() (int, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
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
				key := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, createdCrd)

				if err != nil {
					return -1, err
				}

				return len(createdCrd.Status.PodStatus.Stopped), nil
			}, timeout, interval).Should(Equal(1))

			By("Checking presence of secrets")
			secretList := &corev1.SecretList{}
			opts := &client.ListOptions{
				Namespace: defaultNamespace,
			}
			Eventually(func() int {
				err := k8sClient.List(ctx, secretList, opts)
				if err != nil {
					fmt.Printf("error getting secretList! %v", err)
				}
				count := 0
				for _, s := range secretList.Items {
					if strings.Contains(s.ObjectMeta.Name, createdCrd.Name) {
						count++
					}
				}
				return count
			}, timeout, interval).Should(Equal(2))

			By("deleting crd")
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Validation", func() {
		It("Test labels", func() {
			ctx := context.Background()
			var err error
			var deployedCrdKey types.NamespacedName
			var deployedCrd brokerv1beta1.ActiveMQArtemis
			var deployedStatefulSet *appsv1.StatefulSet
			var deployedResources map[reflect.Type][]client.Object
			statefulSetType := reflect.TypeOf(appsv1.StatefulSet{})

			By("By creating invalid crd")
			invalidCrd := generateArtemisSpec(defaultNamespace)
			invalidCrd.Spec.DeploymentPlan.Labels = map[string]string{"application": "test"}
			Expect(k8sClient.Create(ctx, &invalidCrd)).Should(Succeed())

			By("By creating valid crd")
			validCrd := generateArtemisSpec(defaultNamespace)
			validCrd.Spec.DeploymentPlan.Labels = map[string]string{"test": "test"}
			Expect(k8sClient.Create(ctx, &validCrd)).Should(Succeed())

			By("Checking valid CR")
			deployedCrdKey = types.NamespacedName{Name: validCrd.ObjectMeta.Name, Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deployedCrdKey, &deployedCrd)).Should(Succeed())
				g.Expect(deployedCrd.Name).Should(Equal(validCrd.ObjectMeta.Name))
				g.Expect(len(deployedCrd.Status.PodStatus.Stopped)).Should(Equal(1))
				g.Expect(deployedCrd.Status.PodStatus.Stopped[0]).Should(Equal(namer.CrToSS(validCrd.Name)))
			}, timeout, interval).Should(Succeed())

			By("Checking deployed resources of valid CR")
			deployedResources, err = getDeployedResources(&validCrd, k8sClient)
			Expect(err).Should(Succeed())
			Expect(deployedResources).ShouldNot(BeEmpty())

			By("Checking template labels of valid CR")
			deployedStatefulSet = deployedResources[statefulSetType][0].(*appsv1.StatefulSet)
			Expect(deployedStatefulSet.Spec.Template.Labels["test"]).Should(Equal("test"))

			By("Checking invalid CR")
			deployedCrdKey = types.NamespacedName{Name: invalidCrd.ObjectMeta.Name, Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deployedCrdKey, &deployedCrd)).Should(Succeed())
				g.Expect(deployedCrd.Name).Should(Equal(invalidCrd.ObjectMeta.Name))
			}, timeout, interval).Should(Succeed())

			deployedResources, err = getDeployedResources(&invalidCrd, k8sClient)
			Expect(err).Should(Succeed())
			Expect(deployedResources).Should(BeEmpty())

			By("By updating invalid crd")
			invalidCrd.Spec.DeploymentPlan.Labels = map[string]string{"test": "test"}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deployedCrdKey, &deployedCrd)).Should(Succeed())
				g.Expect(deployedCrd.Name).Should(Equal(invalidCrd.ObjectMeta.Name))
				deployedCrd.Spec.DeploymentPlan.Labels = map[string]string{"test": "test"}
				Expect(k8sClient.Update(ctx, &deployedCrd)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking updated invalid CR")
			deployedCrdKey = types.NamespacedName{Name: invalidCrd.ObjectMeta.Name, Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deployedCrdKey, &deployedCrd)).Should(Succeed())
				g.Expect(deployedCrd.Name).Should(Equal(invalidCrd.ObjectMeta.Name))
				g.Expect(len(deployedCrd.Status.PodStatus.Stopped)).Should(Equal(1))
				g.Expect(deployedCrd.Status.PodStatus.Stopped[0]).Should(Equal(namer.CrToSS(invalidCrd.Name)))
			}, timeout, interval).Should(Succeed())

			By("Checking deployed resources of updated invalid CR")
			deployedResources, err = getDeployedResources(&validCrd, k8sClient)
			Expect(err).Should(Succeed())
			Expect(deployedResources).ShouldNot(BeEmpty())

			By("Checking template labels of updated invalid CR")
			deployedStatefulSet = deployedResources[statefulSetType][0].(*appsv1.StatefulSet)
			Expect(deployedStatefulSet.Spec.Template.Labels["test"]).Should(Equal("test"))

			By("Deleting crds")
			Expect(k8sClient.Delete(ctx, &validCrd)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &invalidCrd)).Should(Succeed())
		})
	})

	Context("Env var updates TRIGGERED_ROLL_COUNT checksum", func() {
		It("Expect TRIGGERED_ROLL_COUNT count non 0", func() {
			By("By creating a new crd")
			var checkSum string
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.AdminUser = "Joe"
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd) }, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("By checking the container stateful set for TRIGGERED_ROLL_COUNT non zero")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == "TRIGGERED_ROLL_COUNT" {
							if env.Value > "0" {
								checkSum = env.Value
								found = true
							}
						}
					}
				}
				return found, err
			}, duration, interval).Should(Equal(true))

			By("update env var")
			Eventually(func() bool {

				err := k8sClient.Get(ctx, types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: crd.ObjectMeta.Namespace}, createdCrd)
				if err == nil {

					createdCrd.Spec.AdminUser = "Joseph"

					err = k8sClient.Update(ctx, createdCrd)
					if err != nil {
						fmt.Printf("error on update! %v\n", err)
					}
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("verify different checksum")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == "TRIGGERED_ROLL_COUNT" {
							if env.Value != checkSum {
								found = true
							}
						}
					}
				}
				return found, err
			}, duration, interval).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("BrokerProperties", func() {
		It("Expect vol mount via config map", func() {
			By("By creating a new crd with BrokerProperties in the spec")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.BrokerProperties = []string{"globalMaxSize=512m"}

			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			Eventually(func() bool { return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd) }, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			hexShaOriginal := HexShaHashOfMap(crd.Spec.BrokerProperties)

			By("By finding a new config map with broker props")
			configMap := &corev1.ConfigMap{}
			key := types.NamespacedName{Name: crd.ObjectMeta.Name + "-props-" + hexShaOriginal, Namespace: crd.ObjectMeta.Namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, configMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By checking the container stateful set for java opts")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
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
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.Containers {
					for _, vm := range container.VolumeMounts {
						// mount path can't have a .
						if strings.Contains(vm.MountPath, "-props-") {
							found = true
						}
					}
				}

				return found, err
			}, duration, interval).Should(Equal(true))

			By("By checking the container stateful launch set for STATEFUL_SET_ORDINAL")
			Eventually(func() (bool, error) {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
				createdSs := &appsv1.StatefulSet{}

				err := k8sClient.Get(ctx, key, createdSs)
				if err != nil {
					return false, err
				}

				found := false
				for _, container := range createdSs.Spec.Template.Spec.Containers {
					for _, command := range container.Command {
						if strings.Contains(command, "STATEFUL_SET_ORDINAL") {
							found = true
						}
					}
				}

				return found, err
			}, duration, interval).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})

		It("Expect new config map on update to BrokerProperties", func() {
			By("By creating a crd with BrokerProperties in the spec")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.BrokerProperties = []string{"globalMaxSize=64g"}
			hexShaOriginal := HexShaHashOfMap(crd.Spec.BrokerProperties)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("By eventualy finding a matching config map with broker props")
			configMapList := &corev1.ConfigMapList{}
			opts := &client.ListOptions{
				Namespace: defaultNamespace,
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
					createdCrd.Spec.BrokerProperties = append(createdCrd.Spec.BrokerProperties, "gen="+strconv.FormatInt(createdCrd.ObjectMeta.Generation, 10))

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
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

			// cannot verify no leaks b/c gc is not enabled on envTest
			// and on delete we don't have any state to determine the owner reference
		})

		It("Expect two crs to coexist", func() {
			By("By creating two crds with BrokerProperties in the spec")
			ctx := context.Background()
			crd1 := generateArtemisSpec(defaultNamespace)
			crd2 := generateArtemisSpec(defaultNamespace)

			Expect(k8sClient.Create(ctx, &crd1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &crd2)).Should(Succeed())

			By("By eventualy finding two config maps with broker props")
			configMapList := &corev1.ConfigMapList{}
			opts := &client.ListOptions{
				Namespace: defaultNamespace,
			}
			Eventually(func() int {
				err := k8sClient.List(ctx, configMapList, opts)
				if err != nil {
					fmt.Printf("error getting list of config opts map! %v", err)
				}

				ret := 0
				for _, cm := range configMapList.Items {
					if strings.Contains(cm.ObjectMeta.Name, crd1.Name) || strings.Contains(cm.ObjectMeta.Name, crd2.Name) {
						ret++
					}
				}
				return ret
			}, timeout, interval).Should(BeEquivalentTo(2))

			// cleanup
			Expect(k8sClient.Delete(ctx, &crd1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &crd2)).Should(Succeed())
		})

		It("Expect two crs to coexist", func() {
			By("By creating two crds with BrokerProperties in the spec")
			ctx := context.Background()
			crd1 := generateArtemisSpec(defaultNamespace)
			crd2 := generateArtemisSpec(defaultNamespace)

			Expect(k8sClient.Create(ctx, &crd1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &crd2)).Should(Succeed())

			By("By eventualy finding two config maps with broker props")
			configMapList := &corev1.ConfigMapList{}
			opts := &client.ListOptions{
				Namespace: defaultNamespace,
			}
			Eventually(func() int {
				err := k8sClient.List(ctx, configMapList, opts)
				if err != nil {
					fmt.Printf("error getting list of configopts map! %v", err)
				}

				ret := 0
				for _, cm := range configMapList.Items {
					if strings.Contains(cm.ObjectMeta.Name, crd1.Name) || strings.Contains(cm.ObjectMeta.Name, crd2.Name) {
						ret++
					}
				}
				return ret
			}, timeout, interval).Should(BeEquivalentTo(2))

			// cleanup
			Expect(k8sClient.Delete(ctx, &crd1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &crd2)).Should(Succeed())
		})

	})

	Context("With address settings via updated cr", func() {
		It("Expect ok deploy", func() {
			By("By creating a crd without address spec")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.BrokerProperties = []string{"globalMaxSize=65g"}

			hexShaOriginal := HexShaHashOfMap(crd.Spec.BrokerProperties)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("By eventualy finding a matching config map with broker props")
			configMapList := &corev1.ConfigMapList{}
			opts := &client.ListOptions{
				Namespace: defaultNamespace,
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

			By("updating the crd with address settings")
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			By("pushing the update on the current version...")
			Eventually(func() bool {

				err := k8sClient.Get(ctx, types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: crd.ObjectMeta.Namespace}, createdCrd)
				if err == nil {

					// add a new property
					createdCrd.Spec.BrokerProperties = append(createdCrd.Spec.BrokerProperties, "gen="+strconv.FormatInt(createdCrd.ObjectMeta.Generation, 10))

					// add address settings, to an existing crd
					ma := "merge_all"
					dlq := "dlq"
					dlqabc := "dlqabc"
					maxSize := "10m"

					createdCrd.Spec.AddressSettings = brokerv1beta1.AddressSettingsType{
						ApplyRule: &ma,
						AddressSetting: []brokerv1beta1.AddressSettingType{
							{
								Match:             "#",
								DeadLetterAddress: &dlq,
							},
							{
								Match:             "abc#",
								DeadLetterAddress: &dlqabc,
								MaxSizeBytes:      &maxSize,
							},
						},
					}

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

			By("tracking the yaconfig init command with user_address_settings and verifying no change on further update")
			key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
			createdSs := &appsv1.StatefulSet{}
			var initArgsString string
			Eventually(func(g Gomega) {

				createdSs := &appsv1.StatefulSet{}

				g.Expect(k8sClient.Get(ctx, key, createdSs))

				initArgsString = strings.Join(createdSs.Spec.Template.Spec.InitContainers[0].Args, ",")
				g.Expect(initArgsString).Should(ContainSubstring("user_address_settings"))

			}, timeout, interval).Should(Succeed())

			By("pushing another update on the current version...")
			Eventually(func() bool {

				err := k8sClient.Get(ctx, types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: crd.ObjectMeta.Namespace}, createdCrd)
				if err == nil {

					createdCrd.Spec.BrokerProperties = append(createdCrd.Spec.BrokerProperties, "gen2="+strconv.FormatInt(createdCrd.ObjectMeta.Generation, 10))

					// add address settings, to an existing crd
					ma := "merge_all"
					dlq := "dlq"
					dlqabc := "dlqabc"
					maxSize := "10m"

					createdCrd.Spec.AddressSettings = brokerv1beta1.AddressSettingsType{
						ApplyRule: &ma,
						AddressSetting: []brokerv1beta1.AddressSettingType{
							{
								Match:             "#",
								DeadLetterAddress: &dlq,
							},
							{
								Match:             "abc#",
								DeadLetterAddress: &dlqabc,
								MaxSizeBytes:      &maxSize,
							},
						},
					}

					err = k8sClient.Update(ctx, createdCrd)
					if err != nil {
						fmt.Printf("error on update! %v\n", err)
					}
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			hexShaModified = HexShaHashOfMap(createdCrd.Spec.BrokerProperties)

			By("again finding the updated config map using the sha")

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

			By("verifying init command args did not change")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, createdSs); err != nil {
					return false
				}

				updatedInitArgs := strings.Join(createdSs.Spec.Template.Spec.InitContainers[0].Args, ",")
				return initArgsString == updatedInitArgs
			}, timeout, interval).Should(BeTrue())

			// cleanup
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("With address cr", func() {
		It("Expect ok deploy", func() {
			By("By creating a crd without  address spec")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			By("By deploying address cr for this namespace but not for this CR")

			addressCrd := brokerv1beta1.ActiveMQArtemisAddress{}
			addressCrd.SetName("a1")
			addressCrd.SetNamespace(defaultNamespace)
			addressCrd.Spec.AddressName = "a1"
			routingTypeShouldBeOptional := "MULTICAST"
			addressCrd.Spec.RoutingType = &routingTypeShouldBeOptional
			addressCrd.Spec.ApplyToCrNames = []string{"bong"}

			Expect(k8sClient.Create(ctx, &addressCrd)).Should(Succeed())

			By("Checking stopped status of CR")
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: defaultNamespace}
				g.Expect(k8sClient.Get(ctx, key, createdCrd)).Should(Succeed())

				g.Expect(len(createdCrd.Status.PodStatus.Stopped)).Should(Equal(1))
			}, timeout, interval).Should(Succeed())

			By("adding an address via update")
			var updatedVersion = "bla"
			key := types.NamespacedName{Name: addressCrd.ObjectMeta.Name, Namespace: defaultNamespace}
			createdAddressCr := &brokerv1beta1.ActiveMQArtemisAddress{}
			Eventually(func(g Gomega) {

				g.Expect(k8sClient.Get(ctx, key, createdAddressCr)).Should(Succeed())

				// ensure a1 applies to this cr
				createdAddressCr.Spec.ApplyToCrNames = append(createdAddressCr.Spec.ApplyToCrNames, createdCrd.Name)
				updatedVersion = createdAddressCr.ResourceVersion
				g.Expect(k8sClient.Update(ctx, createdAddressCr)).Should(Succeed())

			}, timeout, interval).Should(Succeed())

			// cr gets rconciled, but nothing done yet till pods create

			Eventually(func(g Gomega) {
				By("verify update of resource version: " + updatedVersion)
				g.Expect(k8sClient.Get(ctx, key, createdAddressCr)).Should(Succeed())

				g.Expect(createdAddressCr.ResourceVersion).ShouldNot(Equal(updatedVersion))

			}, timeout, interval).Should(Succeed())

			// cleanup
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, createdAddressCr)).Should(Succeed())
		})
	})

	Context("With toggle persistence=true", func() {
		It("Expect ok redeploy", func() {
			By("By creating a crd without persistence")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.DeploymentPlan.PersistenceEnabled = false

			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("By eventualy finding a matching config map with broker props")
			createdSs := &appsv1.StatefulSet{}

			By("Making sure that the ss gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				return err == nil
			}, timeout, interval).Should(Equal(true))

			initialVersion := createdSs.ObjectMeta.ResourceVersion

			By("updating the crd for persistence")
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {

				err := k8sClient.Get(ctx, types.NamespacedName{Name: crd.ObjectMeta.Name, Namespace: crd.ObjectMeta.Namespace}, createdCrd)
				if err == nil {

					createdCrd.Spec.DeploymentPlan.PersistenceEnabled = true
					err = k8sClient.Update(ctx, createdCrd)
					if err != nil {
						fmt.Printf("error on update! %v\n", err)
					}
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Making sure that the ss gets redeployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}

				err := k8sClient.Get(ctx, key, createdSs)

				if err == nil {
					// verify persisted with a new revision, update would have failed
					return createdSs.ObjectMeta.ResourceVersion > "0" && initialVersion != createdSs.ObjectMeta.ResourceVersion
				}
				return err == nil
			}, timeout, interval).Should(Equal(true))

			// cleanup
			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			By("check it has gone")
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("With delopyed controller - acceptor", func() {
		It("Checking acceptor service while expose is false", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

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
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-acceptor-0-svc", Namespace: defaultNamespace}
				acceptorService := &corev1.Service{}
				err := k8sClient.Get(context.Background(), key, acceptorService)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-connector-0-svc", Namespace: defaultNamespace}
				connectorService := &corev1.Service{}
				err := k8sClient.Get(context.Background(), key, connectorService)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("updating with expose=true")
			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				if err == nil {
					crd.Spec.Acceptors[0].Expose = true
					crd.Spec.Connectors[0].Expose = true

					err = k8sClient.Update(ctx, &crd)
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-acceptor-0-svc-ing", Namespace: defaultNamespace}
				exposure := &netv1.Ingress{}
				err := k8sClient.Get(context.Background(), key, exposure)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name + "-" + "new-connector-0-svc-ing", Namespace: defaultNamespace}
				exposure := &netv1.Ingress{}
				err := k8sClient.Get(context.Background(), key, exposure)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
	})

	Context("With deployed controller", func() {
		It("Testing acceptor bindToAllInterfaces default", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name: "new-acceptor",
					Port: 61666,
				},
			}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				g.Expect(len(currentStatefulSet.Spec.Template.Spec.InitContainers)).Should(BeEquivalentTo(1))
				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]

				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						secret, err := secrets.RetriveSecret(namespaceName, secretName, make(map[string]string), k8sClient)
						g.Expect(err).To(BeNil())
						data := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
						//the value is a string of acceptors in xml format:
						//<acceptor name="new-acceptor">...</acceptor><another one>...
						//we need to locate our target acceptor and do the check
						//we use the port as a clue
						g.Expect(strings.Contains(string(data), "ACCEPTOR_IP:61666")).To(BeTrue())
					}
				}
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
		It("Testing acceptor bindToAllInterfaces being false", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			bindTo := false
			crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:                "new-acceptor",
					Port:                61666,
					BindToAllInterfaces: &bindTo,
				},
			}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}

			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				g.Expect(len(currentStatefulSet.Spec.Template.Spec.InitContainers)).Should(BeEquivalentTo(1))
				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]

				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						secret, err := secrets.RetriveSecret(namespaceName, secretName, make(map[string]string), k8sClient)
						g.Expect(err).To(BeNil())
						data := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
						//the value is a string of acceptors in xml format:
						//<acceptor name="new-acceptor">...</acceptor><another one>...
						//we need to locate our target acceptor and do the check
						//we use the port as a clue
						g.Expect(strings.Contains(string(data), "ACCEPTOR_IP:61666")).To(BeTrue())
					}
				}
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
		It("Testing acceptor bindToAllInterfaces being true", func() {
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			bindToAll := true
			notbindToAll := false
			crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:                "new-acceptor",
					Port:                61666,
					BindToAllInterfaces: &bindToAll,
				},
				{
					Name: "new-acceptor-1",
					Port: 61777,
				},
				{
					Name:                "new-acceptor-2",
					Port:                61888,
					BindToAllInterfaces: &notbindToAll,
				},
			}
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}

			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				g.Expect(len(currentStatefulSet.Spec.Template.Spec.InitContainers)).Should(BeEquivalentTo(1))
				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]
				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						secret, err := secrets.RetriveSecret(namespaceName, secretName, make(map[string]string), k8sClient)
						g.Expect(err).To(BeNil())
						data := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
						//the value is a string of acceptors in xml format:
						//<acceptor name="new-acceptor">...</acceptor><another one>...
						//we need to locate our target acceptor and do the check
						//we use the port as a clue
						g.Expect(strings.Contains(string(data), "0.0.0.0:61666")).To(BeTrue())
						//the other one not affected
						g.Expect(strings.Contains(string(data), "ACCEPTOR_IP:61777")).To(BeTrue())
						g.Expect(strings.Contains(string(data), "ACCEPTOR_IP:61888")).To(BeTrue())
					}
				}

			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
	})

	Context("With a deployed controller", func() {
		//TODO: Remove the 4x duplication and add all acceptor settings

		It("Testing acceptor keyStoreProvider being set", func() {
			By("By creating a new custom resource instance")
			ctx := context.Background()
			cr := generateArtemisSpec(defaultNamespace)

			cr.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			keyStoreProvider := "SunJCE"
			cr.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:             "new-acceptor",
					Port:             61666,
					SSLEnabled:       true,
					KeyStoreProvider: keyStoreProvider,
				},
			}
			Expect(k8sClient.Create(ctx, &cr)).Should(Succeed())

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(cr.Name), Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				len := len(currentStatefulSet.Spec.Template.Spec.InitContainers)
				g.Expect(len).Should(BeEquivalentTo(1))

				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]
				By("check AMQ_ACCEPTORS value")
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						secret, err := secrets.RetriveSecret(namespaceName, secretName, make(map[string]string), k8sClient)
						g.Expect(err).To(BeNil())
						data := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
						By("Checking data:" + string(data))
						g.Expect(strings.Contains(string(data), "ACCEPTOR_IP:61666")).To(BeTrue())
						checkSecretHasCorrectKeyValue(g, secretName, namespaceName, envVar.ValueFrom.SecretKeyRef.Key, "keyStoreProvider=SunJCE")
					}
				}

			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &cr)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(cr.Name, defaultNamespace, &cr), timeout, interval).Should(BeTrue())
		})
		It("Testing acceptor trustStoreType being set and unset", func() {
			By("By creating a new custom resource instance")
			ctx := context.Background()
			cr := generateArtemisSpec(defaultNamespace)

			cr.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			trustStoreType := "JCEKS"
			cr.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:           "new-acceptor",
					Port:           61666,
					SSLEnabled:     true,
					TrustStoreType: trustStoreType,
				},
			}
			Expect(k8sClient.Create(ctx, &cr)).Should(Succeed())

			ssResourceVersionWithSslEnabled := ""

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(cr.Name), Namespace: defaultNamespace}
			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				len := len(currentStatefulSet.Spec.Template.Spec.InitContainers)
				g.Expect(len).Should(Equal(1))

				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]

				found := false
				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						//the value is a string of acceptors in xml format:
						//<acceptor name="new-acceptor">...</acceptor><another one>...
						//we need to locate our target acceptor and do the check
						//we use the port as a clue
						checkSecretHasCorrectKeyValue(g, secretName, namespaceName, envVar.ValueFrom.SecretKeyRef.Key, "trustStoreType=JCEKS")
						found = true
					}
				}
				g.Expect(found).Should(BeTrue())
				ssResourceVersionWithSslEnabled = currentStatefulSet.ResourceVersion

			}, timeout, interval).Should(Succeed())

			By("test Updating the CR back to sslEnabled=false")
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}
			Eventually(func(g Gomega) {

				g.Expect(getPersistedVersionedCrd(cr.ObjectMeta.Name, defaultNamespace, createdCrd)).Should(BeTrue())

				createdCrd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
					{
						Name:           "new-acceptor",
						Port:           61666,
						SSLEnabled:     false,
						TrustStoreType: trustStoreType,
					},
				}

				By("Redeploying the CRD")
				g.Expect(k8sClient.Update(ctx, createdCrd)).Should(Succeed())

			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				len := len(currentStatefulSet.Spec.Template.Spec.InitContainers)
				g.Expect(len).Should(Equal(1))

				g.Expect(currentStatefulSet.ResourceVersion).ShouldNot(Equal(ssResourceVersionWithSslEnabled))

				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]
				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}

						secret, err := secrets.RetriveSecret(namespaceName, secretName, make(map[string]string), k8sClient)
						g.Expect(err).Should(BeNil())

						data := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
						g.Expect(string(data)).ShouldNot(ContainSubstring("trustStoreType=JCEKS"))
					}
				}
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &cr)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(cr.Name, defaultNamespace, &cr), timeout, interval).Should(BeTrue())
		})
		It("Testing acceptor trustStoreProvider being set", func() {
			By("By creating a new custom resource instance")
			ctx := context.Background()
			cr := generateArtemisSpec(defaultNamespace)

			cr.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size: 1,
			}
			trustStoreProvider := "SUN"
			cr.Spec.Acceptors = []brokerv1beta1.AcceptorType{
				{
					Name:               "new-acceptor",
					Port:               61666,
					SSLEnabled:         true,
					TrustStoreProvider: trustStoreProvider,
				},
			}
			Expect(k8sClient.Create(ctx, &cr)).Should(Succeed())

			ssNamespacedName := types.NamespacedName{Name: namer.CrToSS(cr.Name), Namespace: defaultNamespace}

			Eventually(func(g Gomega) {
				currentStatefulSet, err := ss.RetrieveStatefulSet(ssNamespacedName.Name, ssNamespacedName, nil, k8sClient)
				g.Expect(err).To(BeNil())
				len := len(currentStatefulSet.Spec.Template.Spec.InitContainers)
				g.Expect(len).Should(Equal(1))

				initContainer := currentStatefulSet.Spec.Template.Spec.InitContainers[0]
				//check AMQ_ACCEPTORS value
				for _, envVar := range initContainer.Env {
					if envVar.Name == "AMQ_ACCEPTORS" {
						secretName := envVar.ValueFrom.SecretKeyRef.Name
						namespaceName := types.NamespacedName{
							Name:      secretName,
							Namespace: defaultNamespace,
						}
						//the value is a string of acceptors in xml format:
						//<acceptor name="new-acceptor">...</acceptor><another one>...
						//we need to locate our target acceptor and do the check
						//we use the port as a clue
						checkSecretHasCorrectKeyValue(g, secretName, namespaceName, envVar.ValueFrom.SecretKeyRef.Key, "trustStoreProvider=SUN")
					}
				}
			}, timeout, interval).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &cr)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(cr.Name, defaultNamespace, &cr), timeout, interval).Should(BeTrue())
		})
	})

	Context("With deployed controller", func() {
		It("verify old ver support", func() {
			By("By creating an old crd")
			ctx := context.Background()

			spec := brokerv2alpha4.ActiveMQArtemisSpec{}
			crd := brokerv2alpha4.ActiveMQArtemis{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ActiveMQArtemis",
					APIVersion: brokerv2alpha4.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      randString(),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}

			crd.Spec.DeploymentPlan = brokerv2alpha4.DeploymentPlanType{
				Size:               1,
				PersistenceEnabled: true,
			}

			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			createdSs := &appsv1.StatefulSet{}
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, createdSs)
				return err == nil
			}, timeout, interval).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
	})

	Context("With deployed controller", func() {

		It("Checking storageClassName is configured", func() {

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				By("By not requesting PVC that cannot be bound, seems to prevent minkube auto pv allocation")
				return
			}
			By("By creating a new crd")
			ctx := context.Background()
			crd := generateArtemisSpec(defaultNamespace)

			crd.Spec.DeploymentPlan = brokerv1beta1.DeploymentPlanType{
				Size:               1,
				PersistenceEnabled: true,
				Storage: brokerv1beta1.StorageType{
					StorageClassName: "some-storage-class",
				},
			}

			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			Eventually(func() bool {
				key := types.NamespacedName{Name: crd.Name, Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, &crd)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			createdSs := &appsv1.StatefulSet{}
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, createdSs)
				return err == nil
			}, timeout, interval).Should(Equal(true))

			volumeTemplates := createdSs.Spec.VolumeClaimTemplates
			Expect(len(volumeTemplates)).To(Equal(1))

			storageClassName := volumeTemplates[0].Spec.StorageClassName
			Expect(*storageClassName).To(Equal("some-storage-class"))

			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())

			By("check it has gone")
			Eventually(checkCrdDeleted(crd.Name, defaultNamespace, &crd), timeout, interval).Should(BeTrue())
		})
	})

	It("populateValidatedUser", func() {

		ctx := context.Background()
		crd := generateArtemisSpec(defaultNamespace)
		crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 1,
			PeriodSeconds:       5,
		}
		crd.Spec.DeploymentPlan.Size = 1
		crd.Spec.DeploymentPlan.RequireLogin = true
		crd.Spec.BrokerProperties = []string{
			"securityEnabled=true",
			"rejectEmptyValidatedUser=true",
			"populateValidatedUser=true",
		}

		propLoginModules := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
		pwd := "activemq"
		user := "Jay"
		moduleName := "prop-module"
		flag := "sufficient"
		propLoginModules[0] = brokerv1beta1.PropertiesLoginModuleType{
			Name: moduleName,
			Users: []brokerv1beta1.UserType{
				{Name: user,
					Password: &pwd,
					Roles:    []string{"jay-role"}},
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

		if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

			By("Deploying security spec")
			_, deployedSecCrd := DeploySecurity("for-jay", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {
				secCrdToDeploy.Spec.LoginModules.PropertiesLoginModules = propLoginModules
				secCrdToDeploy.Spec.SecurityDomains.BrokerDomain = brokerDomain
				secCrdToDeploy.Spec.SecuritySettings.Broker =
					[]brokerv1beta1.BrokerSecuritySettingType{
						{Match: "#",
							Permissions: []brokerv1beta1.PermissionType{
								{OperationType: "send", Roles: []string{"jay-role"}},
								{OperationType: "browse", Roles: []string{"jay-role"}},
							},
						},
					}
			})

			By("Deploying a broker")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Checking ready on SS")
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
				sfsFound := &appsv1.StatefulSet{}

				g.Expect(k8sClient.Get(ctx, key, sfsFound)).Should(Succeed())
				g.Expect(sfsFound.Status.ReadyReplicas).Should(BeEquivalentTo(1))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Sending 1")

			podWithOrdinal := namer.CrToSS(crd.Name) + "-0"
			command := []string{"amq-broker/bin/artemis", "producer", "--user", user, "--password", pwd, "--url", "tcp://" + podWithOrdinal + ":61616", "--message-count", "1", "--destination", "queue://DLQ", "--verbose"}

			Eventually(func(g Gomega) {
				stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)
				g.Expect(stdOutContent).Should(ContainSubstring("Produced: 1 messages"))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Consuming 1")

			command = []string{"amq-broker/bin/artemis", "browser", "--user", user, "--password", pwd, "--url", "tcp://" + podWithOrdinal + ":61616", "--destination", "queue://DLQ", "--verbose"}

			Eventually(func(g Gomega) {
				stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)

				Expect(stdOutContent).Should(ContainSubstring("messageID="))
				Expect(stdOutContent).Should(ContainSubstring("_AMQ_VALIDATED_USER=" + user))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			// cleanup
			Expect(k8sClient.Delete(ctx, &crd)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployedSecCrd)).Should(Succeed())

		}

	})

	It("populateValidatedUser as auto generated guest", func() {

		ctx := context.Background()
		crd := generateArtemisSpec(defaultNamespace)
		crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 1,
			PeriodSeconds:       5,
		}
		crd.Spec.DeploymentPlan.Size = 1
		crd.Spec.BrokerProperties = []string{
			"securityEnabled=true",
			"rejectEmptyValidatedUser=true",
			"populateValidatedUser=true",
		}

		if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

			By("Deploying a broker")
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			By("Checking ready on SS")
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
				sfsFound := &appsv1.StatefulSet{}

				g.Expect(k8sClient.Get(ctx, key, sfsFound)).Should(Succeed())
				g.Expect(sfsFound.Status.ReadyReplicas).Should(BeEquivalentTo(1))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Sending 1")

			podWithOrdinal := namer.CrToSS(crd.Name) + "-0"
			command := []string{"amq-broker/bin/artemis", "producer", "--user", "Jay", "--password", "activemq", "--url", "tcp://" + podWithOrdinal + ":61616", "--message-count", "1", "--destination", "queue://DLQ", "--verbose"}

			Eventually(func(g Gomega) {
				stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)
				g.Expect(stdOutContent).Should(ContainSubstring("Produced: 1 messages"))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Consuming 1")

			command = []string{"amq-broker/bin/artemis", "browser", "--user", "Jay", "--password", "activemq", "--url", "tcp://" + podWithOrdinal + ":61616", "--destination", "queue://DLQ", "--verbose"}

			Eventually(func(g Gomega) {
				stdOutContent := execOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)

				// ...ActiveMQMessage[ID:3a356c2a-e7e1-11ec-ae1c-5ec4168c91b2]:PERSISTENT/ClientMessageImpl[messageID=19, durable=true, address=DLQ,userID=3a356c2a-e7e1-11ec-ae1c-5ec4168c91b2,properties=TypedProperties[__AMQ_CID=3a2f9fc7-e7e1-11ec-ae1c-5ec4168c91b2,_AMQ_ROUTING_TYPE=1,_AMQ_VALIDATED_USER=73ykuMrb,count=0,ThreadSent=Producer ActiveMQQueue[DLQ], thread=0]]
				Expect(stdOutContent).Should(ContainSubstring("messageID="))
				Expect(stdOutContent).Should(ContainSubstring("_AMQ_VALIDATED_USER="))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			// cleanup
			k8sClient.Delete(ctx, &crd)

		}
	})

	It("deploy security cr while broker is not yet ready", func() {

		By("Creating broker with custom probe that relies on security")
		ctx := context.Background()
		randString := randStringWithPrefix("")
		crd := generateOriginalArtemisSpec(defaultNamespace, "broker-"+randString)

		crd.Spec.AdminUser = "admin"
		crd.Spec.AdminPassword = "secret"
		crd.Spec.DeploymentPlan.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/home/jboss/amq-broker/bin/artemis",
						"check",
						"queue",
						"--name",
						"readinessqueue",
						"--produce",
						"1",
						"--consume",
						"1",
						"--silent",
						"--user",
						"user1",
						"--password",
						"ok",
					},
				},
			},
			InitialDelaySeconds: 5,
			TimeoutSeconds:      5,
			PeriodSeconds:       5,
		}
		crd.Spec.DeploymentPlan.Size = 1
		crd.Spec.DeploymentPlan.Clustered = &boolFalse
		crd.Spec.DeploymentPlan.RequireLogin = boolTrue
		crd.Spec.DeploymentPlan.JolokiaAgentEnabled = false
		crd.Spec.Acceptors = []brokerv1beta1.AcceptorType{
			{
				Name:      "internal",
				Protocols: "core,openwire",
				Port:      61616,
			},
		}

		By("Deploying the CRD " + crd.ObjectMeta.Name)
		Expect(k8sClient.Create(ctx, crd)).Should(Succeed())

		createdCrd := &brokerv1beta1.ActiveMQArtemis{}
		createdSs := &appsv1.StatefulSet{}

		By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
		Eventually(func() bool {
			return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())
		Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

		ssKey := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
		By("Checking that Stateful Set is Created " + ssKey.Name)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, ssKey, createdSs)).Should(Succeed())
			By("Checking ss " + ssKey.Name + " resource version" + createdSs.ResourceVersion)
			g.Expect(createdSs.ResourceVersion).ShouldNot(BeNil())
		}, timeout, interval).Should(Succeed())

		brokerKey := types.NamespacedName{Name: createdCrd.Name, Namespace: createdCrd.Namespace}
		if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

			By("verifying not yet ready status - probe failed")
			Eventually(func(g Gomega) {

				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
				By("verify starting status" + fmt.Sprintf("%v", createdCrd.Status.PodStatus))
				g.Expect(len(createdCrd.Status.PodStatus.Starting)).Should(BeEquivalentTo(1))

			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
		}

		By("Deploying security")
		secCrd, createdSecCrd := DeploySecurity("sec-"+randString, defaultNamespace,
			func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {
				secCrdToDeploy.Spec.ApplyToCrNames = []string{createdCrd.Name}
			})

		By("Checking that Stateful Set is updated with sec commands, " + ssKey.Name)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, ssKey, createdSs)).Should(Succeed())

			initContainer := createdSs.Spec.Template.Spec.InitContainers[0]
			securityFound := false
			By("Checking init container args on ss " + ssKey.Name + " resource version" + createdSs.ResourceVersion)

			for _, argStr := range initContainer.Args {
				if strings.Contains(argStr, "/opt/amq-broker/script/cfg/config-security.sh") {
					securityFound = true
					break
				}
			}
			g.Expect(securityFound).Should(BeTrue())
		}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

		if os.Getenv("USE_EXISTING_CLUSTER") == "true" {

			// looks like we must force a rollback of the old ss rollout as it won't complete
			// due to the failure of the readiness probe
			// https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback

			Eventually(func(g Gomega) {
				By("deleting existing pod that is stuck on rollout..")
				pod := &corev1.Pod{}
				key := types.NamespacedName{Name: namer.CrToSS(crd.Name) + "-0", Namespace: defaultNamespace}
				zeroGracePeriodSeconds := int64(0) // immediate delete
				g.Expect(k8sClient.Get(ctx, key, pod)).Should(Succeed())
				g.Expect(pod.Status.Phase).Should(Equal(corev1.PodRunning))
				foundReadyFalse := false
				for _, pc := range pod.Status.Conditions {
					if pc.Type == corev1.ContainersReady && pc.Status == corev1.ConditionFalse {
						foundReadyFalse = true
					}
				}
				g.Expect(foundReadyFalse).Should(BeTrue())
				By("Deleting pod: " + key.Name)
				g.Expect(k8sClient.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: &zeroGracePeriodSeconds})).Should(Succeed())
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("verifying ready with new SS rollout")
			Eventually(func(g Gomega) {

				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
				By("verify ready status" + fmt.Sprintf("%v", createdCrd.Status.PodStatus))
				g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
		}

		By("check it has gone")
		Expect(k8sClient.Delete(ctx, createdCrd)).To(Succeed())
		Eventually(func() bool {
			return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())

		Expect(k8sClient.Delete(ctx, createdSecCrd)).To(Succeed())
		Eventually(func() bool {
			return checkCrdDeleted(secCrd.Name, defaultNamespace, createdSecCrd)
		}, timeout, interval).Should(BeTrue())
	})

	It("managementRBACEnabled is false", func() {

		ctx := context.Background()
		crd := generateArtemisSpec(defaultNamespace)
		crd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 1,
			PeriodSeconds:       5,
		}
		crd.Spec.DeploymentPlan.Size = 1
		crd.Spec.DeploymentPlan.ManagementRBACEnabled = false

		By("Deploying the CRD " + crd.ObjectMeta.Name)
		Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

		createdCrd := &brokerv1beta1.ActiveMQArtemis{}
		createdSs := &appsv1.StatefulSet{}

		By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
		Eventually(func() bool {
			return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())
		Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

		ssKey := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
		By("Checking that Stateful Set is Created " + ssKey.Name)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, ssKey, createdSs)).Should(Succeed())
			By("Checking ss resource version" + createdSs.ResourceVersion)
			g.Expect(createdSs.ResourceVersion).ShouldNot(BeNil())
		}, timeout, interval).Should(Succeed())

		By("By checking the container stateful set for AMQ_ENABLE_MANAGEMENT_RBAC")
		Eventually(func() (bool, error) {
			err := k8sClient.Get(ctx, ssKey, createdSs)
			if err != nil {
				return false, err
			}

			found := false
			for _, container := range createdSs.Spec.Template.Spec.InitContainers {
				for _, env := range container.Env {
					if env.Name == "AMQ_ENABLE_MANAGEMENT_RBAC" {
						if strings.Contains(env.Value, "false") {
							found = true
						}
					}
				}
			}

			return found, err
		}, duration, interval).Should(Equal(true))

		By("By checking it has gone")
		Expect(k8sClient.Delete(ctx, createdCrd)).To(Succeed())
		Eventually(func() bool {
			return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())
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

func DeployBroker(brokerName string, targetNamespace string) (*brokerv1beta1.ActiveMQArtemis, *brokerv1beta1.ActiveMQArtemis) {
	ctx := context.Background()
	brokerCrd := generateOriginalArtemisSpec(targetNamespace, brokerName)

	Expect(k8sClient.Create(ctx, brokerCrd)).Should(Succeed())

	createdBrokerCrd := &brokerv1beta1.ActiveMQArtemis{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, targetNamespace, createdBrokerCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdBrokerCrd.Name).Should(Equal(createdBrokerCrd.ObjectMeta.Name))

	return brokerCrd, createdBrokerCrd

}

func randStringWithPrefix(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	length := 6
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func randString() string {
	return randStringWithPrefix("broker-")
}

func getPersistedVersionedCrd(name string, nameSpace string, object client.Object) bool {
	key := types.NamespacedName{Name: name, Namespace: nameSpace}
	err := k8sClient.Get(ctx, key, object)
	return err == nil
}

func checkCrdDeleted(name string, namespace string, crd client.Object) bool {
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, crd)
	return errors.IsNotFound(err)
}

func checkSecretHasCorrectKeyValue(g Gomega, secName string, ns types.NamespacedName, key string, expectedValue string) {
	g.Eventually(func(g Gomega) {
		secret, err := secrets.RetriveSecret(ns, secName, make(map[string]string), k8sClient)
		g.Expect(err).Should(BeNil())
		data := secret.Data[key]
		g.Expect(strings.Contains(string(data), expectedValue)).Should(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func DeployCustomBroker(crName string, targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemis)) (*brokerv1beta1.ActiveMQArtemis, *brokerv1beta1.ActiveMQArtemis) {
	ctx := context.Background()
	brokerCrd := generateArtemisSpec(targetNamespace)
	if crName != "" {
		brokerCrd.Name = crName
	}

	brokerCrd.Spec.DeploymentPlan.Size = 1

	if customFunc != nil {
		customFunc(&brokerCrd)
	}

	Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())

	createdBrokerCrd := brokerv1beta1.ActiveMQArtemis{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(brokerCrd.Name, targetNamespace, &createdBrokerCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdBrokerCrd.Name).Should(Equal(brokerCrd.ObjectMeta.Name))
	Expect(createdBrokerCrd.Namespace).Should(Equal(targetNamespace))

	return &brokerCrd, &createdBrokerCrd
}
