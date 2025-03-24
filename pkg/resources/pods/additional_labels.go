package pods

var labelsForRh map[string]string = map[string]string{
	"com.company":   "Red_Hat",
	"rht.prod_name": "Red_Hat_Integration",
	"rht.comp":      "Broker_AMQ",
	"rht.subcomp":   "broker-amq",
	"rht.subcomp_t": "application",
}

// going back in time, coalesce on this value
const DEFAULT_PROD_VER = "2021.Q4"

var productVerFromImageVer map[string]string = map[string]string{
	// Product versions i.e. "7.10.0": "2022.Q2"
	"7.12.0": "2024.Q2",
	"7.12.1": "2024.Q3",
	"7.12.2": "2024.Q3",
	"7.12.3": "2025.Q1",
	"7.13.0": "2025.Q1",
}

// the labels returned will be added to broker pod
func GetAdditionalLabels(fullBrokerVersion string) map[string]string {
	labels := make(map[string]string)
	for k, v := range labelsForRh {
		labels[k] = v
	}
	// track image version to ensure labels don't change for a given image
	labels["rht.comp_ver"] = fullBrokerVersion

	// prod_ver needs to mapped and remembered as this is date driven
	var prodVer, found = productVerFromImageVer[fullBrokerVersion]
	if !found {
		prodVer = DEFAULT_PROD_VER
	}
	labels["rht.prod_ver"] = prodVer

	return labels
}
