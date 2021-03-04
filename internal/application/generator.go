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

package application

import (
	"io"

	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Generator struct {
	Name          string
	Resources     konjure.Resources
	Objectives    []string
	DefaultReader io.Reader
}

func (g *Generator) Execute(output kio.Writer) error {
	return kio.Pipeline{
		Inputs: []kio.Reader{g.Resources},
		Filters: []kio.Filter{
			&konjure.Filter{
				Depth:         100,
				DefaultReader: g.DefaultReader,
				KeepStatus:    true,
			},
			&scan.Scanner{
				Selectors:   []scan.Selector{g},
				Transformer: g,
			},
			kio.FilterAll(yaml.Clear("status")),
			&filters.FormatFilter{UseSchema: true},
		},
		Outputs:               []kio.Writer{output},
		ContinueOnEmptyResult: true,
	}.Execute()
}

// Select keeps all of the input resources.
func (g *Generator) Select(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	// TODO Should we limit this only to types we can actually get information from?
	return nodes, nil
}

func (g *Generator) Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error) {
	return nil, nil
}

func (g *Generator) Transform(_ []*yaml.RNode, selected []interface{}) ([]*yaml.RNode, error) {
	result := scan.ObjectSlice{}

	app := &redskyappsv1alpha1.Application{
		Resources: g.Resources,
	}

	for _, o := range g.Objectives {
		app.Objectives = append(app.Objectives, redskyappsv1alpha1.Objective{Name: o})
	}

	if g.Name != "" {
		app.Name = g.Name
	}

	result = append(result, app)
	return result.Read()
}
