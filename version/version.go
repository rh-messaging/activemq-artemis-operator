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
	"7120": "7.12.0",
	"7121": "7.12.1",
	"7122": "7.12.2",
	"7123": "7.12.3",
	"7124": "7.12.4",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.12.0": "7.10.0",
	"7.12.1": "7.10.0",
	"7.12.2": "7.10.0",
	"7.12.3": "7.10.0",
	"7.12.4": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported Apache ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"7.12.0",
	"7.12.1",
	"7.12.2",
	"7.12.3",
	"7.12.4",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"7.12.0": "2.33.0.redhat-00010",
	"7.12.1": "2.33.0.redhat-00013",
	"7.12.2": "2.33.0.redhat-00015",
	"7.12.3": "2.33.0.redhat-00016",
	"7.12.4": "2.33.0.redhat-00017",
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
