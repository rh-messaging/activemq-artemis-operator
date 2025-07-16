package version

import (
	"strings"

	"github.com/blang/semver/v4"
)

var (
	Version = "7.13.1.OPR.1"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.13.1"
	CompactLatestVersion = "7131"

	LatestKubeImage = "registry.redhat.io/amq7/amq-broker-rhel9:7.13.1"
	LatestInitImage = "registry.redhat.io/amq7/amq-broker-init-rhel9:7.13.1"
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"7120": "7.12.0",
	"7121": "7.12.1",
	"7122": "7.12.2",
	"7123": "7.12.3",
	"7130": "7.13.0",
	"7124": "7.12.4",
	"7131": "7.13.1",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.12.0": "7.10.0",
	"7.12.1": "7.10.0",
	"7.12.2": "7.10.0",
	"7.12.3": "7.10.0",
	"7.13.0": "7.10.0",
	"7.12.4": "7.10.0",
	"7.13.1": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"7.12.0",
	"7.12.1",
	"7.12.2",
	"7.12.3",
	"7.13.0",
	"7.12.4",
	"7.13.1",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"7.12.0": "2.33.0.redhat-00010",
	"7.12.1": "2.33.0.redhat-00013",
	"7.12.2": "2.33.0.redhat-00015",
	"7.12.3": "2.33.0.redhat-00016",
	"7.13.0": "2.40.0.redhat-00004",
	"7.12.4": "2.33.0.redhat-00017",
	"7.13.1": "2.40.0.redhat-00004",
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
