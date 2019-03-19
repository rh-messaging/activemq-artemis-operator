package selectors

import (
	"k8s.io/apimachinery/pkg/labels"
)

const (
	LabelAppKey = "application"

	LabelResourceKey = "ActiveMQArtemis"
)

// Set labels in a map
func LabelsForActiveMQArtemis(name string) map[string]string {
	return map[string]string{
		LabelAppKey:      name + "-app",
		LabelResourceKey: name,
	}
}

// return a selector that matches resources for a ActiveMQArtemis resource
func ResourcesByActiveMQArtemisName(name string) labels.Selector {
	set := map[string]string{
		LabelAppKey: name,
		//LabelResourceKey: name,
	}
	return labels.SelectorFromSet(set)
}
