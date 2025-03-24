package version

import (
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

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"7120": "7.12.0",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.12.0": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"7.12.0",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"7.12.0": "2.33.0.redhat-00010",
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
