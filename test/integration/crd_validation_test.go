package integration_test

import (
	//"github.com/rh-messaging/activemq-artemis-operator/test"
	"github.com/fgiorgetti/qpid-dispatch-go-tests/test"
	"testing"
)

// Just to illustrate the structure for this test suite
func TestCrdValidation(t *testing.T) {
	test.Initialize(t, "crd_validation", "CRD Validation Suite")
}
