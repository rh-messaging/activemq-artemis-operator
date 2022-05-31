package cr2jinja2

import (
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCr2Jinja2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cr2Jinja2 Suite")
}

var _ = Describe("Cr2jinja2 Test", func() {
	Context("testing special key must be of string type", func() {
		special := "OFF"
		theKey := GetUniqueShellSafeSubstution(special)
		_, err := strconv.ParseInt(theKey, 10, 64)
		Expect(err).ShouldNot(Succeed())
	})
})
