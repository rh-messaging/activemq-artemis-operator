package integration

import (
	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	brokerv2alpha2 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha2"

	"io/ioutil"
	"strings"
)

var crdTypeMap = map[string]interface{}{
	"broker_activemqartemis_crd.yaml":                 &brokerv2alpha2.ActiveMQArtemis{},
	"broker_v2alpha1_activemqartemisaddress_crd.yaml": &brokerv2alpha1.ActiveMQArtemisAddress{},
}

var crNameMap = map[string]string{
	"activemqartemis_cr.yaml":                          "broker_activemqartemis_crd.yaml",
	"activemqartemisaddress_cr.yaml":                   "broker_v2alpha1_activemqartemisaddress_crd.yaml",
	"address-queue-create.yaml":                        "broker_v2alpha1_activemqartemisaddress_crd.yaml",
	"artemis-basic-deployment.yaml":                    "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-ssl-deployment.yaml":                      "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-cluster-deployment.yaml":                  "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-persistence-deployment.yaml":              "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-ssl-persistence-cluster-deployment.yaml":  "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-ssl-persistence-deployment.yaml":          "broker_v2alpha1_activemqartemis_crd.yaml",
	"artemis-aio-journal.yaml":                         "broker_v2alpha1_activemqartemis_crd.yaml",
	"broker_v2alpha1_activemqartemisscaledown_cr.yaml": "broker_v2alpha1_activemqartemisscaledown_crd.yaml",
}

var _ = ginkgo.Describe("CRD Validation Test", func() {

	ginkgo.It("Test CRD Schema", func() {
		for crdFileName, amqType := range crdTypeMap {
			schema := getSchema(crdFileName)
			missingEntries := schema.GetMissingEntries(amqType)
			for _, missing := range missingEntries {
				gomega.Expect(strings.HasPrefix(missing.Path, "/status")).To(gomega.BeTrue(), "Discrepancy between CRD and Struct",
					"Missing or incorrect schema validation at %v, expected type %v  in CRD file %v", missing.Path, missing.Type, crdFileName)
			}
		}
	})

	ginkgo.It("Test Sample Custom Resources", func() {
		testCustomResource("../../deploy/crs")
	})

	ginkgo.It("Test Example Custome Resources", func() {
		testCustomResource("../../deploy/examples")
	})
})

func testCustomResource(resLoc string) {
	fileList, err := ioutil.ReadDir(resLoc)
	gomega.Expect(err).To(gomega.BeNil())

	var input map[string]interface{}
	for _, file := range fileList {

		//if !strings.HasSuffix(file.Name(), "cr.yaml") {
		//	continue // if we move cr.yaml and crd.yaml files into sep directories, this check can go away
		//}

		// determine which cr/crd pairing in use for all *cr.yaml files
		var crFileName, crdFileName string
		for cr, crd := range crNameMap {
			if strings.HasSuffix(file.Name(), cr) {
				crFileName, crdFileName = file.Name(), crd
				break
			}
		}
		println(crFileName, crdFileName)
		gomega.Expect(crdFileName).ShouldNot(gomega.BeEmpty(), "No matching CRD file found for CR suffixed: %s", crFileName)
		schema := getSchema(crdFileName)
		yamlString, err := ioutil.ReadFile(resLoc + "/" + crFileName)

		gomega.Expect(err).To(gomega.BeNil(), "Error reading %v CR yaml", crFileName)
		gomega.Ω(yaml.Unmarshal([]byte(yamlString), &input)).Should(gomega.Succeed())
		gomega.Expect(schema.Validate(input)).Should(gomega.Succeed(), "File %v does not validate against the CRD schema", file)
	}
}

func getSchema(crdFile string) validation.Schema {

	yamlString, err := ioutil.ReadFile("../../deploy/crds/" + crdFile)
	gomega.Expect(err).Should(gomega.Succeed(), "Error reading CRD yaml %v", yamlString)

	schema, err := validation.New([]byte(yamlString))
	gomega.Expect(err).Should(gomega.Succeed())

	return schema
}
