package version

import (
	"strings"

	"github.com/blang/semver/v4"
)

var (
	Version = "7.x.x.OPR.1"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.x.x"
	CompactLatestVersion = "7xx"

	LatestKubeImage = "registry.redhat.io/amq7/amq-broker-rhel8:7.x.x" + LatestVersion
	LatestInitImage = "registry.redhat.io/amq7/amq-broker-init-rhel8:7.x.x" + LatestVersion
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"7110": "7.11.0",
	"7111": "7.11.1",
	"7112": "7.11.2",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.11.0": "7.10.0",
	"7.11.1": "7.10.0",
	"7.11.2": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"7.11.0",
	"7.11.1",
	"7.11.2",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"7.11.0": "2.28.0.redhat-00003",
	"7.11.1": "2.28.0.redhat-00004",
	"7.11.2": "2.28.0.redhat-00009",
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
