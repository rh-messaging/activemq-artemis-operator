package selectors

const (
	LabelAppKey             = "application"
	LabelActiveMQArtemisKey = "ActiveMQArtemis"
	LabelBrokerKey          = "broker"
)

type LabelerInterface interface {
	Labels() map[string]string
	Base(baseName string) *LabelerData
	Suffix(labelSuffix string) *LabelerData
	Generate()
}

type LabelerData struct {
	baseName    string
	suffix      string
	resourceKey string
	labels      map[string]string
}

func NewBrokerLabeler() *LabelerData {
	return &LabelerData{resourceKey: LabelBrokerKey}
}

func NewActiveMQArtemisLabeler() *LabelerData {
	return &LabelerData{resourceKey: LabelActiveMQArtemisKey}
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
	l.labels[l.resourceKey] = l.baseName
}

func GetLabels(crName string) map[string]string {
	labelBuilder := NewActiveMQArtemisLabeler()
	labelBuilder.Base(crName).Suffix("app").Generate()
	return labelBuilder.Labels()
}
