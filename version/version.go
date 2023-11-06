package version

import "strings"

var (
	Version = "7.10.5.OPR.1"
	// PriorVersion - prior version
	PriorVersion = "7.9.4"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.10.5"
	CompactLatestVersion = "7105"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.9.4"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.7.0"

	LatestKubeImage = "registry.redhat.io/amq7/amq-broker-rhel8:7.10.5"
	LatestInitImage = "registry.redhat.io/amq7/amq-broker-init-rhel8:7.10.5"
)

func DefaultImageName(archSpecificRelatedImageEnvVarName string) string {
	if strings.Contains(archSpecificRelatedImageEnvVarName, "_Init_") {
		return LatestInitImage
	} else {
		return LatestKubeImage
	}
}

// SupportedVersions - product versions this operator supports
var SupportedVersions = []string{LatestVersion, LastMicroVersion, LastMinorVersion}
var SupportedMicroVersions = []string{LatestVersion, LastMicroVersion}
var OperandVersionFromOperatorVersion map[string]string = map[string]string{
	"0.17.0": "7.7.0",
	"0.18.0": "7.8.0",
	"0.19.0": "7.8.1",
	"7.8.1":  "7.8.1",
	"7.8.2":  "7.8.2",
	"7.8.3":  "7.8.3",
	"7.9.0":  "7.9.0",
	"7.9.1":  "7.9.1",
	"7.9.2":  "7.9.2",
	"7.9.3":  "7.9.3",
	"7.9.4":  "7.9.4",
	"7.10.0": "7.10.0",
	"7.10.1": "7.10.1",
	"7.10.2": "7.10.2",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"70":  "7.7.0",
	"80":  "7.8.0",
	"81":  "7.8.1",
	"82":  "7.8.2",
	"83":  "7.8.3",
	"90":  "7.9.0",
	"91":  "7.9.1",
	"92":  "7.9.2",
	"93":  "7.9.3",
	"94":  "7.9.4",
	"100": "7.10.0",
	"101": "7.10.1",
	"102": "7.10.2",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"70":  "770",
	"80":  "780",
	"81":  "781",
	"82":  "782",
	"83":  "783",
	"90":  "790",
	"91":  "791",
	"92":  "792",
	"93":  "793",
	"94":  "794",
	"100": "7100",
	"101": "7101",
	"102": "7102",
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
	"7.10.3": "7103",
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
	"7103": "7.10.3",
	"7104": "7.10.4",
	"7105": "7.10.5",
}

var MinorVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0":  "70",
	"7.8.0":  "80",
	"7.8.1":  "81",
	"7.8.2":  "82",
	"7.8.3":  "83",
	"7.9.0":  "90",
	"7.9.1":  "91",
	"7.9.2":  "92",
	"7.9.3":  "93",
	"7.9.4":  "94",
	"7.10.0": "100",
	"7.10.1": "101",
	"7.10.2": "102",
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
	"7.10.3": "7.10.0",
	"7.10.4": "7.10.0",
	"7.10.5": "7.10.0",
}

var YacfgProfileName string = "amq_broker"
