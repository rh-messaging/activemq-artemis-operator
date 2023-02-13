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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var boolFalse = false
var boolTrue = true

var _ = Describe("security controller", func() {

	BeforeEach(func() {
		BeforeEachSpec()

		if verbose {
			fmt.Println("Time with MicroSeconds: ", time.Now().Format("2006-01-02 15:04:05.000000"), " test:", CurrentSpecReport())
		}
	})

	AfterEach(func() {
	})

	Context("broker with security custom resources", Label("broker-security-res"), func() {

		It("security after recreating broker cr", func() {

			By("deploy a security cr")
			securityCr, createdSecurityCr := DeploySecurity(NextSpecResourceName(), defaultNamespace, func(candidate *brokerv1beta1.ActiveMQArtemisSecurity) {
			})

			By("deploy a broker cr")
			brokerCr, createdBrokerCr := DeployCustomBroker(defaultNamespace, nil)

			By("checking the security gets applied")
			requestedSs := &appsv1.StatefulSet{}
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBrokerCr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}

				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied
			}, timeout, interval).Should(BeTrue())

			By("delete the broker cr")
			Expect(k8sClient.Delete(ctx, createdBrokerCr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(brokerCr.Name, defaultNamespace, createdBrokerCr)
			}, timeout, interval).Should(BeTrue())

			By("re-deploy the broker cr")
			brokerCr, createdBrokerCr = DeployCustomBroker(defaultNamespace, func(candidate *brokerv1beta1.ActiveMQArtemis) {
				candidate.Name = brokerCr.Name
			})

			By("verify the security is re-applied")
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBrokerCr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}

				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied
			}, timeout, interval).Should(BeTrue())

			//cleanup
			Expect(k8sClient.Delete(ctx, createdBrokerCr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(brokerCr.Name, defaultNamespace, createdBrokerCr)
			}, existingClusterTimeout, existingClusterInterval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdSecurityCr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(securityCr.Name, defaultNamespace, createdSecurityCr)
			}, existingClusterTimeout, existingClusterInterval).Should(BeTrue())
		})
	})

	Context("Reconcile Test", func() {
		It("testing security applied after broker", Label("security-apply-restart"), func() {
			By("deploying the broker cr")
			crd, createdCrd := DeployCustomBroker(defaultNamespace, func(brokerCrdToDeploy *brokerv1beta1.ActiveMQArtemis) {

				brokerCrdToDeploy.Spec.Acceptors = []brokerv1beta1.AcceptorType{
					{
						Name:       "all",
						Expose:     true,
						Port:       61616,
						Protocols:  "all",
						SSLEnabled: false,
					},
				}
				brokerCrdToDeploy.Spec.AdminUser = "admin"
				brokerCrdToDeploy.Spec.AdminPassword = "admin"
				brokerCrdToDeploy.Spec.Console.Expose = true
				brokerCrdToDeploy.Spec.DeploymentPlan.Clustered = &boolFalse
				brokerCrdToDeploy.Spec.DeploymentPlan.JolokiaAgentEnabled = true
				brokerCrdToDeploy.Spec.DeploymentPlan.MessageMigration = &boolTrue
				brokerCrdToDeploy.Spec.DeploymentPlan.PersistenceEnabled = true
				brokerCrdToDeploy.Spec.DeploymentPlan.RequireLogin = true
				brokerCrdToDeploy.Spec.DeploymentPlan.Size = common.Int32ToPtr(1)
			})

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				By("make sure the broker is up and running")
				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
					sfsFound := &appsv1.StatefulSet{}

					g.Expect(k8sClient.Get(ctx, key, sfsFound)).Should(Succeed())
					g.Expect(sfsFound.Status.ReadyReplicas).Should(BeEquivalentTo(1))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Checking no security gets applied to broker at the moment" + createdCrd.Name)
				podWithOrdinal := namer.CrToSS(crd.Name) + "-0"
				command := []string{"ls", "amq-broker/etc"}

				Eventually(func(g Gomega) {
					stdOutContent := ExecOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)
					g.Expect(stdOutContent).ShouldNot(ContainSubstring("keycloak"))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}

			By("deploying the security cr")
			secCrd, createdSecCrd := DeploySecurity("ex-keycloak", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {

				brokerModuleName := "keycloak-broker"
				consoleModuleName := "keycloak-console"
				directAccess := "directAccess"
				realm := "artemis-keycloak-demo"
				brokerResource := "artemis-broker"
				brokerAuthUrl := "http://10.111.246.36:8080/auth"
				principalAttribute := "preferred_username"
				external := "external"
				token := "9699685c-8a30-45cf-bf19-0d38bbac5fdc"
				bearerToken := "bearerToken"
				consoleResource := "artemis-console"
				consoleAuthUrl := "http://keycloak.3387.com/auth"
				var confidentialPort int32 = 0

				brokerDomainName := "activemq"
				consoleDomainName := "console"
				requiredFlag := "required"
				mgmtDomain := "org.apache.activemq.artemis"
				listMethod := "list*"
				sendMethod := "sendMessage*"
				browseMethod := "browse*"

				secCrdToDeploy.Spec.LoginModules = brokerv1beta1.LoginModulesType{
					KeycloakLoginModules: []brokerv1beta1.KeycloakLoginModuleType{
						{
							Name:       brokerModuleName,
							ModuleType: &directAccess,
							Configuration: brokerv1beta1.KeycloakModuleConfigurationType{
								Realm:                   &realm,
								Resource:                &brokerResource,
								AuthServerUrl:           &brokerAuthUrl,
								UseResourceRoleMappings: &boolTrue,
								PrincipalAttribute:      &principalAttribute,
								SslRequired:             &external,
								Credentials: []brokerv1beta1.KeyValueType{
									{
										Key:   "secret",
										Value: &token,
									},
								},
							},
						},
						{
							Name:       consoleModuleName,
							ModuleType: &bearerToken,
							Configuration: brokerv1beta1.KeycloakModuleConfigurationType{
								Realm:                   &realm,
								Resource:                &consoleResource,
								AuthServerUrl:           &consoleAuthUrl,
								PrincipalAttribute:      &principalAttribute,
								UseResourceRoleMappings: &boolTrue,
								SslRequired:             &external,
								ConfidentialPort:        &confidentialPort,
							},
						},
					},
				}
				secCrdToDeploy.Spec.SecurityDomains = brokerv1beta1.SecurityDomainsType{
					BrokerDomain: brokerv1beta1.BrokerDomainType{
						Name: &brokerDomainName,
						LoginModules: []brokerv1beta1.LoginModuleReferenceType{
							{
								Name: &brokerModuleName,
								Flag: &requiredFlag,
							},
						},
					},
					ConsoleDomain: brokerv1beta1.BrokerDomainType{
						Name: &consoleDomainName,
						LoginModules: []brokerv1beta1.LoginModuleReferenceType{
							{
								Name: &consoleModuleName,
								Flag: &requiredFlag,
							},
						},
					},
				}
				secCrdToDeploy.Spec.SecuritySettings = brokerv1beta1.SecuritySettingsType{
					Broker: []brokerv1beta1.BrokerSecuritySettingType{
						{
							Match: "Info",
							Permissions: []brokerv1beta1.PermissionType{
								{
									OperationType: "createDurableQueue",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "deleteDurableQueue",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "createNonDurableQueue",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "deleteNonDurableQueue",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "send",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "consume",
									Roles: []string{
										"amq",
									},
								},
							},
						},
						{
							Match: "activemq.management.#",
							Permissions: []brokerv1beta1.PermissionType{
								{
									OperationType: "createNonDurableQueue",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "createAddress",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "consume",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "manage",
									Roles: []string{
										"amq",
									},
								},
								{
									OperationType: "send",
									Roles: []string{
										"amq",
									},
								},
							},
						},
					},
					Management: brokerv1beta1.ManagementSecuritySettingsType{
						HawtioRoles: []string{
							"guest",
						},
						Authorisation: brokerv1beta1.AuthorisationConfigType{
							RoleAccess: []brokerv1beta1.RoleAccessType{
								{
									Domain: &mgmtDomain,
									AccessList: []brokerv1beta1.DefaultAccessType{
										{
											Method: &listMethod,
											Roles: []string{
												"guest",
											},
										},
										{
											Method: &sendMethod,
											Roles: []string{
												"guest",
											},
										},
										{
											Method: &browseMethod,
											Roles: []string{
												"guest",
											},
										},
									},
								},
							},
						},
					},
				}
			})
			By("checking security is applied")
			requestedSs := &appsv1.StatefulSet{}

			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdCrd.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}

				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied
			}, timeout, interval).Should(BeTrue())

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				By("Checking ready on SS")
				Eventually(func(g Gomega) {
					key := types.NamespacedName{Name: namer.CrToSS(crd.Name), Namespace: defaultNamespace}
					sfsFound := &appsv1.StatefulSet{}

					g.Expect(k8sClient.Get(ctx, key, sfsFound)).Should(Succeed())
					g.Expect(sfsFound.Status.ReadyReplicas).Should(BeEquivalentTo(1))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Checking security gets applied to broker " + createdCrd.Name)
				podWithOrdinal := namer.CrToSS(crd.Name) + "-0"
				command := []string{"ls", "amq-broker/etc"}

				Eventually(func(g Gomega) {
					stdOutContent := ExecOnPod(podWithOrdinal, crd.Name, defaultNamespace, command, g)
					g.Expect(stdOutContent).Should(ContainSubstring("keycloak"))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}

			By("checking resources get removed")
			Expect(k8sClient.Delete(ctx, crd)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(crd.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdSecCrd)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(secCrd.ObjectMeta.Name, defaultNamespace, createdSecCrd)
			}, timeout, interval).Should(BeTrue())

		})

		It("reconcile twice with nothing changed", func() {

			By("Creating security cr")
			ctx := context.Background()
			crd := generateSecuritySpec("", defaultNamespace)

			brokerDomainName := "activemq"
			loginModuleName := "module1"
			loginModuleFlag := "sufficient"

			loginModuleList := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
			propLoginModule := brokerv1beta1.PropertiesLoginModuleType{
				Name: loginModuleName,
				Users: []brokerv1beta1.UserType{
					{
						Name:     "user1",
						Password: nil,
						Roles: []string{
							"admin", "amq",
						},
					},
				},
			}
			loginModuleList = append(loginModuleList, propLoginModule)
			crd.Spec.LoginModules.PropertiesLoginModules = loginModuleList

			crd.Spec.SecurityDomains.BrokerDomain = brokerv1beta1.BrokerDomainType{
				Name: &brokerDomainName,
				LoginModules: []brokerv1beta1.LoginModuleReferenceType{
					{
						Name: &loginModuleName,
						Flag: &loginModuleFlag,
					},
				},
			}

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, crd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemisSecurity{}

			By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
			Eventually(func() bool {
				return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())
			Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

			var securityHandler common.ActiveMQArtemisConfigHandler
			Eventually(func() bool {
				securityHandler = GetBrokerConfigHandler(types.NamespacedName{
					Name:      crd.ObjectMeta.Name,
					Namespace: defaultNamespace,
				})
				return securityHandler != nil
			}, timeout, interval).Should(BeTrue())

			realHandler, ok := securityHandler.(*ActiveMQArtemisSecurityConfigHandler)
			Expect(ok).To(BeTrue())

			By("Redeploying the same CR")
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      crd.ObjectMeta.Name,
					Namespace: defaultNamespace,
				},
			}

			result, err := securityReconciler.Reconcile(context.Background(), request)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(common.GetReconcileResyncPeriod()))

			newHandler := GetBrokerConfigHandler(types.NamespacedName{
				Name:      crd.ObjectMeta.Name,
				Namespace: defaultNamespace,
			})
			Expect(newHandler).NotTo(BeNil())

			newRealHandler, ok2 := newHandler.(*ActiveMQArtemisSecurityConfigHandler)
			Expect(ok2).To(BeTrue())

			equal := realHandler == newRealHandler
			Expect(equal).To(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdCrd))
			Eventually(func() bool {
				return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
			}, timeout, interval).Should(BeTrue())

		})

		It("Testing applyToCrNames working properly", func() {

			By("Creating security cr")
			crd := generateSecuritySpec("", defaultNamespace)

			brokerDomainName := "activemq"
			loginModuleName := "module1"
			loginModuleFlag := "sufficient"

			loginModuleList := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
			propLoginModule := brokerv1beta1.PropertiesLoginModuleType{
				Name: loginModuleName,
				Users: []brokerv1beta1.UserType{
					{
						Name:     "user1",
						Password: nil,
						Roles: []string{
							"admin", "amq",
						},
					},
				},
			}
			loginModuleList = append(loginModuleList, propLoginModule)
			crd.Spec.LoginModules.PropertiesLoginModules = loginModuleList

			crd.Spec.SecurityDomains.BrokerDomain = brokerv1beta1.BrokerDomainType{
				Name: &brokerDomainName,
				LoginModules: []brokerv1beta1.LoginModuleReferenceType{
					{
						Name: &loginModuleName,
						Flag: &loginModuleFlag,
					},
				},
			}
			crd.Name = "security"

			defaultbrokerNamespace := defaultNamespace
			broker3Namespace := "broker3-namespace"
			broker4Namespace := "broker4-namespace"
			broker0Name := "broker0"
			broker1Name := "broker1"
			broker2Name := "broker2"
			broker3Name := "broker0"
			broker4Name := "broker0"

			broker0 := types.NamespacedName{
				Name:      broker0Name,
				Namespace: defaultbrokerNamespace,
			}
			broker1 := types.NamespacedName{
				Name:      broker1Name,
				Namespace: defaultbrokerNamespace,
			}
			broker2 := types.NamespacedName{
				Name:      broker2Name,
				Namespace: defaultbrokerNamespace,
			}
			broker3 := types.NamespacedName{
				Name:      broker3Name,
				Namespace: broker3Namespace,
			}
			broker4 := types.NamespacedName{
				Name:      broker4Name,
				Namespace: broker4Namespace,
			}

			secHandler := ActiveMQArtemisSecurityConfigHandler{
				SecurityCR: crd,
				NamespacedName: types.NamespacedName{
					Name:      crd.Name,
					Namespace: defaultNamespace,
				},
				owner: nil,
			}

			By("Default security applies to all in the same namespace but none in others")
			Expect(crd.Spec.ApplyToCrNames).To(BeEmpty())

			Expect(secHandler.IsApplicableFor(broker0)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker1)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker2)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker3)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker4)).To(BeFalse())

			By("ApplyToCrNames being empty applies to all in the same namespace but none in others")
			crd.Spec.ApplyToCrNames = []string{""}
			Expect(crd.Spec.ApplyToCrNames[0]).To(BeEmpty())

			Expect(secHandler.IsApplicableFor(broker0)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker1)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker2)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker3)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker4)).To(BeFalse())

			By("ApplyToCrNames being * applies to all in the same namespace but none in others")
			crd.Spec.ApplyToCrNames = []string{"*"}
			Expect(crd.Spec.ApplyToCrNames[0]).To(Equal("*"))

			Expect(secHandler.IsApplicableFor(broker0)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker1)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker2)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker3)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker4)).To(BeFalse())

			By("ApplyToCrNames being broker0 only applies to broker0 in the same namespace")
			crd.Spec.ApplyToCrNames = []string{"broker0"}
			Expect(crd.Spec.ApplyToCrNames[0]).To(Equal("broker0"))

			Expect(secHandler.IsApplicableFor(broker0)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker1)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker2)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker3)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker4)).To(BeFalse())

			By("ApplyToCrNames being broker0, broker1 only applies to broker0 and broker1 in the same namespace")
			crd.Spec.ApplyToCrNames = []string{"broker0", "broker1"}
			Expect(crd.Spec.ApplyToCrNames[0]).To(Equal("broker0"))
			Expect(crd.Spec.ApplyToCrNames[1]).To(Equal("broker1"))

			Expect(secHandler.IsApplicableFor(broker0)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker1)).To(BeTrue())
			Expect(secHandler.IsApplicableFor(broker2)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker3)).To(BeFalse())
			Expect(secHandler.IsApplicableFor(broker4)).To(BeFalse())

		})

		It("Reconcile security on multiple broker CRs", func() {

			By("Deploying 3 brokers")
			broker1Cr, createdBroker1Cr := DeployBroker("ex-aao", defaultNamespace)
			broker2Cr, createdBroker2Cr := DeployBroker("ex-aao1", defaultNamespace)
			broker3Cr, createdBroker3Cr := DeployBroker("ex-aao2", defaultNamespace)

			secCrd, createdSecCrd := DeploySecurity("", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {
				applyToCrs := make([]string, 0)
				applyToCrs = append(applyToCrs, "ex-aao")
				applyToCrs = append(applyToCrs, "ex-aao2")
				secCrdToDeploy.Spec.ApplyToCrNames = applyToCrs
			})

			requestedSs := &appsv1.StatefulSet{}

			By("Checking security gets applied to broker1 " + broker1Cr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBroker1Cr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}

				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied
			}, timeout, interval).Should(BeTrue())

			By("Checking security doesn't gets applied to broker2 " + broker2Cr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBroker2Cr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}
				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied

			}, timeout, interval).Should(BeFalse())

			By("Checking security gets applied to broker3 " + broker3Cr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBroker3Cr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					fmt.Printf("error retrieving broker3 ss %v\n", err)
					return false
				}
				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				for _, arg := range initContainer.Args {
					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true
						break
					}
				}
				return secApplied

			}, timeout, interval).Should(BeTrue())

			By("check it has gone")
			Expect(k8sClient.Delete(ctx, createdBroker1Cr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(broker1Cr.ObjectMeta.Name, defaultNamespace, createdBroker1Cr)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdBroker2Cr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(broker2Cr.ObjectMeta.Name, defaultNamespace, createdBroker2Cr)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdBroker3Cr)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(broker3Cr.ObjectMeta.Name, defaultNamespace, createdBroker3Cr)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdSecCrd)).Should(Succeed())
			Eventually(func() bool {
				return checkCrdDeleted(secCrd.ObjectMeta.Name, defaultNamespace, createdSecCrd)
			}, timeout, interval).Should(BeTrue())

		})

		It("Reconcile security on broker with non shell safe annotations", func() {

			By("Deploying broker")
			brokerCrd := generateArtemisSpec(defaultNamespace)
			brokerCrd.Spec.DeploymentPlan.Size = common.Int32ToPtr(1)
			// make is speedy for real cluster checks
			brokerCrd.Spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: 1,
				PeriodSeconds:       1,
				TimeoutSeconds:      5,
			}
			Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())

			createdBrokerCr := &brokerv1beta1.ActiveMQArtemis{}

			Eventually(func() bool {
				return getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, defaultNamespace, createdBrokerCr)
			}, timeout, interval).Should(BeTrue())
			Expect(brokerCrd.Name).Should(Equal(createdBrokerCr.ObjectMeta.Name))

			_, createdSecCrd := DeploySecurity("", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {
				applyToCrs := make([]string, 0)
				applyToCrs = append(applyToCrs, createdBrokerCr.ObjectMeta.Name)
				secCrdToDeploy.Spec.ApplyToCrNames = applyToCrs
				secCrdToDeploy.Annotations = map[string]string{
					"testannotation": "pltf-amq (1)",
				}
			})

			requestedSs := &appsv1.StatefulSet{}

			By("Checking security gets applied to broker1 " + createdBrokerCr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBrokerCr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					return false
				}

				initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
				secApplied := false
				emptyMetadata := false
				for _, arg := range initContainer.Args {

					if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
						secApplied = true

						if !(strings.Contains(arg, "testannotation")) {
							emptyMetadata = true
							break
						}
					}
				}
				return secApplied && emptyMetadata
			}, timeout, interval).Should(BeTrue())

			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				By("Checking status of CR because we expect it to deploy on a real cluster")
				key := types.NamespacedName{Name: createdBrokerCr.ObjectMeta.Name, Namespace: defaultNamespace}

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, key, createdBrokerCr)).Should(Succeed())

					g.Expect(len(createdBrokerCr.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())
			}

			Expect(k8sClient.Delete(ctx, createdBrokerCr)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, createdSecCrd)).Should(Succeed())

		})

		It("Reconcile security with management role access", func() {

			if os.Getenv("USE_EXISTING_CLUSTER") != "true" {
				return
			}

			By("Deploying broker")
			brokerCrd := generateArtemisSpec(defaultNamespace)
			brokerCrd.Spec.DeploymentPlan.Size = common.Int32ToPtr(1)
			Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())

			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			By("Checking the pod is up and running")
			Eventually(func(g Gomega) {
				brokerKey := types.NamespacedName{Name: brokerCrd.Name, Namespace: brokerCrd.Namespace}
				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
				g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))

			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Deploying security with management role access")
			mgmtDomain := "org.apache.activemq.artemis"
			method1 := "list*"
			method2 := "sendMessage*"
			method3 := "browse*"
			accessList := []brokerv1beta1.DefaultAccessType{
				{
					Method: &method1,
					Roles:  []string{"guest"},
				},
				{
					Method: &method2,
					Roles:  []string{"guest"},
				},
				{
					Method: &method3,
					Roles:  []string{"guest"},
				},
			}
			roleAccess := []brokerv1beta1.RoleAccessType{
				{
					Domain:     &mgmtDomain,
					AccessList: accessList,
				},
			}

			allowedDomain := "org.apache.activemq.artemis.allowed"
			allowedList := []brokerv1beta1.AllowedListEntryType{
				{
					Domain: &allowedDomain,
				},
			}

			_, createdSecCrd := DeploySecurity("", defaultNamespace, func(secCrdToDeploy *brokerv1beta1.ActiveMQArtemisSecurity) {

				secCrdToDeploy.Spec.SecuritySettings.Management = brokerv1beta1.ManagementSecuritySettingsType{
					Authorisation: brokerv1beta1.AuthorisationConfigType{
						AllowedList: allowedList,
						RoleAccess:  roleAccess,
					},
				}
			})

			By("Checking the pod get started")
			brokerKey := types.NamespacedName{Name: brokerCrd.Name, Namespace: brokerCrd.Namespace}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())
				g.Expect(len(createdCrd.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			By("Checking the management.xml has the correct role-access element")
			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}
			restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme))
			Expect(err).To(BeNil())

			podOrdinal := strconv.FormatInt(int64(0), 10)
			podName := namer.CrToSS(brokerCrd.Name) + "-" + podOrdinal

			brokerName := brokerCrd.Name
			Eventually(func(g Gomega) {
				execReq := restClient.
					Post().
					Namespace(defaultNamespace).
					Resource("pods").
					Name(podName).
					SubResource("exec").
					VersionedParams(&corev1.PodExecOptions{
						Container: brokerName + "-container",
						Command:   []string{"cat", "amq-broker/etc/management.xml"},
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

				By("Checking for output from pod")
				g.Eventually(func(g Gomega) {
					By("Checking for output from pod")
					g.Expect(capturedOut.Len() > 0)
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Checking pod output")
				content := capturedOut.String()
				g.Expect(content).Should(ContainSubstring("<match domain=\"org.apache.activemq.artemis\""))
				g.Expect(content).Should(ContainSubstring("<access method=\"list*\" roles=\"guest\""))
				g.Expect(content).Should(ContainSubstring("<access method=\"sendMessage*\" roles=\"guest\""))
				g.Expect(content).Should(ContainSubstring("<access method=\"browse*\" roles=\"guest\""))
				g.Expect(content).Should(ContainSubstring("<entry domain=\"org.apache.activemq.artemis.allowed\""))
			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, createdSecCrd)).Should(Succeed())
		})

	})

	It("reconcile after Broker CR deployed, verify force reconcile", func() {

		By("Creating Broker CR")
		ctx := context.Background()
		brokerCrd := generateArtemisSpec(defaultNamespace)
		brokerCrd.Spec.DeploymentPlan.Size = common.Int32ToPtr(0)

		Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())

		createdBrokerCrd := &brokerv1beta1.ActiveMQArtemis{}

		Eventually(func() (int, error) {
			key := types.NamespacedName{Name: brokerCrd.ObjectMeta.Name, Namespace: defaultNamespace}
			err := k8sClient.Get(ctx, key, createdBrokerCrd)

			if err != nil {
				return -1, err
			}

			return len(createdBrokerCrd.Status.PodStatus.Stopped), nil
		}, timeout, interval).Should(Equal(1))

		// after stable status, determine version
		createdSs := &appsv1.StatefulSet{}
		ssKey := types.NamespacedName{Name: namer.CrToSS(createdBrokerCrd.Name), Namespace: defaultNamespace}

		By("Making sure that the ss gets deployed " + createdBrokerCrd.ObjectMeta.Name)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, ssKey, createdSs)).Should(Succeed())
		}, timeout, interval).Should(Succeed())

		versionSsDeployed := createdSs.ObjectMeta.ResourceVersion
		By("tracking cc resource version: " + versionSsDeployed)

		By("Creating security cr")
		crd := generateSecuritySpec("", defaultNamespace)

		brokerDomainName := "activemq"
		loginModuleName := "module1"
		loginModuleFlag := "sufficient"

		loginModuleList := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
		propLoginModule := brokerv1beta1.PropertiesLoginModuleType{
			Name: loginModuleName,
			Users: []brokerv1beta1.UserType{
				{
					Name:     "user1",
					Password: nil,
					Roles: []string{
						"admin", "amq",
					},
				},
			},
		}
		loginModuleList = append(loginModuleList, propLoginModule)
		crd.Spec.LoginModules.PropertiesLoginModules = loginModuleList

		crd.Spec.SecurityDomains.BrokerDomain = brokerv1beta1.BrokerDomainType{
			Name: &brokerDomainName,
			LoginModules: []brokerv1beta1.LoginModuleReferenceType{
				{
					Name: &loginModuleName,
					Flag: &loginModuleFlag,
				},
			},
		}

		By("Deploying the CRD " + crd.ObjectMeta.Name)
		Expect(k8sClient.Create(ctx, crd)).Should(Succeed())

		createdCrd := &brokerv1beta1.ActiveMQArtemisSecurity{}

		By("Making sure that the CRD gets deployed " + crd.ObjectMeta.Name)
		Eventually(func() bool {
			return getPersistedVersionedCrd(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())
		Expect(createdCrd.Name).Should(Equal(crd.ObjectMeta.Name))

		// make sure broker gets new SS for createdBrokerCrd
		Eventually(func(g Gomega) {

			g.Expect(k8sClient.Get(ctx, ssKey, createdSs)).Should(Succeed())
			By("Verifying != ss set resoruce version, deployed: " + versionSsDeployed + ", current: " + createdSs.GetResourceVersion())
			g.Expect(versionSsDeployed).ShouldNot(Equal(createdSs.GetResourceVersion()))

		}, timeout, interval).Should(Succeed())

		By("check it has gone")
		Expect(k8sClient.Delete(ctx, createdBrokerCrd))
		Expect(k8sClient.Delete(ctx, createdCrd))
		Eventually(func() bool {
			return checkCrdDeleted(crd.ObjectMeta.Name, defaultNamespace, createdCrd)
		}, timeout, interval).Should(BeTrue())

	})

})

func DeploySecurity(secName string, targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemisSecurity)) (*brokerv1beta1.ActiveMQArtemisSecurity, *brokerv1beta1.ActiveMQArtemisSecurity) {
	ctx := context.Background()
	secCrd := generateSecuritySpec(secName, targetNamespace)

	brokerDomainName := "activemq"
	loginModuleName := "module1"
	loginModuleFlag := "sufficient"
	okDefaultPwd := "ok"

	loginModuleList := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
	propLoginModule := brokerv1beta1.PropertiesLoginModuleType{
		Name: loginModuleName,
		Users: []brokerv1beta1.UserType{
			{
				Name:     "user1",
				Password: &okDefaultPwd,
				Roles: []string{
					"admin", "amq",
				},
			},
		},
	}
	loginModuleList = append(loginModuleList, propLoginModule)
	secCrd.Spec.LoginModules.PropertiesLoginModules = loginModuleList

	secCrd.Spec.SecurityDomains.BrokerDomain = brokerv1beta1.BrokerDomainType{
		Name: &brokerDomainName,
		LoginModules: []brokerv1beta1.LoginModuleReferenceType{
			{
				Name: &loginModuleName,
				Flag: &loginModuleFlag,
			},
		},
	}

	customFunc(secCrd)

	Expect(k8sClient.Create(ctx, secCrd)).Should(Succeed())

	createdSecCrd := &brokerv1beta1.ActiveMQArtemisSecurity{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(secCrd.ObjectMeta.Name, targetNamespace, createdSecCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdSecCrd.Name).Should(Equal(secCrd.ObjectMeta.Name))

	return secCrd, createdSecCrd
}

func generateSecuritySpec(secName string, targetNamespace string) *brokerv1beta1.ActiveMQArtemisSecurity {

	spec := brokerv1beta1.ActiveMQArtemisSecuritySpec{}

	theName := secName
	if secName == "" {
		theName = randString()
	}

	toCreate := brokerv1beta1.ActiveMQArtemisSecurity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemisSecurity",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      theName,
			Namespace: targetNamespace,
		},
		Spec: spec,
	}

	return &toCreate
}
