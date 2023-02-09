package version

import "strings"

var (
	Version = "1.0.10"
	// PriorVersion - prior version
	PriorVersion = "1.0.9"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "2.28.0"
	CompactLatestVersion = "2280"
	// LastMinorVersion product version supported
	LastMinorVersion = "2.20.0"

	LatestKubeImage = "quay.io/artemiscloud/activemq-artemis-broker-kubernetes:artemis." + LatestVersion
	LatestInitImage = "quay.io/artemiscloud/activemq-artemis-broker-init:artemis." + LatestVersion
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var CompactVersionFromVersion map[string]string = map[string]string{
	"2.15.0": "2150",
	"2.16.0": "2160",
	"2.18.0": "2180",
	"2.20.0": "2200",
	"2.21.0": "2210",
	"2.22.0": "2220",
	"2.23.0": "2230",
	"2.25.0": "2250",
	"2.26.0": "2260",
	"2.27.0": "2270",
	"2.28.0": "2280",
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"2150": "2.15.0",
	"2160": "2.16.0",
	"2180": "2.18.0",
	"2200": "2.20.0",
	"2210": "2.21.0",
	"2220": "2.22.0",
	"2230": "2.23.0",
	"2250": "2.25.0",
	"2260": "2.26.0",
	"2270": "2.27.0",
	"2280": "2.28.0",
}

//The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"2.15.0": "2.15.0",
	"2.16.0": "2.16.0",
	"2.18.0": "2.18.0",
	"2.20.0": "2.18.0",
	"2.21.0": "2.21.0",
	"2.22.0": "2.21.0",
	"2.23.0": "2.21.0",
	"2.25.0": "2.21.0",
	"2.26.0": "2.21.0",
	"2.27.0": "2.21.0",
	"2.28.0": "2.21.0",
}

var YacfgProfileName string = "artemis"
