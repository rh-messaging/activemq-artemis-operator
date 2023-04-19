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
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = Describe("artemis controller", Label("do"), func() {

	BeforeEach(func() {
		BeforeEachSpec()
	})

	Context("operator logging config test", Label("do-operator-log"), func() {
		It("test operator with env var", func() {
			if os.Getenv("DEPLOY_OPERATOR") == "true" {
				// re-install a new operator to have a fresh log
				uninstallOperator(false)
				installOperator(nil)
				By("checking default operator should have INFO logs")
				Eventually(func(g Gomega) {
					oprLog, err := GetOperatorLog(defaultNamespace)
					g.Expect(err).To(BeNil())
					g.Expect(*oprLog).To(ContainSubstring("INFO"))
					g.Expect(*oprLog).NotTo(ContainSubstring("DEBUG"))
					g.Expect(*oprLog).NotTo(ContainSubstring("ERROR"))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("Uninstall existing operator")
				uninstallOperator(false)

				By("install the operator again with logging env var")
				envMap := make(map[string]string)
				envMap["ARGS"] = "--zap-log-level=error"
				installOperator(envMap)
				By("delploy a basic broker to produce some more log")
				brokerCr, createdCr := DeployCustomBroker(defaultNamespace, nil)

				By("wait for pod so enough log is generated")
				Eventually(func(g Gomega) {
					getPersistedVersionedCrd(brokerCr.Name, defaultNamespace, createdCr)
					g.Expect(len(createdCr.Status.PodStatus.Ready)).Should(BeEquivalentTo(1))
				}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

				By("check no INFO/DEBUG in the log")
				oprLog, err := GetOperatorLog(defaultNamespace)
				Expect(err).To(BeNil())
				Expect(*oprLog).NotTo(ContainSubstring("INFO"))
				Expect(*oprLog).NotTo(ContainSubstring("DEBUG"))

				//clean up all resources
				Expect(k8sClient.Delete(ctx, createdCr)).Should(Succeed())
			}
		})
	})
})

func GetOperatorLog(ns string) (*string, error) {
	cfg, err := config.GetConfig()
	Expect(err).To(BeNil())
	labelSelector, err := labels.Parse("control-plane=controller-manager")
	Expect(err).To(BeNil())
	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).To(BeNil())
	listOpts := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	podList, err := clientset.CoreV1().Pods(ns).List(ctx, listOpts)
	Expect(err).To(BeNil())
	Expect(len(podList.Items)).To(Equal(1))
	operatorPod := podList.Items[0]

	podLogOpts := corev1.PodLogOptions{}
	req := clientset.CoreV1().Pods(ns).GetLogs(operatorPod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.Background())
	Expect(err).To(BeNil())
	defer podLogs.Close()

	Expect(err).To(BeNil())

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	Expect(err).To(BeNil())
	str := buf.String()

	return &str, nil
}
