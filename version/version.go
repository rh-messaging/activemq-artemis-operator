package version

import (
	"os"
	"strings"

	"github.com/blang/semver/v4"
)

var (
	Version = "8.0.0.OPR.1"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "8.0.0"
	CompactLatestVersion = "800"

	LatestKubeImage = "registry.redhat.io/amq8/amq-broker-rhel9:8.0.0"
	LatestInitImage = "registry.redhat.io/amq8/amq-broker-init-rhel9:8.0.0"
)

var (
	defaultVersion        string
	defaultCompactVersion string

	defaultKubeImage string
	defaultInitImage string
)

func GetDefaultVersion() string {
	if defaultVersion == "" {
		defaultVersion = os.Getenv("DEFAULT_BROKER_VERSION")
		if defaultVersion == "" {
			defaultVersion = LatestVersion
		}
	}
	return defaultVersion
}

func GetDefaultCompactVersion() string {
	if defaultCompactVersion == "" {
		defaultCompactVersion = os.Getenv("DEFAULT_BROKER_COMPACT_VERSION")
		if defaultCompactVersion == "" {
			defaultCompactVersion = CompactLatestVersion
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
	"800": "8.0.0",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"8.0.0": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"8.0.0",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"8.0.0": "2.43.0.temporary-redhat-00022",
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
