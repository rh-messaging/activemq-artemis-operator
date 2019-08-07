package namer

type NamerInterface interface {
	Name() string
	Base(baseName string) *NamerData
	Prefix(namePrefix string) *NamerData
	Suffix(nameSuffix string) *NamerData
	Generate()
}

type NamerData struct {
	baseName string
	prefix   string
	suffix   string
	name     string
}

func (n *NamerData) Generate() {
	if len(n.prefix) > 0 {
		n.name = n.prefix + "-" + n.baseName
	} else {
		n.name = n.baseName
	}
	if len(n.suffix) > 0 {
		n.name = n.name + "-" + n.suffix
	}
}

//func NewNamer() *NamerData {
//	var namer NamerData
//	return &namer
//}

func (n *NamerData) Base(baseName string) *NamerData {
	n.baseName = baseName
	return n
}

func (n *NamerData) Prefix(namePrefix string) *NamerData {
	n.prefix = namePrefix
	return n
}

func (n *NamerData) Suffix(nameSuffix string) *NamerData {
	n.suffix = nameSuffix
	return n
}

func (n *NamerData) Name() string {
	return n.name
}
