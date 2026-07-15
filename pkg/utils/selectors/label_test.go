/*
Copyright 2026.

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

package selectors

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Labeler", func() {
	It("uses ActiveMQArtemis tracking label for ActiveMQArtemis labeler", func() {
		labeler := NewActiveMQArtemisLabeler()
		labeler.Base("my-broker").Suffix("app").Generate()
		labels := labeler.Labels()

		Expect(labels).To(HaveKeyWithValue(LabelActiveMQArtemisKey, "my-broker"))
		Expect(labels).To(HaveKeyWithValue(LabelAppKey, "my-broker-app"))
		Expect(labels).NotTo(HaveKey(LabelBrokerKey))
	})

	It("uses Broker tracking label for Broker labeler", func() {
		labeler := NewBrokerLabeler()
		labeler.Base("my-broker").Suffix("app").Generate()
		labels := labeler.Labels()

		Expect(labels).To(HaveKeyWithValue(LabelBrokerKey, "my-broker"))
		Expect(labels).To(HaveKeyWithValue(LabelAppKey, "my-broker-app"))
		Expect(labels).NotTo(HaveKey(LabelActiveMQArtemisKey))
	})

	It("keeps ActiveMQArtemis as default in GetLabels", func() {
		labels := GetLabels("ex-aao")

		Expect(labels).To(HaveKeyWithValue(LabelActiveMQArtemisKey, "ex-aao"))
		Expect(labels).To(HaveKeyWithValue(LabelAppKey, "ex-aao-app"))
		Expect(labels).NotTo(HaveKey(LabelBrokerKey))
	})
})
