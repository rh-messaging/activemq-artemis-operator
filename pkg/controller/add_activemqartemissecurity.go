package controller

import (
	v1alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v1alpha1/activemqartemissecurity"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, v1alpha1.Add)
}
