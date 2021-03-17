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
	"io"

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
	// ContainerResourcesSelectors are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelectors []generation.ContainerResourcesSelector
	// ReplicaSelectors are the selectors for determining what application resources to scan for desired replica counts.
	ReplicaSelectors []generation.ReplicaSelector
	// IncludeApplicationResources is a flag indicating that the application resources should be included in the output.
	IncludeApplicationResources bool
	// Default reader to use instead of stdin.
	DefaultReader io.Reader
}

// SetDefaultSelectors adds the default selectors to the generator, this requires that the application
// already be configured on the generator.
func (g *Generator) SetDefaultSelectors() {
	// NOTE: This method is completely arbitrary based on what we think the desired output might be.
	// This bridges the gap between the powerful selection logic that is implemented vs the simple
	// selection configuration that is actually exposed on the Application.

	// Always add container resource selectors, conditionally with an explicit label selector
	var crsLabelSelector string
	if g.Application.Parameters != nil && g.Application.Parameters.ContainerResources != nil {
		crsLabelSelector = g.Application.Parameters.ContainerResources.LabelSelector
	}
	g.ContainerResourcesSelectors = []generation.ContainerResourcesSelector{
		{
			GenericSelector: scan.GenericSelector{
				Group:         "apps|extensions",
				Kind:          "Deployment",
				LabelSelector: crsLabelSelector,
			},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
		{
			GenericSelector: scan.GenericSelector{
				Group:         "apps|extensions",
				Kind:          "StatefulSet",
				LabelSelector: crsLabelSelector,
			},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
	}

	// Only add replica selectors if the parameter is explicitly configured
	if g.Application.Parameters != nil && g.Application.Parameters.Replicas != nil {
		g.ReplicaSelectors = []generation.ReplicaSelector{
			{
				GenericSelector: scan.GenericSelector{
					Group:         "apps|extensions",
					Kind:          "Deployment",
					LabelSelector: g.Application.Parameters.Replicas.LabelSelector,
				},
				Path:               "/spec/replicas",
				CreateIfNotPresent: true,
			},
			{
				GenericSelector: scan.GenericSelector{
					Group:         "apps|extensions",
					Kind:          "StatefulSet",
					LabelSelector: g.Application.Parameters.Replicas.LabelSelector,
				},
				Path:               "/spec/replicas",
				CreateIfNotPresent: true,
			},
		}
	}
}

// Execute the experiment generation pipeline, sending the results to the supplied writer.
func (g *Generator) Execute(output kio.Writer) error {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			// Read the resource from the application
			g.Application.Resources,
		},
		Filters: []kio.Filter{
			// Expand resource references using Konjure
			scan.NewKonjureFilter(application.WorkingDirectory(&g.Application), g.DefaultReader),

			// Scan the resources and transform them into an experiment (and it's supporting resources)
			&scan.Scanner{
				Transformer: &generation.Transformer{
					IncludeApplicationResources: g.IncludeApplicationResources,
					MergeGenerated:              len(g.Application.Scenarios) > 1,
				},
				Selectors: g.selectors(),
			},

			// Label the generated resources
			kio.FilterAll(yaml.SetLabel(redskyappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetNamespace(g.Application.Namespace)),
			kio.FilterAll(generation.SetExperimentName(g.experimentName())),
			kio.FilterAll(generation.SetExperimentLabel(redskyappsv1alpha1.LabelApplication, g.Application.Name)),
			kio.FilterAll(generation.SetExperimentLabel(redskyappsv1alpha1.LabelScenario, g.scenarioName())),

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

// experimentName returns the effective experiment name being generated.
func (g *Generator) experimentName() string {
	return application.ExperimentName(&g.Application)
}

// scenarioName returns the scenario name iff there is only one scenario present.
func (g *Generator) scenarioName() string {
	if len(g.Application.Scenarios) == 1 {
		return g.Application.Scenarios[0].Name
	}
	return ""
}

// selectors returns the selectors used to make discoveries during the scan.
func (g *Generator) selectors() []scan.Selector {
	var result []scan.Selector

	// Container resource selectors look for resource requests/limits
	for i := range g.ContainerResourcesSelectors {
		result = append(result, &g.ContainerResourcesSelectors[i])
	}

	// Replica selectors look for horizontally scalable resources
	for i := range g.ReplicaSelectors {
		result = append(result, &g.ReplicaSelectors[i])
	}

	// TODO EnvVarSelector
	// TODO IngressSelector
	// TODO ConfigMapSelector?

	// The application selector runs last and looks at the application configuration itself
	result = append(result, &generation.ApplicationSelector{
		Application: &g.Application,
	})

	return result
}

// validate is basically just a hook to perform final verifications before actually emitting anything.
func (g *Generator) validate([]*yaml.RNode) error {

	for i := range g.Application.Objectives {
		if !g.Application.Objectives[i].Implemented {
			return fmt.Errorf("generated experiment cannot optimize objective: %s", g.Application.Objectives[i].Name)
		}
	}

	return nil
}
