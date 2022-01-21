package version

var (
	Version = "7.9.2"
	// PriorVersion - prior version
	PriorVersion = "7.9.1"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.9.2"
	CompactLatestVersion = "792"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.9.1"
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
	"7.9.0":  "7.9.0",
	"7.9.1":  "7.9.1",
	"7.9.2":  "7.9.2",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "7.7.0",
	"80": "7.8.0",
	"81": "7.8.1",
	"82": "7.8.2",
	"83": "7.8.3",
	"90": "7.9.0",
	"91": "7.9.1",
	"92": "7.9.2",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "770",
	"80": "780",
	"81": "781",
	"82": "782",
	"83": "783",
	"90": "790",
	"91": "791",
	"92": "792",
}

var CompactVersionFromVersion map[string]string = map[string]string{
	"7.7.0": "770",
	"7.8.0": "780",
	"7.8.1": "781",
	"7.8.2": "782",
	"7.8.3": "783",
	"7.9.0": "790",
	"7.9.1": "791",
	"7.9.2": "792",
}

var FullVersionFromCompactVersion map[string]string = map[string]string{
	"770": "7.7.0",
	"780": "7.8.0",
	"781": "7.8.1",
	"782": "7.8.2",
	"783": "7.8.3",
	"790": "7.9.0",
	"791": "7.9.1",
	"792": "7.9.2",
}

var MinorVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0": "70",
	"7.8.0": "80",
	"7.8.1": "81",
	"7.8.2": "82",
	"7.8.3": "83",
	"7.9.0": "90",
	"7.9.1": "91",
	"7.9.2": "92",
}

//The yacfg profile to use for a given full version of broker
var YacfgProfileVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0": "7.7.0",
	"7.8.0": "7.8.0",
	"7.8.1": "7.8.1",
	"7.8.2": "7.8.2",
	"7.8.3": "7.8.2",
	"7.9.0": "7.9.0",
	"7.9.1": "7.9.0",
	"7.9.2": "7.9.0",
}

var YacfgProfileName string = "amq_broker"
