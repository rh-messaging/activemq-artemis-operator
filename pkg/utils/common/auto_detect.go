/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package common

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type AutoDetector struct {
	dc discovery.DiscoveryInterface
}

// New creates a new auto-detect runner
func NewAutoDetect(mgr manager.Manager) (*AutoDetector, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	return &AutoDetector{dc: dc}, nil
}

func (b *AutoDetector) DetectOpenshift() error {
	stateManager := GetStateManager()
	isOpenshift, err := discovery.IsResourceEnabled(b.dc,
		schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "openshiftapiservers",
		})

	if err != nil {
		return err
	}

	if isOpenshift {
		// Set state that its Openshift (helps to differentiate between openshift and kubernetes)
		stateManager.SetState(OpenShiftAPIServerKind, true)
	} else {
		stateManager.SetState(OpenShiftAPIServerKind, false)
	}
	return nil
}
