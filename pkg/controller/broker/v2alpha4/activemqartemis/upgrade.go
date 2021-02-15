package v2alpha4activemqartemis

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	brokerv2alpha4 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha4"
	"github.com/go-logr/logr"
	"github.com/artemiscloud/activemq-artemis-operator/version"
)

//const (
//	// LatestVersion product version supported
//	LatestVersion        = "7.8.1"
//	CompactLatestVersion = "781"
//	// LastMicroVersion product version supported
//	LastMicroVersion = "7.8.0"
//	// LastMinorVersion product version supported
//	LastMinorVersion = "7.7.0"
//)
//
//// SupportedVersions - product versions this operator supports
//var SupportedVersions = []string{LatestVersion, LastMicroVersion, LastMinorVersion}
//var OperandVersionFromOperatorVersion map[string]string = map[string]string{
//	"0.17.0": "7.7.0",
//	"0.18.0": "7.8.0",
//	"0.19.0": "7.8.1",
//}
//var FullVersionFromMinorVersion map[string]string = map[string]string{
//	"70": "7.7.0",
//	"80": "7.8.0",
//	"81": "7.8.1",
//}
//
//var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
//	"70": "770",
//	"80": "780",
//	"81": "781",
//}

func checkProductUpgrade(cr *brokerv2alpha4.ActiveMQArtemis) (upgradesMinor, upgradesEnabled bool, err error) {

	err = nil

	//setDefaults(cr)
	if isVersionSupported(cr.Spec.Version) {
		if cr.Spec.Version != version.LatestVersion && cr.Spec.Upgrades.Enabled {
			upgradesEnabled = cr.Spec.Upgrades.Enabled
			upgradesMinor = cr.Spec.Upgrades.Minor
		}
	} else {
		err = fmt.Errorf("Product version %s is not allowed in operator version %s. The following versions are allowed - %s", cr.Spec.Version, version.Version, version.SupportedVersions)
	}

	return upgradesMinor, upgradesEnabled, err
}

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

// MajorMinorMicro ...
func MajorMinorMicro(productVersion string) (major, minor, micro string) {
	version := strings.Split(productVersion, ".")
	for len(version) < 3 {
		version = append(version, "0")
	}
	return version[0], version[1], version[2]
}

func setDefaults(cr *brokerv2alpha4.ActiveMQArtemis) {
	if cr.GetAnnotations() == nil {
		cr.SetAnnotations(map[string]string{
			brokerv2alpha4.SchemeGroupVersion.Group: version.OperandVersionFromOperatorVersion[version.Version],
		})
	}
	if len(cr.Spec.Version) == 0 {
		cr.Spec.Version = version.LatestVersion
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

func checkUpgradeVersions(customResource *brokerv2alpha4.ActiveMQArtemis, err error, reqLogger logr.Logger) error {
	_, _, err = checkProductUpgrade(customResource)
	//if err != nil {
	//	log.Info("checkProductUpgrade failed")
	//} else {
	//	hasUpdates = true
	//}
	//specifiedMinorVersion := getMinorImageVersion(customResource.Spec.Version)
	specifiedMajorVersion, specifiedMinorVersion, specifiedMicroVersion := MajorMinorMicro(customResource.Spec.Version)
	reqLogger.V(1).Info("Specified major version " + specifiedMajorVersion)
	reqLogger.V(1).Info("Specified minor version " + specifiedMinorVersion)
	reqLogger.V(1).Info("Specified micro version " + specifiedMicroVersion)
	if customResource.Spec.Upgrades.Enabled && customResource.Spec.Upgrades.Minor {
		imageName, imageTag, imageContext := GetImage(customResource.Spec.DeploymentPlan.Image)
		reqLogger.V(1).Info("Current imageName " + imageName)
		reqLogger.V(1).Info("Current imageTag " + imageTag)
		reqLogger.V(1).Info("Current imageContext " + imageContext)

		imageTagNoDash := strings.Replace(imageTag, "-", ".", -1)
		imageVersionSplitFromTag := strings.Split(imageTagNoDash, ".")
		var currentMinorVersion = ""
		if 3 == len(imageVersionSplitFromTag) {
			currentMinorVersion = imageVersionSplitFromTag[0] + imageVersionSplitFromTag[1]
		}
		reqLogger.V(1).Info("Current minor version " + currentMinorVersion)

		if specifiedMinorVersion != currentMinorVersion {
			// reset current annotations and update CR use to specified product version
			customResource.SetAnnotations(map[string]string{
				brokerv2alpha4.SchemeGroupVersion.Group: version.FullVersionFromMinorVersion[specifiedMinorVersion]})
			customResource.Spec.Version = version.FullVersionFromMinorVersion[specifiedMinorVersion]

			upgradeVersionEnvBrokerImage := os.Getenv("RELATED_IMAGE_ActiveMQ_Artemis_Broker_Kubernetes_" + version.CompactFullVersionFromMinorVersion[specifiedMinorVersion])
			if "s390x" == runtime.GOARCH || "ppc64le" == runtime.GOARCH {
				upgradeVersionEnvBrokerImage = upgradeVersionEnvBrokerImage + "_" + runtime.GOARCH
			}
			reqLogger.V(1).Info("Upgrade image from env " + upgradeVersionEnvBrokerImage)

			if "" != upgradeVersionEnvBrokerImage {
				customResource.Spec.DeploymentPlan.Image = upgradeVersionEnvBrokerImage
			}

			imageName, imageTag, imageContext = GetImage(customResource.Spec.DeploymentPlan.Image)
			reqLogger.V(1).Info("Updated imageName " + imageName)
			reqLogger.V(1).Info("Updated imageTag " + imageTag)
			reqLogger.V(1).Info("Updated imageContext " + imageContext)
		}
	}

	return err
}
