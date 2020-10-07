package v2alpha3activemqartemis

import (
	"fmt"
	api "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/artemiscloud/activemq-artemis-operator/version"
	"strings"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.7.0"
	CompactLatestVersion = "770"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.6.0"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.6.0"
)

// SupportedVersions - product versions this operator supports
var SupportedVersions = []string{LatestVersion, LastMicroVersion, LastMinorVersion}
var OperandVersionFromOperatorVersion map[string]string = map[string]string{
	"0.9.1" : "7.5.0",
	"0.13.0" : "7.6.0",
	"0.15.0" : "7.7.0",
	"0.17.0" : "7.7.0",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"75" : "7.5.0",
	"76" : "7.6.0",
	"77" : "7.7.0",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"75" : "750",
	"76" : "760",
	"77" : "770",
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
