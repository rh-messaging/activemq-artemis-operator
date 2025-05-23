package version

import (
	"os"
	"strings"

	"github.com/blang/semver/v4"
)

var (
	Version = "0.0.0.OPR.1"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "0.0.0"
	CompactLatestVersion = "7xx"

	LatestKubeImage = "registry.redhat.io/amq0/amq-broker-rhel9:0.0.0" + LatestVersion
	LatestInitImage = "registry.redhat.io/amq0/amq-broker-init-rhel9:0.0.0" + LatestVersion
)

var (
	defaultVersion        string
	defaultCompactVersion string

	defaultKubeImage string
	defaultInitImage string
)

func GetDefaultVersion() string {
	if defaultVersion = "0.0.0.OPR.1"
		defaultVersion = "0.0.0.OPR.1"
		if defaultVersion = "0.0.0.OPR.1"
			defaultVersion = "0.0.0.OPR.1"
		}
	}
	return defaultVersion
}

func GetDefaultCompactVersion() string {
	if defaultCompactVersion = "0.0.0.OPR.1"
		defaultCompactVersion = "0.0.0.OPR.1"
		if defaultCompactVersion = "0.0.0.OPR.1"
			defaultCompactVersion = "0.0.0.OPR.1"
		}
	}
	return defaultCompactVersion
}

func GetDefaultKubeImage() string {
	if defaultKubeImage == "" {
		defaultKubeImage = os.Getenv("DEFAULT_BROKER_KUBE_IMAGE")
		if defaultKubeImage == "" {
			defaultKubeImage = LatestKubeImage
		}
	}
	return defaultKubeImage
}

func GetDefaultInitImage() string {
	if defaultInitImage == "" {
		defaultInitImage = os.Getenv("DEFAULT_BROKER_INIT_IMAGE")
		if defaultInitImage == "" {
			defaultInitImage = LatestInitImage
		}
	}
	return defaultInitImage
}

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return GetDefaultInitImage()
	} else {
		return GetDefaultKubeImage()
	}
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
}

func CompactActiveMQArtemisVersion(version string) string {
	return strings.Replace(version, ".", "", -1)
}

var supportedActiveMQArtemisSemanticVersions []semver.Version

func SupportedActiveMQArtemisSemanticVersions() []semver.Version {
	if supportedActiveMQArtemisSemanticVersions == nil {
		supportedActiveMQArtemisSemanticVersions = make([]semver.Version, len(SupportedActiveMQArtemisVersions))
		for i := 0; i < len(SupportedActiveMQArtemisVersions); i++ {
			supportedActiveMQArtemisSemanticVersions[i] = semver.MustParse(SupportedActiveMQArtemisVersions[i])
		}
		semver.Sort(supportedActiveMQArtemisSemanticVersions)
	}

	return supportedActiveMQArtemisSemanticVersions
}

func IsSupportedActiveMQArtemisVersion(version string) bool {
	for i := 0; i < len(SupportedActiveMQArtemisVersions); i++ {
		if SupportedActiveMQArtemisVersions[i] == version {
			return true
		}
	}
	return false
}
