package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha4"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/cr2jinja2"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	//"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"bytes"
	//"os"
	//"gopkg.in/yaml.v2"
	//"encoding/json"
)

/*
Usage cr2jinja2 [options] broker_cr.yaml
options:
-e the environment variable
-o the output file
-v the cr version, e.g. v2alpha4
*/
func main() {

	envVar := flag.String("e", "", "env var")
	outputVar := flag.String("o", "default.yaml", "output file")
	crVersion := flag.String("v", "v2alpha4", "cr version")

	flag.Parse()

	fmt.Println("-e:", *envVar)
	fmt.Println("-o", *outputVar)
	fmt.Println("-v", *crVersion)
	fmt.Println("tail: ", flag.Args())
	if len(flag.Args()) != 1 {
		fmt.Println("you need to pass in the cr file")
		return
	}
	crFile := flag.Args()[0]
	fmt.Println("cr file " + crFile)
	yamlFile, err := ioutil.ReadFile(crFile)

	if err != nil {
		fmt.Println("err", err)
		return
	}

	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFile), 1024)

	var result string

	if "v2alpha3" == *crVersion {
		fmt.Println("cr3 is used")
		brokerCr := v2alpha3.ActiveMQArtemis{}
		result = processCr(decoder, envVar, outputVar, &brokerCr)
	} else if "v2alpha4" == *crVersion {
		fmt.Println("cr4 is used")
		brokerCr := v2alpha4.ActiveMQArtemis{}
		result = processCr(decoder, envVar, outputVar, &brokerCr)
	} else {
		fmt.Printf("Unsupported CR version: %s\n", *crVersion)
		return
	}

	fmt.Println("result: \n" + result)

}

func processCr(decoder *k8syaml.YAMLOrJSONDecoder, envVar *string, outputVar *string, brokerCr interface{}) string {

	err := decoder.Decode(brokerCr)

	if err != nil {
		fmt.Printf("Error parsing YAML file: %s\n", err)
		return ""
	}

	result, _ := cr2jinja2.MakeBrokerCfgOverrides(brokerCr, envVar, outputVar)

	return result
}
