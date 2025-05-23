package pods

import (
	"testing"

	"github.com/arkmq-org/activemq-artemis-operator/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPods(t *testing.T) {
	RegisterFailHandler(Fail)
}

func TestCurrentCompVer(t *testing.T) {

	if _, found := productVerFromImageVer[version.LatestVersion]; !found {
		t.Errorf("Expect entry for " + version.LatestVersion)
	}
}

func TestGetAdditionalLabels(t *testing.T) {

	lables := GetAdditionalLabels(version.LatestVersion)
	expected := 7
	if len(lables) != expected {
		t.Errorf("Expect %v lables but got %v", len(lables), expected)
	}
}
