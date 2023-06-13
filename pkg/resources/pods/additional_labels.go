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
	"7.9.1":  "2021.Q4",
	"7.9.2":  "2022.Q1",
	"7.9.3":  "2022.Q1",
	"7.9.4":  "2022.Q2",
	"7.10.0": "2022.Q2",
	"7.10.1": "2022.Q3",
	"7.10.2": "2022.Q4",
	"7.11.0": "2023.Q1",
	"7.11.1": "2023.Q2",
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
