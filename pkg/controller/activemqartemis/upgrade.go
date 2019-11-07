package activemqartemis

import (
	"fmt"
	api "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/version"
	"strings"
)

const (
	// CurrentVersion product version supported
	CurrentVersion = "7.6.0"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.5.1"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.5.0"
)

// SupportedVersions - product versions this operator supports
var SupportedVersions = []string{CurrentVersion, LastMicroVersion, LastMinorVersion}

// checkProductUpgrade ...
func checkProductUpgrade(cr *api.ActiveMQArtemis) (minor, micro bool, err error) {
	setDefaults(cr)
	if checkVersion(cr.Spec.Version) {
		if cr.Spec.Version != CurrentVersion && cr.Spec.Upgrades.Enabled {
			micro = cr.Spec.Upgrades.Enabled
			minor = cr.Spec.Upgrades.Minor
		}
	} else {
		err = fmt.Errorf("Product version %s is not allowed in operator version %s. The following versions are allowed - %s", cr.Spec.Version, version.Version, SupportedVersions)
	}
	return minor, micro, err
}

// checkVersion ...
func checkVersion(productVersion string) bool {
	for _, version := range SupportedVersions {
		if version == productVersion {
			return true
		}
	}
	return false
}

// getMinorImageVersion ...
func getMinorImageVersion(productVersion string) string {
	major, minor, _ := MajorMinorMicro(productVersion)
	return strings.Join([]string{major, minor}, "")
}

// MajorMinorMicro ...
func MajorMinorMicro(productVersion string) (major, minor, micro string) {
	version := strings.Split(productVersion, ".")
	for len(version) < 3 {
		version = append(version, "0")
	}
	return version[0], version[1], version[2]
}

func setDefaults(cr *api.ActiveMQArtemis) {
	if cr.GetAnnotations() == nil {
		cr.SetAnnotations(map[string]string{
			api.SchemeGroupVersion.Group: version.Version,
		})
	}
	if len(cr.Spec.Version) == 0 {
		cr.Spec.Version = CurrentVersion
	}

}
