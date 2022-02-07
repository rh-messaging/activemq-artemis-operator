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
package common

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStateManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "state manaer Suite")
}

var _ = BeforeSuite(func() {
	fmt.Println("=======Before state manager Suite========")
})

var _ = AfterSuite(func() {
	fmt.Println("=======After state manager Suite========")
})

var _ = Describe("state manager test", func() {
	Context("TestSingelton", func() {

		stateManager := GetStateManager()
		stateManagerTwo := GetStateManager()

		stateManager.SetState("Test", "string")

		Expect(stateManager.GetState("NotSet")).Should(BeNil())
		Expect(stateManager.GetState("Test")).Should(Equal("string"))
		Expect(stateManager).Should(Equal(stateManagerTwo))
	})
})
