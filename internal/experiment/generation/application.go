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
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ApplicationSelector is responsible for "scanning" the application definition itself.
type ApplicationSelector struct {
	Application *optimizeappsv1alpha1.Application
	Scenario    *optimizeappsv1alpha1.Scenario
	Objective   *optimizeappsv1alpha1.Objective
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

	// NOTE: We iterate by index and obtain pointers because some of these evaluate for side effects

	if s.Scenario != nil {
		switch {
		case s.Scenario.StormForger != nil:
			result = append(result, &StormForgerSource{Scenario: s.Scenario, Objective: s.Objective, Application: s.Application})
		case s.Scenario.Locust != nil:
			result = append(result, &LocustSource{Scenario: s.Scenario, Objective: s.Objective, Application: s.Application})
		case s.Scenario.Custom != nil:
			result = append(result, &CustomSource{Scenario: s.Scenario, Objective: s.Objective, Application: s.Application})
		}
	}

	if s.Objective != nil {
		for i := range s.Objective.Goals {
			switch {
			case s.Objective.Goals[i].Requests != nil:
				result = append(result, &RequestsMetricsSource{Goal: &s.Objective.Goals[i]})
			case s.Objective.Goals[i].Duration != nil:
				result = append(result, &DurationMetricsSource{Goal: &s.Objective.Goals[i]})
			case s.Objective.Goals[i].Prometheus != nil:
				result = append(result, &PrometheusMetricsSource{Goal: &s.Objective.Goals[i]})
			case s.Objective.Goals[i].Datadog != nil:
				result = append(result, &DatadogMetricsSource{Goal: &s.Objective.Goals[i]})
			}
		}
	}

	result = append(result, &BuiltInPrometheus{
		SetupTaskName:          "monitoring",
		ClusterRoleName:        "optimize-prometheus",
		ServiceAccountName:     "optimize-setup",
		ClusterRoleBindingName: "optimize-setup-prometheus",
	})

	return result, nil
}
