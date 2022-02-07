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
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Background represents a procedure that runs in the background, periodically auto-detecting features
type Background struct {
	dc     discovery.DiscoveryInterface
	ticker *time.Ticker
}

// New creates a new auto-detect runner
func NewAutoDetect(mgr manager.Manager) (*Background, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	return &Background{dc: dc}, nil
}

// Start initializes the auto-detection process that runs in the background
func (b *Background) Start() {
	b.autoDetectCapabilities()
	// periodically attempts to auto detect all the capabilities for this operator
	b.ticker = time.NewTicker(5 * time.Second)

	go func() {
		for range b.ticker.C {
			b.autoDetectCapabilities()
		}
	}()
}

// Stop causes the background process to stop auto detecting capabilities
func (b *Background) Stop() {
	b.ticker.Stop()
}

func (b *Background) autoDetectCapabilities() {
	b.detectOpenshift()
	b.detectMonitoringResources()
	b.detectRoute()
}

func (b *Background) detectRoute() {
	resourceExists, _ := ResourceExists(b.dc, routev1.SchemeGroupVersion.String(), RouteKind)
	if resourceExists {
		// Set state that the Route kind exists. Used to determine when a route or an Ingress should be created
		stateManager := GetStateManager()
		stateManager.SetState(RouteKind, true)
	}
}

func (b *Background) detectMonitoringResources() {
	// detect the PrometheusRule resource type exist on the cluster
}

func (b *Background) detectOpenshift() {
	apiGroupVersion := "operator.openshift.io/v1"
	kind := OpenShiftAPIServerKind
	stateManager := GetStateManager()
	isOpenshift, _ := ResourceExists(b.dc, apiGroupVersion, kind)
	if isOpenshift {
		// Set state that its Openshift (helps to differentiate between openshift and kubernetes)
		stateManager.SetState(OpenShiftAPIServerKind, true)
	} else {
		stateManager.SetState(OpenShiftAPIServerKind, false)
	}
}

func ResourceExists(dc discovery.DiscoveryInterface, apiGroupVersion, kind string) (bool, error) {
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}
	for _, apiList := range apiLists {
		if apiList.GroupVersion == apiGroupVersion {
			for _, r := range apiList.APIResources {
				if r.Kind == kind {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
