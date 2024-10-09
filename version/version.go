package version

import (
	"strings"

	"github.com/blang/semver/v4"
)

var (
	Version = "7.11.7.OPR.2"
	// PriorVersion - prior version
	PriorVersion = "7.10.2.OPR.2"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.11.7"
	CompactLatestVersion = "7117"

	LatestKubeImage = "registry.redhat.io/amq7/amq-broker-rhel8:7.11.7"
	LatestInitImage = "registry.redhat.io/amq7/amq-broker-init-rhel8:7.11.7"
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"770":  "7.7.0",
	"780":  "7.8.0",
	"781":  "7.8.1",
	"782":  "7.8.2",
	"783":  "7.8.3",
	"790":  "7.9.0",
	"791":  "7.9.1",
	"792":  "7.9.2",
	"793":  "7.9.3",
	"794":  "7.9.4",
	"7100": "7.10.0",
	"7101": "7.10.1",
	"7102": "7.10.2",
	"7110": "7.11.0",
	"7111": "7.11.1",
	"7103": "7.10.3",
	"7112": "7.11.2",
	"7113": "7.11.3",
	"7104": "7.10.4",
	"7114": "7.11.4",
	"7105": "7.10.5",
	"7115": "7.11.5",
	"7116": "7.11.6",
	"7106": "7.10.6",
	"7117": "7.11.7",
	"7107": "7.10.7",
}

// The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0":  "7.7.0",
	"7.8.0":  "7.8.0",
	"7.8.1":  "7.8.1",
	"7.8.2":  "7.8.2",
	"7.8.3":  "7.8.2",
	"7.9.0":  "7.9.0",
	"7.9.1":  "7.9.0",
	"7.9.2":  "7.9.0",
	"7.9.3":  "7.9.0",
	"7.9.4":  "7.9.0",
	"7.10.0": "7.10.0",
	"7.10.1": "7.10.0",
	"7.10.2": "7.10.0",
	"7.11.0": "7.10.0",
	"7.11.1": "7.10.0",
	"7.10.3": "7.10.0",
	"7.11.2": "7.10.0",
	"7.11.3": "7.10.0",
	"7.10.4": "7.10.0",
	"7.11.4": "7.10.0",
	"7.10.5": "7.10.0",
	"7.11.5": "7.10.0",
	"7.11.6": "7.10.0",
	"7.10.6": "7.10.0",
	"7.11.7": "7.10.0",
	"7.10.7": "7.10.0",
}

var YacfgProfileName string = "amq_broker"

// Sorted array of supported ActiveMQ Artemis versions
var SupportedActiveMQArtemisVersions = []string{
	"7.10.0",
	"7.10.1",
	"7.10.2",
	"7.11.0",
	"7.11.1",
	"7.10.3",
	"7.11.2",
	"7.11.3",
	"7.10.4",
	"7.11.4",
	"7.10.5",
	"7.11.5",
	"7.11.6",
	"7.10.6",
	"7.11.7",
	"7.10.7",
}

var ActiveMQArtemisVersionfromFullVersion map[string]string = map[string]string{
	"7.10.0": "2.21.0.redhat-00025",
	"7.10.1": "2.21.0.redhat-00030",
	"7.10.2": "2.21.0.redhat-00041",
	"7.11.0": "2.28.0.redhat-00003",
	"7.11.1": "2.28.0.redhat-00004",
	"7.10.3": "2.21.0.redhat-00044",
	"7.11.2": "2.28.0.redhat-00009",
	"7.11.3": "2.28.0.redhat-00011",
	"7.10.4": "2.21.0.redhat-00045",
	"7.11.4": "2.28.0.redhat-00012",
	"7.10.5": "2.21.0.redhat-00046",
	"7.11.5": "2.28.0.redhat-00016",
	"7.11.6": "2.28.0.redhat-00019",
	"7.10.6": "2.21.0.redhat-00048",
	"7.11.7": "2.28.0.redhat-00022",
	"7.10.7": "2.21.0.redhat-00052",
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
