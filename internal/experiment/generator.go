/*
Copyright 2021 GramLabs, Inc.

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
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/application"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"github.com/yujunz/go-getter"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Generator struct {
	// The definition of the application to generate an experiment for.
	Application redskyappsv1alpha1.Application
	// ContainerResourcesSelectors are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelectors []ContainerResourcesSelector
	// ReplicaSelectors are the selectors for determining what application resources to scan for desired replica counts.
	ReplicaSelectors []ReplicaSelector
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
	g.ContainerResourcesSelectors = []ContainerResourcesSelector{
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
		g.ReplicaSelectors = []ReplicaSelector{
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
	// Hack the sort order used by the format filter to make experiments sort more naturally
	yaml.FieldOrder["parameters"] = 100
	yaml.FieldOrder["metrics"] = 200
	yaml.FieldOrder["targetRef"] = 100
	yaml.FieldOrder["patch"] = 200
	defer func() {
		delete(yaml.FieldOrder, "parameters")
		delete(yaml.FieldOrder, "metrics")
		delete(yaml.FieldOrder, "targetRef")
		delete(yaml.FieldOrder, "patch")
	}()

	// Execute the pipeline
	return kio.Pipeline{
		Inputs: []kio.Reader{
			g.Application.Resources,
		},
		Filters: []kio.Filter{
			&konjure.Filter{Depth: 100, DefaultReader: g.DefaultReader, KeepStatus: true},
			g.newScannerFilter(),
			&ApplicationFilter{Application: &g.Application},
			kio.FilterAll(yaml.Clear("status")),
			&filters.FormatFilter{UseSchema: true},
		},
		Outputs: []kio.Writer{
			output,
		},
	}.Execute()
}

func (g *Generator) newScannerFilter() kio.Filter {
	scanner := &scan.Scanner{
		Transformer: &Transformer{
			DefaultExperimentName:       application.ExperimentName(&g.Application),
			MergeGenerated:              len(g.Application.Scenarios) > 1,
			IncludeApplicationResources: g.IncludeApplicationResources,
		},
	}

	for i := range g.ContainerResourcesSelectors {
		scanner.Selectors = append(scanner.Selectors, &g.ContainerResourcesSelectors[i])
	}

	for i := range g.ReplicaSelectors {
		scanner.Selectors = append(scanner.Selectors, &g.ReplicaSelectors[i])
	}

	// TODO EnvVarSelector
	// TODO IngressSelector
	// TODO ConfigMapSelector?

	// The application selector should run last so it can fill in anything that is missing
	scanner.Selectors = append(scanner.Selectors, &ApplicationSelector{Application: &g.Application})

	return scanner
}
