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
	"path/filepath"
	"time"

	"github.com/thestormforge/konjure/pkg/konjure"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Generator is use to generate application definitions.
type Generator struct {
	// The name of the application to generate.
	Name string
	// The collection of resources defining the application.
	Resources konjure.Resources
	// File name containing a description of the load to generate.
	ScenarioFile string
	// The list of goal names to include in the application
	Goals []string
	// The filter to provide additional documentation in the generated YAML.
	Documentation DocumentationFilter
	// An explicit working directory used to relativize file paths.
	WorkingDirectory string
	// Configure the filter options.
	scan.FilterOptions
}

func (g *Generator) Execute(output kio.Writer) error {
	return kio.Pipeline{
		Inputs: []kio.Reader{g.Resources},
		Filters: []kio.Filter{
			g.FilterOptions.NewFilter(g.WorkingDirectory),
			&scan.Scanner{
				Selectors:   []scan.Selector{g},
				Transformer: g,
			},
			kio.FilterAll(yaml.Clear("status")),
			&filters.FormatFilter{UseSchema: true},
			&g.Documentation, // Documentation is added last so everything is sorted
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

// Map scans for useful information to include in the application definition
func (g *Generator) Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error) {
	var result []interface{}

	// If the resource stream already contains an application, we will use that
	// as a starting point for the rest of the generation.
	if meta.Kind == "Application" && meta.APIVersion == optimizeappsv1alpha1.GroupVersion.String() {
		data, err := node.MarshalJSON()
		if err != nil {
			return nil, err
		}

		app := &optimizeappsv1alpha1.Application{}
		if err := json.Unmarshal(data, app); err != nil {
			return nil, err
		}

		result = append(result, app)
	}

	return result, nil
}

// Transform converts the scan information into an application definition.
func (g *Generator) Transform(_ []*yaml.RNode, selected []interface{}) ([]*yaml.RNode, error) {
	result := sfio.ObjectSlice{}

	app := &optimizeappsv1alpha1.Application{}
	for _, sel := range selected {
		switch s := sel.(type) {

		case *optimizeappsv1alpha1.Application:
			g.merge(s, app)

		}
	}

	if err := g.apply(app); err != nil {
		return nil, err
	}

	if err := g.clean(app); err != nil {
		return nil, err
	}

	result = append(result, app)
	return result.Read()
}

// merge a source application into another application.
func (g *Generator) merge(src, dst *optimizeappsv1alpha1.Application) {
	if src.Name != "" {
		dst.Name = src.Name
	}

	if src.Namespace != "" {
		dst.Namespace = src.Namespace
	}

	if len(dst.Labels) > 0 {
		for k, v := range src.Labels {
			dst.Labels[k] = v
		}
	} else {
		dst.Labels = src.Labels
	}

	if len(dst.Annotations) > 0 {
		for k, v := range src.Annotations {
			dst.Annotations[k] = v
		}
	} else {
		dst.Annotations = src.Annotations
	}

	dst.Resources = append(dst.Resources, src.Resources...)
	dst.Scenarios = append(dst.Scenarios, src.Scenarios...)
	dst.Objectives = append(dst.Objectives, src.Objectives...)
}

// apply adds the generator configuration to the supplied application
func (g *Generator) apply(app *optimizeappsv1alpha1.Application) error {
	now := time.Now().UTC().Format(time.RFC3339)
	metav1.SetMetaDataAnnotation(&app.ObjectMeta, optimizeappsv1alpha1.AnnotationLastScanned, now)

	if g.Name != "" {
		app.Name = g.Name
	}

	app.Resources = append(app.Resources, g.Resources...)

	if s, err := g.readScenario(); err != nil {
		return err
	} else if s != nil {
		app.Scenarios = append(app.Scenarios, *s)
	}

	if o, err := g.readObjective(); err != nil {
		return err
	} else if o != nil {
		app.Objectives = append(app.Objectives, *o)
	}

	return nil
}

// clean ensures that the application state is reasonable.
func (g *Generator) clean(app *optimizeappsv1alpha1.Application) error {
	var resources []konjure.Resource
	for _, r := range app.Resources {
		switch {
		case r.Resource != nil:
			var resourceSpecs []string
			for _, rr := range r.Resource.Resources {
				// If the resource is specific to the current process, do not
				// include it in the output
				if rr == "-" || filepath.Dir(rr) == "/dev/fd" {
					continue
				}

				resourceSpecs = append(resourceSpecs, rr)
			}
			if len(resourceSpecs) > 0 {
				r.Resource.Resources = resourceSpecs
				resources = append(resources, r)
			}

		default:
			resources = append(resources, r)
		}
	}
	app.Resources = resources

	return nil
}

// readScenario attempts to create a scenario for the application.
func (g *Generator) readScenario() (*optimizeappsv1alpha1.Scenario, error) {
	// If there is no scenario file, do nothing
	if g.ScenarioFile == "" {
		return nil, nil
	}

	// This is a really basic assumption given we only support two scenario flavors right now
	switch filepath.Ext(g.ScenarioFile) {
	case ".js":
		return &optimizeappsv1alpha1.Scenario{
			StormForger: &optimizeappsv1alpha1.StormForgerScenario{
				TestCaseFile: g.ScenarioFile,
			},
		}, nil

	case ".py":
		return &optimizeappsv1alpha1.Scenario{
			Locust: &optimizeappsv1alpha1.LocustScenario{
				Locustfile: g.ScenarioFile,
			},
		}, nil
	}

	return nil, nil
}

// readObjective attempts to create an objective for the application.
func (g *Generator) readObjective() (*optimizeappsv1alpha1.Objective, error) {
	if len(g.Goals) == 0 {
		return nil, nil
	}

	obj := &optimizeappsv1alpha1.Objective{}
	for _, goal := range g.Goals {
		if goal != "" {
			obj.Goals = append(obj.Goals, optimizeappsv1alpha1.Goal{Name: goal})
		}
	}

	return obj, nil
}
