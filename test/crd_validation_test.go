package test

import (
	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"
	brokerv1alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"strings"
	"testing"
)

var crNameMap = map[string]string {
	"activemqartemis_cr.yaml": 			"broker_v1alpha1_activemqartemis_crd.yaml",
	"activemqartemisaddress_cr.yaml": 	"broker_v1alpha1_activemqartemisaddress_crd.yaml",
	"address-queue-create.yaml": 		"broker_v1alpha1_activemqartemisaddress_crd.yaml",
	"amq-basic-deployment.yaml": 		"broker_v1alpha1_activemqartemis_crd.yaml",
	"amq-ssl-deployment.yaml": 			"broker_v1alpha1_activemqartemis_crd.yaml",
}

var crdTypeMap = map[string]interface{} {
	"broker_v1alpha1_activemqartemis_crd.yaml":			&brokerv1alpha1.ActiveMQArtemis{},
	"broker_v1alpha1_activemqartemisaddress_crd.yaml": 	&brokerv1alpha1.ActiveMQArtemisAddress{},
}

func TestCRDSchemas(t *testing.T) {
	for crdFileName, amqType := range crdTypeMap {
		schema := getSchema(t, crdFileName)
		missingEntries := schema.GetMissingEntries(amqType)
		for _, missing := range missingEntries {
			if strings.HasPrefix(missing.Path, "/status") {
				//Not using subresources, so status is not expected to appear in CRD
			} else {
				assert.Fail(t, "Discrepancy between CRD and Struct",
					"Missing or incorrect schema validation at %v, expected type %v  in CRD file %v", missing.Path, missing.Type, crdFileName)
			}
		}
	}
}

func TestSampleCustomResources(t *testing.T) {

	// fetch list of all CRD files
	fileList, err := ioutil.ReadDir("../deploy/crds")
	assert.NoError(t, err, "Error reading CR Files %v", fileList)

	var input map[string]interface{}
	for _, file := range fileList {

		if !strings.HasSuffix(file.Name(), "cr.yaml") {
			continue // if we move cr.yaml and crd.yaml files into sep directories, this check can go away
		}

		// determine which cr/crd pairing in use for all *cr.yaml files
		var crFileName, crdFileName string
		for cr, crd := range crNameMap {
			if strings.HasSuffix(file.Name(), cr) {
				crFileName, crdFileName = file.Name(), crd
				break
			}

		}

		assert.NotEmpty(t, crdFileName, "No matching CRD file found for CR suffixed: %s", crFileName)
		schema := getSchema(t, crdFileName)
		yamlString, err := ioutil.ReadFile("../deploy/crds/" + crFileName)
		assert.NoError(t, err, "Error reading %v CR yaml", crFileName)
		assert.NoError(t, yaml.Unmarshal([]byte(yamlString), &input))
		assert.NoError(t, schema.Validate(input), "File %v does not validate against the CRD schema", file)
	}
}

func TestExampleCustomResources(t *testing.T) {

	// fetch list of all CR files
	fileList, err := ioutil.ReadDir("../deploy/examples")
	assert.NoError(t, err, "Error reading CR Files %v", fileList)
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
		assert.NotEmpty(t, crdFileName, "No matching CRD file found for CR suffixed: %s", crFileName)
		schema := getSchema(t, crdFileName)
		yamlString, err := ioutil.ReadFile("../deploy/examples/" + crFileName)
		assert.NoError(t, err, "Error reading %v CR yaml", crFileName)
		assert.NoError(t, yaml.Unmarshal([]byte(yamlString), &input))
		assert.NoError(t, schema.Validate(input), "File %v does not validate against the CRD schema", file)
	}
}

func getSchema(t *testing.T, crdFile string) validation.Schema {

	yamlString, err := ioutil.ReadFile("../deploy/crds/" + crdFile)
	assert.NoError(t, err, "Error reading CRD yaml %v", yamlString)

	schema, err := validation.New([]byte(yamlString))
	assert.NoError(t, err)

	return schema
}

