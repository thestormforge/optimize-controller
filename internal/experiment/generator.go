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
	"fmt"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/application"
	"github.com/thestormforge/optimize-controller/internal/experiment/generation"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Generator is used to create an experiment definition.
type Generator struct {
	// The definition of the application to generate an experiment for.
	Application redskyappsv1alpha1.Application
	// The name of the experiment to generate.
	ExperimentName string
	// The name of the scenario to generate an experiment for. Required if there are more then one scenario.
	Scenario string
	// The name of the objective to generate an experiment for. Required if there are more then one set of objectives.
	Objective string
	// ContainerResourcesSelectors are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelectors []generation.ContainerResourcesSelector
	// ReplicaSelectors are the selectors for determining what application resources to scan for desired replica counts.
	ReplicaSelectors []generation.ReplicaSelector
	// IncludeApplicationResources is a flag indicating that the application resources should be included in the output.
	IncludeApplicationResources bool
	// Configure the filter options.
	scan.FilterOptions
}

// SetDefaultSelectors adds the default selectors to the generator, this requires that the application
// already be configured on the generator.
func (g *Generator) SetDefaultSelectors() {
	// NOTE: This method is completely arbitrary based on what we think the desired output might be.
	// This bridges the gap between the powerful selection logic that is implemented vs the simple
	// selection configuration that is actually exposed on the Application.

	// Add replica selectors
	for i := range g.Application.Parameters {
		switch {

		case g.Application.Parameters[i].ContainerResources != nil:
			for _, gs := range defaultSelectorKinds() {
				gs.LabelSelector = g.Application.Parameters[i].ContainerResources.Selector
				g.ContainerResourcesSelectors = append(g.ContainerResourcesSelectors, generation.ContainerResourcesSelector{
					GenericSelector:    gs,
					Path:               "/spec/template/spec/containers",
					CreateIfNotPresent: true,
					Resources:          g.Application.Parameters[i].ContainerResources.Resources,
				})
			}

		case g.Application.Parameters[i].Replicas != nil:
			for _, gs := range defaultSelectorKinds() {
				gs.LabelSelector = g.Application.Parameters[i].Replicas.Selector
				g.ReplicaSelectors = append(g.ReplicaSelectors, generation.ReplicaSelector{
					GenericSelector:    gs,
					Path:               "/spec/replicas",
					CreateIfNotPresent: true,
				})
			}
		}

	}

	// Do not allow container resource selectors to be empty
	if len(g.ContainerResourcesSelectors) == 0 {
		for _, gs := range defaultSelectorKinds() {
			g.ContainerResourcesSelectors = append(g.ContainerResourcesSelectors, generation.ContainerResourcesSelector{
				GenericSelector:    gs,
				Path:               "/spec/template/spec/containers",
				CreateIfNotPresent: true,
			})
		}
	}
}

func defaultSelectorKinds() []scan.GenericSelector {
	return []scan.GenericSelector{
		{Group: "apps|extensions", Kind: "Deployment"},
		{Group: "apps|extensions", Kind: "StatefulSet"},
	}
}

// appendSelectorIfNotExist
func appendSelectorIfNotExist(selectors []string, selector string) []string {
	for _, s := range selectors {
		// TODO We should be working off normalized label selectors
		if s == selector {
			return selectors
		}
	}

	return append(selectors, selector)
}

// Execute the experiment generation pipeline, sending the results to the supplied writer.
func (g *Generator) Execute(output kio.Writer) error {
	scenario, err := application.GetScenario(&g.Application, g.Scenario)
	if err != nil {
		return err
	}

	objective, err := application.GetObjective(&g.Application, g.Objective)
	if err != nil {
		return err
	}

	// Compute the effective scenario, objective, and experiment names
	scenarioName, objectiveName, experimentName := "", "", g.ExperimentName
	if scenario != nil {
		scenarioName = scenario.Name
	}
	if objective != nil {
		objectiveName = objective.Name
	}
	if experimentName == "" {
		experimentName = application.ExperimentName(&g.Application, scenarioName, objectiveName)
	}

	return kio.Pipeline{
		ContinueOnEmptyResult: true,
		Inputs: []kio.Reader{
			// Read the resource from the application
			g.Application.Resources,
		},
		Filters: []kio.Filter{
			// Expand resource references using Konjure
			g.FilterOptions.NewFilter(application.WorkingDirectory(&g.Application)),

			// Scan the resources and transform them into an experiment (and it's supporting resources)
			&scan.Scanner{
				Transformer: &generation.Transformer{
					IncludeApplicationResources: g.IncludeApplicationResources,
				},
				Selectors: append(g.selectors(),
					&generation.ApplicationSelector{
						Application: &g.Application,
						Scenario:    scenario,
						Objective:   objective,
					}),
			},

			// Label the generated resources
			kio.FilterAll(yaml.SetLabel(redskyappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetNamespace(g.Application.Namespace)),
			kio.FilterAll(generation.SetExperimentName(experimentName)),
			kio.FilterAll(generation.SetExperimentLabel(redskyappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetExperimentLabel(redskyappsv1alpha1.LabelScenario, scenarioName)),
			kio.FilterAll(generation.SetExperimentLabel(redskyappsv1alpha1.LabelObjective, objectiveName)),

			// Apply Kubernetes formatting conventions and clean up the objects
			&filters.FormatFilter{UseSchema: true},
			kio.FilterAll(yaml.ClearAnnotation(filters.FmtAnnotation)),
			kio.FilterAll(yaml.Clear("status")),
		},
		Outputs: []kio.Writer{
			// Validate the resulting resources before sending them to the supplier writer
			kio.WriterFunc(g.validate),
			output,
		},
	}.Execute()
}

// selectors returns the selectors used to make discoveries during the scan.
func (g *Generator) selectors() []scan.Selector {
	var result []scan.Selector

	// Replica selectors look for horizontally scalable resources
	for i := range g.ReplicaSelectors {
		result = append(result, &g.ReplicaSelectors[i])
	}

	// Container resource selectors look for resource requests/limits
	for i := range g.ContainerResourcesSelectors {
		result = append(result, &g.ContainerResourcesSelectors[i])
	}

	// TODO EnvVarSelector
	// TODO IngressSelector
	// TODO ConfigMapSelector?

	return result
}

// validate is basically just a hook to perform final verifications before actually emitting anything.
func (g *Generator) validate([]*yaml.RNode) error {

	objective, err := application.GetObjective(&g.Application, g.Objective)
	if err != nil {
		return err
	}

	if objective == nil {
		return nil
	}

	for i := range objective.Goals {
		if !objective.Goals[i].Implemented {
			return fmt.Errorf("generated experiment cannot optimize for goal %q", objective.Goals[i].Name)
		}
	}

	return nil
}
