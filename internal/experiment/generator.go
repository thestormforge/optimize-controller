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
	"fmt"
	"log"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/internal/application"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment/generation"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Generator is used to create an experiment definition.
type Generator struct {
	// The definition of the application to generate an experiment for.
	Application optimizeappsv1alpha1.Application
	// The name of the experiment to generate.
	ExperimentName string
	// The name of the scenario to generate an experiment for. Required if there are more then one scenario.
	Scenario string
	// The name of the objective to generate an experiment for. Required if there are more then one set of objectives.
	Objective string
	// IncludeApplicationResources is a flag indicating that the application resources should be included in the output.
	IncludeApplicationResources bool
	// Configure the filter options.
	scan.FilterOptions
}

// Execute the experiment generation pipeline, sending the results to the supplied writer.
func (g *Generator) Execute(output kio.Writer) error {
	pipeline, err := g.Pipeline()
	if err != nil {
		return err
	}

	pipeline.Filters = append([]kio.Filter{scan.NewKonjureFilter(application.WorkingDirectory(&g.Application), g.DefaultReader)}, pipeline.Filters...)

	pipeline.Outputs = append(pipeline.Outputs, output)

	return pipeline.Execute()
}

func (g *Generator) Pipeline() (kio.Pipeline, error) {
	scenario, err := application.GetScenario(&g.Application, g.Scenario)
	if err != nil {
		return kio.Pipeline{}, err
	}

	objective, err := application.GetObjective(&g.Application, g.Objective)
	if err != nil {
		return kio.Pipeline{}, err
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
			kio.FilterAll(yaml.SetLabel(optimizeappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetNamespace(g.Application.Namespace)),
			kio.FilterAll(generation.SetExperimentName(experimentName)),
			kio.FilterAll(generation.SetExperimentLabel(optimizeappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetExperimentLabel(optimizeappsv1alpha1.LabelScenario, scenarioName)),
			kio.FilterAll(generation.SetExperimentLabel(optimizeappsv1alpha1.LabelObjective, objectiveName)),

			// Apply Kubernetes formatting conventions and clean up the objects
			&filters.FormatFilter{UseSchema: true},
			kio.FilterAll(yaml.ClearAnnotation(filters.FmtAnnotation)),
			kio.FilterAll(yaml.Clear("status")),
		},
		Outputs: []kio.Writer{
			// Validate the resulting resources before sending them to the supplied writer
			kio.WriterFunc(g.validate),
		},
	}, nil
}

// selectors returns the selectors used to make discoveries during the scan.
func (g *Generator) selectors() []scan.Selector {
	var result []scan.Selector

	for i := range g.Application.Parameters {
		switch {

		case g.Application.Parameters[i].ContainerResources != nil:
			result = append(result, &generation.ContainerResourcesSelector{
				GenericSelector: scan.GenericSelector{
					LabelSelector: g.Application.Parameters[i].ContainerResources.Selector,
				},
				Resources:          g.Application.Parameters[i].ContainerResources.Resources,
				CreateIfNotPresent: true,
			})

		case g.Application.Parameters[i].Replicas != nil:
			result = append(result, &generation.ReplicaSelector{
				GenericSelector: scan.GenericSelector{
					LabelSelector: g.Application.Parameters[i].Replicas.Selector,
				},
				CreateIfNotPresent: true,
			})

		case g.Application.Parameters[i].EnvironmentVariable != nil:
			result = append(result, &generation.EnvironmentVariablesSelector{
				GenericSelector: scan.GenericSelector{
					LabelSelector: g.Application.Parameters[i].EnvironmentVariable.Selector,
				},
				VariableName: g.Application.Parameters[i].EnvironmentVariable.Name,
				ValuePrefix:  g.Application.Parameters[i].EnvironmentVariable.Prefix,
				ValueSuffix:  g.Application.Parameters[i].EnvironmentVariable.Suffix,
				Values:       g.Application.Parameters[i].EnvironmentVariable.Values,
			})
		}

	}

	// Make sure we have at least one selector that will produce parameters
	if len(result) == 0 {
		result = append(result, &generation.ContainerResourcesSelector{CreateIfNotPresent: true})
	}

	// Apply defaults to any selector that supports it
	for i := range result {
		if def, ok := result[i].(interface{ Default() }); ok {
			def.Default()
		}
	}

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

// This doesnt necessarily need to live here, but seemed to make sense
func Run(appCh chan *redskyappsv1alpha1.Application) {
	for app := range appCh {
		// TODO
		//
		// How do we want to scope the app.yaml here?
		//
		// Should we assume that since we're in control of the endpoint that is generating
		// the app.yaml, we should only ever have 1 scenario/objective?
		// Or should we support fetching app.yaml from any endpoint
		// at which point we need to select an appropriate scenario/objective and handle
		// all the same nuances from the redskyctl generate experiment -
		// name/namespace/objective/scenario/resources location
		//
		// Do annotations here make sense to provide that hint around which
		// objective/scenario to pick?

		if app.Namespace == "" || app.Name == "" {
			log.Println("bad app.yaml")
			continue
		}

		g := &Generator{
			Application: *app,
		}

		// TODO
		// (larger note above) how do we want to handle scenario/objective filtering?
		g.SetDefaultSelectors()

		var inputsBuf bytes.Buffer

		// TODO
		// Since we're using konjure for this, we need to bundle all of the bins konjure supports
		// inside the controller container
		// Or should we swap to client libraries?
		if err := g.Execute(kio.ByteWriter{Writer: &inputsBuf}); err != nil {
			log.Println(err)
			continue
		}

		// Do we want to look for annotations to trigger the start of the experiment?
		// ex, `stormforge.dev/application-verified: true` translates to `exp.spec.replicas = 1`

		// TODO
		// What next with the generated experiment?
		//
		// handle creation here?
		// // just experiment.yaml, no rbac?
		// maybe send up to some api for preview/confirmation?
		// maybe both?
		// maybe neither?
	}
}
