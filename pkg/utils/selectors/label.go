package selectors

import (
	"k8s.io/apimachinery/pkg/labels"
)

const (
	LabelAppKey      = "application"
	LabelResourceKey = "ActiveMQArtemis"
)

type LabelerInterface interface {
	Labels() map[string]string
	Base(baseName string) *LabelerData
	Suffix(labelSuffix string) *LabelerData
	Generate()
}

type LabelerData struct {
	baseName string
	suffix   string
	labels   map[string]string
}

func (l *LabelerData) Labels() map[string]string {
	return l.labels
}

func (l *LabelerData) Base(name string) *LabelerData {
	l.baseName = name
	return l
}

func (l *LabelerData) Suffix(labelSuffix string) *LabelerData {
	l.suffix = labelSuffix
	return l
}

func (l *LabelerData) Generate() {
	l.labels = make(map[string]string)
	l.labels[LabelAppKey] = l.baseName + "-" + l.suffix //"-app"
	l.labels[LabelResourceKey] = l.baseName
}

// return a selector that matches resources for a ActiveMQArtemis resource
func ResourcesByActiveMQArtemisName(name string) labels.Selector {

	set := map[string]string{
		LabelAppKey: name,
		//LabelResourceKey: baseName,
	}

	return labels.SelectorFromSet(set)
}

var LabelBuilder LabelerData