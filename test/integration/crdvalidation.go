package integration

import (
	"github.com/RHsyseng/operator-utils/pkg/validation"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	brokerv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/ghodss/yaml"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"io/ioutil"
	"strings"
)

var crdTypeMap = map[string]interface{}{
	"broker_activemqartemis_crd.yaml":                 &brokerv2alpha3.ActiveMQArtemis{},
	"broker_v2alpha1_activemqartemisaddress_crd.yaml": &brokerv2alpha1.ActiveMQArtemisAddress{},
}

var crdVersionMap = map[string]string{
	"broker_activemqartemis_crd.yaml":                 "v2alpha3",
	"broker_v2alpha1_activemqartemisaddress_crd.yaml": "",
}

var crNameMap = map[string]string{
	"activemqartemis_cr.yaml":                                "broker_activemqartemis_crd.yaml",
	"activemqartemisaddress_cr.yaml":                         "broker_activemqartemisaddress_crd.yaml",
	"address-queue-create.yaml":                              "broker_activemqartemisaddress_crd.yaml",
	"address-queue-create-auto-removed.yaml":                 "broker_activemqartemisaddress_crd.yaml",
	"artemis-basic-deployment.yaml":                          "broker_activemqartemis_crd.yaml",
	"artemis-ssl-deployment.yaml":                            "broker_activemqartemis_crd.yaml",
	"artemis-cluster-deployment.yaml":                        "broker_activemqartemis_crd.yaml",
	"artemis-persistence-deployment.yaml":                    "broker_activemqartemis_crd.yaml",
	"artemis-ssl-persistence-cluster-deployment.yaml":        "broker_activemqartemis_crd.yaml",
	"artemis-ssl-persistence-deployment.yaml":                "broker_activemqartemis_crd.yaml",
	"artemis-aio-journal.yaml":                               "broker_activemqartemis_crd.yaml",
	"artemis-basic-address-settings-deployment.yaml":         "broker_activemqartemis_crd.yaml",
	"artemis-basic-resources-deployment.yaml":                "broker_activemqartemis_crd.yaml",
	"artemis-merge-replace-address-settings-deployment.yaml": "broker_activemqartemis_crd.yaml",
	"artemis-replace-address-settings-deployment.yaml":       "broker_activemqartemis_crd.yaml",

	"broker_activemqartemisscaledown_cr.yaml": "broker_activemqartemisscaledown_crd.yaml",
}

var _ = ginkgo.Describe("CRD Validation Test", func() {

	//mark this test as Pending because
	//https://github.com/artemiscloud/activemq-artemis-operator/issues/19
	ginkgo.It("Test CRD Schema", func() {
		ginkgo.Skip("*** Skipping test for know issue #19")
		for crdFileName, amqType := range crdTypeMap {
			version := crdVersionMap[crdFileName]
			schema := getSchema(crdFileName, version)
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
		schema := getSchema(crdFileName, crdVersionMap[crdFileName])
		yamlString, err := ioutil.ReadFile(resLoc + "/" + crFileName)

		gomega.Expect(err).To(gomega.BeNil(), "Error reading %v CR yaml", crFileName)
		gomega.Ω(yaml.Unmarshal([]byte(yamlString), &input)).Should(gomega.Succeed())
		gomega.Expect(schema.Validate(input)).Should(gomega.Succeed(), "File %v does not validate against the CRD schema", file)
	}
}

func getSchema(crdFile string, version string) validation.Schema {

	yamlString, err := ioutil.ReadFile("../../deploy/crds/" + crdFile)
	gomega.Expect(err).Should(gomega.Succeed(), "Error reading CRD yaml %v", yamlString)

	var schema validation.Schema

	if "" != version {
		schema, err = validation.NewVersioned([]byte(yamlString), version)
	} else {
		schema, err = validation.New([]byte(yamlString))
	}
	gomega.Expect(err).Should(gomega.Succeed())

	return schema
}
