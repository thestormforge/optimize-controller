/*
Copyright 2020 GramLabs, Inc.

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

package generation

import (
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ApplicationSelector is responsible for "scanning" the application definition itself.
type ApplicationSelector struct {
	Application *redskyappsv1alpha1.Application
}

var _ scan.Selector = &ApplicationSelector{}

// Select only returns an empty node to ensure that map will be called.
func (s *ApplicationSelector) Select([]*yaml.RNode) ([]*yaml.RNode, error) {
	// In order to evaluate side effects on the application state, we CANNOT introduce the serialized
	// application into the resource node stream and process it from there. We still need to
	// return exactly one node here to make sure `Map` gets called.
	return []*yaml.RNode{yaml.MakeNullNode()}, nil
}

// Map ignores inputs (it should just be the null node from select) and produces additional markers for transform.
func (s *ApplicationSelector) Map(*yaml.RNode, yaml.ResourceMeta) ([]interface{}, error) {
	var result []interface{}
	var needsPrometheus bool

	// NOTE: We iterate by index and obtain pointers because some of these evaluate for side effects

	for i := range s.Application.Scenarios {
		scenario := &s.Application.Scenarios[i]
		switch {
		case scenario.StormForger != nil:
			result = append(result, &StormForgerSource{Scenario: scenario, Application: s.Application})
			needsPrometheus = true
		case scenario.Locust != nil:
			result = append(result, &LocustSource{Scenario: scenario, Application: s.Application})
			needsPrometheus = true
		}
	}

	for i := range s.Application.Objectives {
		objective := &s.Application.Objectives[i]
		switch {
		case objective.Implemented:
			// Do nothing, there must have been a scenario specific implementation
		case objective.Requests != nil:
			result = append(result, &RequestsMetricsSource{Objective: objective})
			needsPrometheus = true
		case objective.Duration != nil:
			result = append(result, &DurationMetricsSource{Objective: objective})
		}
	}

	if needsPrometheus {
		result = append(result, &BuiltInPrometheus{
			SetupTaskName:          "monitoring",
			ClusterRoleName:        "redsky-prometheus",
			ServiceAccountName:     "redsky-setup",
			ClusterRoleBindingName: "redsky-setup-prometheus",
		})
	}

	return result, nil
}
