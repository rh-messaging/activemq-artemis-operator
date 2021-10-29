package pods

var labelsFor7_9_1 map[string]string = map[string]string{
	"com.company":   "Red_Hat",
	"rht.prod_name": "Red_Hat_Integration",
	"rht.prod_ver":  "2021.Q4",
	"rht.comp":      "Broker_AMQ",
	"rht.comp_ver":  "7.9.1",
	"rht.subcomp":   "broker-amq",
	"rht.subcomp_t": "application",
}

var labelsFromVersion map[string]map[string]string = map[string]map[string]string{
	"7.9.1": labelsFor7_9_1,
}

// the labels returned will be added to broker pod
func GetAdditionalLabels(fullBrokerVersion string) map[string]string {
	if labels, ok := labelsFromVersion[fullBrokerVersion]; ok {
		return labels
	}
	return nil
}
