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
	"strings"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Define utility constants for object names and testing timeouts/durations and intervals.
const (
	defaultNamespace = "default"
	timeout          = time.Second * 15
	duration         = time.Second * 10
	interval         = time.Millisecond * 250
	verobse          = false
)

var _ = Describe("security controller", func() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("Reconcile Test", func() {
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

			fmt.Printf("original handler %p\n", realHandler)

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

			fmt.Printf("new handler %p\n", newRealHandler)

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
					fmt.Printf("error retrieving broker1 ss %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fmt.Printf("ss1 %v\n", requestedSs)
			initContainer := requestedSs.Spec.Template.Spec.InitContainers[0]
			secApplied := false
			for _, arg := range initContainer.Args {
				fmt.Println("checking arg " + arg)
				if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
					secApplied = true
					break
				}
			}
			Expect(secApplied).To(BeTrue())

			By("Checking security doesn't gets applied to broker2 " + broker2Cr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBroker2Cr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					fmt.Printf("error retrieving broker2 ss %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fmt.Printf("ss2 %v\n", requestedSs)
			initContainer = requestedSs.Spec.Template.Spec.InitContainers[0]
			secApplied = false
			for _, arg := range initContainer.Args {
				fmt.Println("checking arg " + arg)
				if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
					secApplied = true
					break
				}
			}
			Expect(secApplied).To(BeFalse())

			By("Checking security gets applied to broker3 " + broker3Cr.Name)
			Eventually(func() bool {
				key := types.NamespacedName{Name: namer.CrToSS(createdBroker3Cr.Name), Namespace: defaultNamespace}
				err := k8sClient.Get(ctx, key, requestedSs)
				if err != nil {
					fmt.Printf("error retrieving broker3 ss %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fmt.Printf("ss3 %v\n", requestedSs)
			initContainer = requestedSs.Spec.Template.Spec.InitContainers[0]
			secApplied = false
			for _, arg := range initContainer.Args {
				fmt.Println("checking arg " + arg)
				if strings.Contains(arg, "mkdir -p /init_cfg_root/security/security") {
					secApplied = true
					break
				}
			}
			Expect(secApplied).To(BeTrue())

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
	})
})

func DeploySecurity(secName string, targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemisSecurity)) (*brokerv1beta1.ActiveMQArtemisSecurity, *brokerv1beta1.ActiveMQArtemisSecurity) {
	ctx := context.Background()
	secCrd := generateSecuritySpec(secName, targetNamespace)

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

	fmt.Printf("going to deploy security %v\n", secCrd)

	Expect(k8sClient.Create(ctx, secCrd)).Should(Succeed())

	createdSecCrd := &brokerv1beta1.ActiveMQArtemisSecurity{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(secCrd.ObjectMeta.Name, targetNamespace, createdSecCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdSecCrd.Name).Should(Equal(createdSecCrd.ObjectMeta.Name))

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
