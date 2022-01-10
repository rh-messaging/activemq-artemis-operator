package version

var (
	Version = "7.8.4"
	// PriorVersion - prior version
	PriorVersion = "7.8.3"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.8.4"
	CompactLatestVersion = "784"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.8.3"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.7.0"
)

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
	"7.8.4":  "7.8.4",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "7.7.0",
	"80": "7.8.0",
	"81": "7.8.1",
	"82": "7.8.2",
	"83": "7.8.3",
	"84": "7.8.4",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "770",
	"80": "780",
	"81": "781",
	"82": "782",
	"83": "783",
	"84": "784",
}

var CompactVersionFromVersion map[string]string = map[string]string{
	"7.7.0": "770",
	"7.8.0": "780",
	"7.8.1": "781",
	"7.8.2": "782",
	"7.8.3": "783",
	"7.8.4": "784",
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"770": "7.7.0",
	"780": "7.8.0",
	"781": "7.8.1",
	"782": "7.8.2",
	"783": "7.8.3",
	"784": "7.8.4",
}

var MinorVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0": "70",
	"7.8.0": "80",
	"7.8.1": "81",
	"7.8.2": "82",
	"7.8.3": "83",
	"7.8.4": "84",
}

//The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0": "7.7.0",
	"7.8.0": "7.8.0",
	"7.8.1": "7.8.1",
	"7.8.2": "7.8.2",
	"7.8.3": "7.8.2",
	"7.8.4": "7.8.2",
}
var YacfgProfileName string = "amq_broker"
