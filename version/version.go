package version

import "strings"

var (
	Version = "7.11.0.OPR.1"
	// PriorVersion - prior version
	PriorVersion = "7.10.2.OPR.2"

	//Vars injected at build-time
	BuildTimestamp = ""
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.11.0"
	CompactLatestVersion = "7110"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.10.0"

	LatestKubeImage = "registry.redhat.io/amq7/amq-broker-rhel8:7.11"
	LatestInitImage = "registry.redhat.io/amq7/amq-broker-init-rhel8:7.11"
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

var CompactVersionFromVersion map[string]string = map[string]string{
	"7.7.0":  "770",
	"7.8.0":  "780",
	"7.8.1":  "781",
	"7.8.2":  "782",
	"7.8.3":  "783",
	"7.9.0":  "790",
	"7.9.1":  "791",
	"7.9.2":  "792",
	"7.9.3":  "793",
	"7.9.4":  "794",
	"7.10.0": "7100",
	"7.10.1": "7101",
	"7.10.2": "7102",
	"7.11.0": "7110",
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
}

//The yacfg profile to use for a given full version of broker
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
}

var YacfgProfileName string = "amq_broker"
