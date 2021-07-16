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

package application

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

// GetScenario returns the named scenario from the application, if possible.
func GetScenario(app *optimizeappsv1alpha1.Application, scenario string) (*optimizeappsv1alpha1.Scenario, error) {
	if scenario == "" && len(app.Scenarios) == 1 {
		return &app.Scenarios[0], nil
	}

	if len(app.Scenarios) == 0 {
		return nil, nil
	}

	cleanScenario := cleanName(scenario)
	var result *optimizeappsv1alpha1.Scenario
	var names []string
	for i := range app.Scenarios {
		names = append(names, app.Scenarios[i].Name)
		if cleanName(app.Scenarios[i].Name) != cleanScenario {
			continue
		}

		if result != nil {
			return nil, fmt.Errorf("scenario name %q is ambiguous", scenario)
		}

		result = &app.Scenarios[i]
	}

	if result != nil {
		return result, nil
	}

	if scenario == "" {
		return nil, fmt.Errorf("must specify a scenario, allowed values are: %s", strings.Join(names, ", "))
	}

	return nil, fmt.Errorf("unknown scenario %q (expected one of: %s)", scenario, strings.Join(names, ", "))
}

// GetObjective returns the goals of an objective, if possible.
func GetObjective(app *optimizeappsv1alpha1.Application, objective string) (*optimizeappsv1alpha1.Objective, error) {
	if objective == "" && len(app.Objectives) == 1 {
		return &app.Objectives[0], nil
	}

	if len(app.Objectives) == 0 {
		return nil, nil
	}

	cleanObjective := cleanName(objective)
	var result *optimizeappsv1alpha1.Objective
	var names []string
	for i := range app.Objectives {
		names = append(names, app.Objectives[i].Name)
		if cleanName(app.Objectives[i].Name) != cleanObjective {
			continue
		}

		if result != nil {
			return nil, fmt.Errorf("objective name %q is ambiguous", objective)
		}

		result = &app.Objectives[i]
	}

	if result != nil {
		return result, nil
	}

	if objective == "" {
		return nil, fmt.Errorf("must specify an objective, allowed values are: %s", strings.Join(names, ", "))
	}

	return nil, fmt.Errorf("unknown objective %q (expected one of: %s)", objective, strings.Join(names, ", "))
}

// ExperimentName returns the name of an experiment corresponding to the application.
func ExperimentName(app *optimizeappsv1alpha1.Application, scenario, objective string) string {
	name := cleanName(app.Name)

	// Cap name length to 54 so we can add 9 more characters before hitting 63
	if len(name) > 54 {
		name = name[0:54]
	}

	// Hash the scenario and objective names
	h := sha1.New()
	_, _ = h.Write([]byte(cleanName(scenario)))
	_, _ = h.Write([]byte(cleanName(objective)))

	return fmt.Sprintf("%s-%x", name, h.Sum(nil)[0:4])
}

// GuessScenarioAndObjective attempts to match an experiment name back to the scenario
// and objective names used to generate it.
func GuessScenarioAndObjective(app *optimizeappsv1alpha1.Application, experimentName string) (scenario, objective string) {
	for i := range app.Scenarios {
		for j := range app.Objectives {
			if ExperimentName(app, app.Scenarios[i].Name, app.Objectives[j].Name) == experimentName {
				return app.Scenarios[i].Name, app.Objectives[j].Name
			}
		}
	}

	return "", ""
}

// WorkingDirectory returns the directory the application was loaded from. This
// directory should be used as the effective working directory when resolving relative
// paths found in the application definition.
func WorkingDirectory(app *optimizeappsv1alpha1.Application) string {
	if path := app.Annotations[kioutil.PathAnnotation]; path != "" {
		return filepath.Dir(path)
	}
	return ""
}

func cleanName(n string) string {
	n = strings.Map(func(r rune) rune {
		r = unicode.ToLower(r)
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '-' {
			return r
		}
		return -1
	}, n)

	if n == "" {
		n = "default"
	}

	return n
}
