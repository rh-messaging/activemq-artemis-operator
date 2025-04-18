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
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	brokerv1beta1 "github.com/arkmq-org/activemq-artemis-operator/api/v1beta1"
	"github.com/arkmq-org/activemq-artemis-operator/pkg/utils/common"
)

var _ = Describe("minimal", func() {

	var installedCertManager bool = false

	BeforeEach(func() {
		BeforeEachSpec()

		if verbose {
			fmt.Println("Time with MicroSeconds: ", time.Now().Format("2006-01-02 15:04:05.000000"), " test:", CurrentSpecReport())
		}

		if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
			//if cert manager/trust manager is not installed, install it
			if !CertManagerInstalled() {
				Expect(InstallCertManager()).To(Succeed())
				installedCertManager = true
			}

			rootIssuer = InstallClusteredIssuer(rootIssuerName, nil)

			rootCert = InstallCert(rootCertName, rootCertNamespce, func(candidate *cmv1.Certificate) {
				candidate.Spec.IsCA = true
				candidate.Spec.CommonName = "artemis.root.ca"
				candidate.Spec.SecretName = rootCertSecretName
				candidate.Spec.IssuerRef = cmmetav1.ObjectReference{
					Name: rootIssuer.Name,
					Kind: "ClusterIssuer",
				}
			})

			caIssuer = InstallClusteredIssuer(caIssuerName, func(candidate *cmv1.ClusterIssuer) {
				candidate.Spec.SelfSigned = nil
				candidate.Spec.CA = &cmv1.CAIssuer{
					SecretName: rootCertSecretName,
				}
			})
			InstallCaBundle(common.DefaultOperatorCASecretName, rootCertSecretName, caPemTrustStoreName)

		}

	})

	AfterEach(func() {

		if false && os.Getenv("USE_EXISTING_CLUSTER") == "true" {
			UnInstallCaBundle(common.DefaultOperatorCASecretName)
			UninstallClusteredIssuer(caIssuerName)
			UninstallCert(rootCert.Name, rootCert.Namespace)
			UninstallClusteredIssuer(rootIssuerName)

			if installedCertManager {
				Expect(UninstallCertManager()).To(Succeed())
				installedCertManager = false
			}
		}
		AfterEachSpec()
	})

	Context("restricted rbac", func() {

		It("operator role access", func() {

			if os.Getenv("USE_EXISTING_CLUSTER") != "true" {
				return
			}

			By("installing operator cert")
			InstallCert(common.DefaultOperatorCertSecretName, defaultNamespace, func(candidate *cmv1.Certificate) {
				candidate.Spec.SecretName = common.DefaultOperatorCertSecretName
				candidate.Spec.CommonName = "activemq-artemis-operator"
				candidate.Spec.IssuerRef = cmmetav1.ObjectReference{
					Name: caIssuer.Name,
					Kind: "ClusterIssuer",
				}
			})

			ctx := context.Background()

			// empty CRD, name is used for cert subject to match the headless service
			crd := brokerv1beta1.ActiveMQArtemis{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ActiveMQArtemis",
					APIVersion: brokerv1beta1.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      NextSpecResourceName(),
					Namespace: defaultNamespace,
				},
			}

			sharedOperandCertName := common.DefaultOperandCertSecretName
			By("installing restricted mtls broker cert")
			InstallCert(sharedOperandCertName, defaultNamespace, func(candidate *cmv1.Certificate) {
				candidate.Spec.SecretName = sharedOperandCertName
				candidate.Spec.CommonName = "activemq-artemis-operand"
				candidate.Spec.DNSNames = []string{common.OrdinalFQDNS(crd.Name, defaultNamespace, 0)}
				candidate.Spec.IssuerRef = cmmetav1.ObjectReference{
					Name: caIssuer.Name,
					Kind: "ClusterIssuer",
				}
			})

			crd.Spec.Restricted = common.NewTrue()

			// how the jdk command line can be configured or modified
			crd.Spec.Env = []corev1.EnvVar{
				{Name: "JDK_JAVA_OPTIONS", Value: "-Djavax.net.debug=ssl -Djava.security.debug=logincontext"},
				//{Name: "JAVA_ARGS_APPEND", Value: "-DordinalProp=${STATEFUL_SET_ORDINAL}"},
			}
			crd.Spec.BrokerProperties = []string{
				"messageCounterSamplePeriod=500",
			}

			By("Deploying the CRD " + crd.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, &crd)).Should(Succeed())

			brokerKey := types.NamespacedName{Name: crd.Name, Namespace: crd.Namespace}
			createdCrd := &brokerv1beta1.ActiveMQArtemis{}

			By("Checking ready, operator can access broker status via jmx")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, brokerKey, createdCrd)).Should(Succeed())

				if verbose {
					fmt.Printf("STATUS: %v\n\n", createdCrd.Status.Conditions)
				}
				g.Expect(meta.IsStatusConditionTrue(createdCrd.Status.Conditions, brokerv1beta1.ReadyConditionType)).Should(BeTrue())
				g.Expect(meta.IsStatusConditionTrue(createdCrd.Status.Conditions, brokerv1beta1.ConfigAppliedConditionType)).Should(BeTrue())

			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			serverName := common.OrdinalFQDNS(crd.Name, defaultNamespace, 0)
			By("setting up operator identity on http client")
			httpClient := http.Client{
				Transport: http.DefaultTransport,
				// A timeout less than 3 seconds may cause connection issues when
				// the server requires to change the chiper.
				Timeout: time.Second * 3,
			}

			httpClientTransport := httpClient.Transport.(*http.Transport)
			httpClientTransport.TLSClientConfig = &tls.Config{
				ServerName:         serverName,
				InsecureSkipVerify: false,
			}
			httpClientTransport.TLSClientConfig.GetClientCertificate =
				func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
					return common.GetOperatorClientCertificate(k8sClient, cri)
				}

			if rootCas, err := common.GetRootCAs(k8sClient); err == nil {
				httpClientTransport.TLSClientConfig.RootCAs = rootCas
			}

			By("Checking metrics with mtls are visible")
			Eventually(func(g Gomega) {

				resp, err := httpClient.Get("https://" + serverName + ":8888/metrics")
				if verbose {
					fmt.Printf("Resp form metrics get resp: %v, error: %v\n", resp, err)
				}
				g.Expect(err).Should(Succeed())

				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				g.Expect(err).Should(Succeed())

				lines := strings.Split(string(body), "\n")

				var done = false
				for _, line := range lines {
					if verbose {
						fmt.Printf("%s\n", line)
					}
					if strings.Contains(line, "artemis_total_pending_message_count") {
						done = true
					}
				}
				g.Expect(done).To(BeTrue())

			}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, createdCrd)).Should(Succeed())

			UninstallCert(common.DefaultOperatorCertSecretName, defaultNamespace)
			UninstallCert(sharedOperandCertName, defaultNamespace)
		})
	})
})
