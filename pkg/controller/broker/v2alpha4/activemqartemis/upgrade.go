package v2alpha4activemqartemis

import (
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"strings"
)

func isVersionSupported(specifiedVersion string) bool {
	for _, thisSupportedVersion := range version.SupportedVersions {
		if thisSupportedVersion == specifiedVersion {
			return true
		}
	}
	return false
}

func getMinorImageVersion(productVersion string) string {
	major, minor, _ := MajorMinorMicro(productVersion)
	return strings.Join([]string{major, minor}, "")
}

func MajorMinorMicro(productVersion string) (major, minor, micro string) {
	version := strings.Split(productVersion, ".")
	for len(version) < 3 {
		version = append(version, "0")
	}
	return version[0], version[1], version[2]
}
