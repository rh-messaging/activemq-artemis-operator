package version

var (
	Version = "0.19.0"
	// PriorVersion - prior version
	PriorVersion = "0.18.0"
)

const (
	// LatestVersion product version supported
	LatestVersion        = "7.8.1"
	CompactLatestVersion = "781"
	// LastMicroVersion product version supported
	LastMicroVersion = "7.8.0"
	// LastMinorVersion product version supported
	LastMinorVersion = "7.7.0"
)

// SupportedVersions - product versions this operator supports
var SupportedVersions = []string{LatestVersion, LastMicroVersion, LastMinorVersion}
var OperandVersionFromOperatorVersion map[string]string = map[string]string{
	"0.17.0": "7.7.0",
	"0.18.0": "7.8.0",
	"0.19.0": "7.8.1",
}
var FullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "7.7.0",
	"80": "7.8.0",
	"81": "7.8.1",
}

var CompactFullVersionFromMinorVersion map[string]string = map[string]string{
	"70": "770",
	"80": "780",
	"81": "781",
}

var CompactVersionFromVersion map[string]string = map[string]string{
	"7.7.0": "770",
	"7.8.0": "780",
	"7.8.1": "781",
}

var MinorVersionFromFullVersion map[string]string = map[string]string{
	"7.7.0": "70",
	"7.8.0": "80",
	"7.8.1": "81",
}
