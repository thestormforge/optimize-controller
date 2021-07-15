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

package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

func (r *Runner) scan(app applications.Application, scenario applications.Scenario) (*redskyappsv1alpha1.Application, error) {
	if err := validate(app, scenario); err != nil {
		return nil, err
	}

	// TODO this might(?) belong in internal/server

	// Construct a controller representation of an application from the api definition
	baseApp := &redskyappsv1alpha1.Application{}

	resources, err := r.scanResources(app)
	if err != nil {
		return nil, err
	}

	baseApp.Resources = resources

	params, err := r.scanParameters(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Parameters = params

	objectives, err := r.scanObjectives(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Objectives = objectives

	scenarios, stormforger, err := r.scanScenarios(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Scenarios = scenarios

	// TODO follow up with note in scanScenarios
	baseApp.StormForger = stormforger

	return baseApp, nil
}

func (r *Runner) generateApp(app redskyappsv1alpha1.Application) ([]byte, error) {
	g := &Generator{
		Application: app,
	}

	// Exposed for testing so we can pass through
	// fake kubectl output
	if r.kubectlExecFn != nil {
		g.FilterOptions = scan.FilterOptions{KubectlExecutor: r.kubectlExecFn}
	}

	var output bytes.Buffer
	if err := g.Execute(kio.ByteWriter{Writer: &output}); err != nil {
		return nil, fmt.Errorf("%s: %w", "failed to generate experiment", err)
	}

	return output.Bytes(), nil

}

func validate(app applications.Application, scenario applications.Scenario) error {
	if app.Name == "" {
		return fmt.Errorf("invalid application, missing name")
	}

	if len(app.Resources) == 0 {
		return fmt.Errorf("invalid application, no resources specified")
	}

	if len(scenario.Objective) == 0 {
		return fmt.Errorf("invalid scenario, no objectives specified")
	}

	if len(scenario.Configuration) == 0 {
		return fmt.Errorf("invalid scenario, no configuration specified")
	}

	return nil
}

func (r *Runner) scanResources(app applications.Application) (konjure.Resources, error) {
	var kResources konjure.Resources

	for _, resource := range app.Resources {
		rawResource, err := json.Marshal(resource)
		if err != nil {
			return nil, err
		}

		// r := appResource{}
		res := konjure.Resource{}
		if err := json.Unmarshal(rawResource, &res); err != nil {
			return nil, err
		}

		// Only support Kubernetes resources for now
		if res.Kubernetes != nil {
			if len(res.Kubernetes.Namespaces) == 0 {
				return nil, fmt.Errorf("invalid resources, no namespace specified")
			}
		}

		kResources = append(kResources, res)
	}

	return kResources, nil
}

func (r *Runner) scanParameters(scenario applications.Scenario) ([]redskyappsv1alpha1.Parameter, error) {
	// Parameters
	rawParams, err := json.Marshal(scenario.Configuration)
	if err != nil {
		return nil, err
	}

	params := []redskyappsv1alpha1.Parameter{}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	return params, nil
}

func (r *Runner) scanObjectives(scenario applications.Scenario) ([]redskyappsv1alpha1.Objective, error) {
	rawObjectives, err := json.Marshal(scenario.Objective)
	if err != nil {
		return nil, err
	}
	objectives := []redskyappsv1alpha1.Objective{}
	if err := json.Unmarshal(rawObjectives, &objectives); err != nil {
		return nil, err
	}

	return objectives, nil
}

func (r *Runner) scanScenarios(scenario applications.Scenario) ([]redskyappsv1alpha1.Scenario, *redskyappsv1alpha1.StormForger, error) {
	rawSF, err := json.Marshal(scenario.StormForgePerformance)
	if err != nil {
		return nil, nil, err
	}

	sf := redskyappsv1alpha1.StormForgerScenario{}
	if err := json.Unmarshal(rawSF, &sf); err != nil {
		return nil, nil, err
	}

	appScenario := []redskyappsv1alpha1.Scenario{
		{
			// TODO Should this be name or title?
			Name:        scenario.Name,
			StormForger: &sf,
		},
	}

	// TODO I feel like we're going to need to populate the StormForger struct but we dont have any information/context from
	// what the api provides
	return appScenario, &redskyappsv1alpha1.StormForger{}, nil
}
