package v2alpha4activemqartemis

import (
	"fmt"
	"strings"

	api "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha4"
	"github.com/artemiscloud/activemq-artemis-operator/version"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.8.1"
	CompactLatestVersion = "781"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.8.0"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.7.0"
)

// SupportedVersions - product versions this operator supports
var SupportedVersions = []string{LatestVersion, LastMicroVersion, LastMinorVersion}
var OperandVersionFromOperatorVersion map[string]string = map[string]string{
	"0.17.0": "7.7.0",
	"0.18.0": "7.8.0",
	"0.19.0": "7.8.1",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "7.7.0",
	"80": "7.8.0",
	"81": "7.8.1",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "770",
	"80": "780",
	"81": "781",
}

func checkProductUpgrade(cr *api.ActiveMQArtemis) (upgradesMinor, upgradesEnabled bool, err error) {

	err = nil
	if isVersionSupported(cr.Spec.Version) {
		if cr.Spec.Version != LatestVersion && cr.Spec.Upgrades.Enabled {
			upgradesEnabled = cr.Spec.Upgrades.Enabled
			upgradesMinor = cr.Spec.Upgrades.Minor
		}
	} else {
		err = fmt.Errorf("Product version %s is not allowed in operator version %s. The following versions are allowed - %s", cr.Spec.Version, version.Version, SupportedVersions)
	}
	return upgradesMinor, upgradesEnabled, err
}

func isVersionSupported(specifiedVersion string) bool {
	for _, thisSupportedVersion := range SupportedVersions {
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
			api.SchemeGroupVersion.Group: OperandVersionFromOperatorVersion[version.Version],
		})
	}
	if len(cr.Spec.Version) == 0 {
		cr.Spec.Version = LatestVersion
	}
}

func GetImage(imageURL string) (image, imageTag, imageContext string) {
	urlParts := strings.Split(imageURL, "/")
	if len(urlParts) > 1 {
		imageContext = urlParts[len(urlParts)-2]
	}
	imageAndTag := urlParts[len(urlParts)-1]
	imageParts := strings.Split(imageAndTag, ":")
	image = imageParts[0]
	if len(imageParts) > 1 {
		imageTag = imageParts[len(imageParts)-1]
	}
	return image, imageTag, imageContext
}
