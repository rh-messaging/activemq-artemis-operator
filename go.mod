module github.com/artemiscloud/activemq-artemis-operator

go 1.16

require (
	//this module is problematic
	github.com/RHsyseng/operator-utils v1.4.7
	github.com/artemiscloud/activemq-artemis-management v0.0.0-20211124154607-0acf12a607ee
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/openshift/api v3.9.0+incompatible
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.0
)
