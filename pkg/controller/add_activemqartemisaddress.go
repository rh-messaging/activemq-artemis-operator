package controller

import (
	v2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha3/activemqartemisaddress"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, v2alpha3.Add)
}
