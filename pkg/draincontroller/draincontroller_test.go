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
package draincontroller

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestDrainController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Drain Controller Suite")
}

var _ = Describe("Drain Controller Test", func() {
	Context("Global template test", func() {
		It("testing global template has correct openshift ping dns service port", func() {
			pod := corev1.Pod{}
			err := json.Unmarshal([]byte(globalPodTemplateJson), &pod)
			Expect(err).Should(Succeed())

			var servicePort string
			envVars := pod.Spec.Containers[0].Env
			for _, v := range envVars {
				if v.Name == "OPENSHIFT_DNS_PING_SERVICE_PORT" {
					servicePort = v.Value
				}
			}
			Expect(servicePort).To(Equal("7800"))
		})
	})
})
