package selectors

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

func GetLabels(crName string) map[string]string {
	labelBuilder := LabelerData{}
	labelBuilder.Base(crName).Suffix("app").Generate()
	return labelBuilder.labels
}
