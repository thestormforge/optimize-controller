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

package sfio

import (
	"net/url"
	"strings"

	"github.com/thestormforge/konjure/pkg/filters"
	optimizev1alpha1 "github.com/thestormforge/optimize-controller/v2/api/v1alpha1"
	optimizev1beta1 "github.com/thestormforge/optimize-controller/v2/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// MetadataMigrationFilter is a KYAML filter for performing label/annotation migration.
type MetadataMigrationFilter struct {
}

// Filter replaces legacy prefixes found on labels and annotations.
func (f *MetadataMigrationFilter) Filter(node *yaml.RNode) (*yaml.RNode, error) {
	// NOTE: We are not migrating state that used at runtime, e.g. finalizers or the special "initializers" annotation.

	replaceFieldPrefix := yaml.FilterFunc(func(rn *yaml.RNode) (*yaml.RNode, error) {
		return nil, rn.VisitFields(func(node *yaml.MapNode) error {
			return node.Key.PipeE(
				&PrefixClearer{Value: "redskyops.dev/"},
				&yaml.PrefixSetter{Value: "stormforge.io/"},
			)
		})
	})

	return node.Pipe(
		yaml.Tee(yaml.Lookup(yaml.MetadataField, yaml.LabelsField), replaceFieldPrefix),
		yaml.Tee(yaml.Lookup(yaml.MetadataField, yaml.AnnotationsField), replaceFieldPrefix),
	)
}

// ExperimentMigrationFilter is a KYAML filter for performing experiment migration.
type ExperimentMigrationFilter struct {
}

// Filter applies migration changes to all recognized experiment nodes in the supplied list.
func (f *ExperimentMigrationFilter) Filter(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   optimizev1alpha1.GroupVersion.Group,
				Version: optimizev1alpha1.GroupVersion.Version,
				Kind:    "Experiment",
			}),
			yaml.FilterFunc(f.MigrateExperimentV1alpha1),
		),

		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   optimizev1beta1.GroupVersion.Group,
				Version: optimizev1beta1.GroupVersion.Version,
				Kind:    "Experiment",
			}),
			yaml.FilterFunc(f.MigrateExperimentV1beta1),
		),
	)
}

// MigrateExperimentV1beta1 converts a resource node from a v1beta1 Experiment to the latest format.
func (f *ExperimentMigrationFilter) MigrateExperimentV1beta1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Fix all the nested labels and annotations on the experiment
		yaml.Tee(
			yaml.Lookup("spec", "trialTemplate"), &MetadataMigrationFilter{},
			yaml.Lookup("spec", "jobTemplate"), &MetadataMigrationFilter{},
			yaml.Lookup("spec", "template"), &MetadataMigrationFilter{},
		),
	)
}

// MigrateExperimentV1alpha1 converts a resource node from a v1alpha1 Experiment to a v1beta1 Experiment.
func (f *ExperimentMigrationFilter) MigrateExperimentV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Remove the "# trial", "# job", "# pod" comments if present
		yaml.Tee(
			yaml.Lookup("spec"), ClearFieldComment("template", "# trial"),
			yaml.Lookup("spec"), ClearFieldComment("template", "# job"),
			yaml.Lookup("spec"), ClearFieldComment("template", "# pod"),
		),

		// Rename the template fields
		yaml.Tee(
			yaml.Lookup("spec"), RenameField("template", "trialTemplate"),
			yaml.Lookup("spec"), RenameField("template", "jobTemplate"),
		),

		// Migrate the parameters
		yaml.Tee(
			yaml.Lookup("spec", "parameters"), yaml.FilterFunc(f.migrateParametersV1alpha1),
		),

		// Migrate the metrics
		yaml.Tee(
			yaml.Lookup("spec", "metrics"), yaml.FilterFunc(f.migrateMetricsV1alpha1),
		),

		// Finally, set the apiVersion
		yaml.Tee(
			yaml.SetField("apiVersion", yaml.NewStringRNode(optimizev1beta1.GroupVersion.String())),
		),
	)
}

func (f *ExperimentMigrationFilter) migrateParametersV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	elements, err := node.Elements()
	if err != nil {
		return nil, err
	}

	for _, e := range elements {
		err := e.PipeE(
			// If values is non-empty, remove min/max
			yaml.Tee(
				Has(yaml.Lookup("values", "-")),
				yaml.Tee(yaml.Clear("min")),
				yaml.Tee(yaml.Clear("max")),
			),
		)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

func (f *ExperimentMigrationFilter) migrateMetricsV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	elements, err := node.Elements()
	if err != nil {
		return nil, err
	}

	for _, e := range elements {
		m := metric{}
		if err := e.YNode().Decode(&m); err != nil {
			return nil, err
		}

		err := e.PipeE(
			// Change metric type "local" to "kubernetes"
			yaml.Tee(
				yaml.MatchField("type", "local"),
				yaml.Set(yaml.NewStringRNode(string(optimizev1beta1.MetricKubernetes))),
			),

			// Change type "pods" to "" and add "target: { kind: PodList }"
			yaml.Tee(
				Has(yaml.MatchField("type", "pods")),
				yaml.Tee(yaml.Clear("type")),
				yaml.SetField("target", yaml.NewMapRNode(&map[string]string{
					"kind": "PodList",
				})),
			),

			// Set or clear the URL field
			yaml.Tee(
				m.setURLField("url"),
			),

			// Rename the selector to target (must happen AFTER setting the "url" field because we unmarshal it)
			yaml.Tee(
				RenameField("selector", "target"),
			),

			// Remove fields that no longer exist
			yaml.Tee(yaml.Clear("path")),
			yaml.Tee(yaml.Clear("port")),
			yaml.Tee(yaml.Clear("scheme")),
		)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

// metric represents the parts of a v1alpha1 metric required for producing a v1beta1 metric URL.
type metric struct {
	Type     string                `yaml:"type"`
	Scheme   string                `yaml:"scheme"`
	Selector *metav1.LabelSelector `yaml:"selector"`
	Port     IntOrString           `yaml:"port"`
	Path     string                `yaml:"path"`
}

// setURLField returns a filter that will set the named field with the metric URL.
func (m *metric) setURLField(name string) yaml.Filter {
	switch m.Type {
	case "prometheus":
		if m.Selector == nil || (len(m.Selector.MatchLabels) == 1 && m.Selector.MatchLabels["app"] == "prometheus") {
			return yaml.Clear(name)
		}
	case "datadog":
		u := url.URL{RawQuery: url.Values{"aggregator": []string{m.Scheme}}.Encode()}
		return yaml.SetField(name, yaml.NewStringRNode(u.String()))
	case "jsonpath":
		// Don't return empty
	default:
		return yaml.Clear(name)
	}

	u := url.URL{
		Scheme: m.Scheme,
		Host:   optimizev1alpha1.LegacyHostnamePlaceholder,
		Path:   m.Path,
	}

	if u.Scheme == "" {
		u.Scheme = "http"
	}

	if m.Port.IntValue() > 0 {
		u.Host += ":" + m.Port.String()
	}

	if p := strings.SplitN(m.Path, "?", 2); len(p) > 1 {
		u.Path = p[0]
		u.RawQuery = p[1]
	}

	return yaml.SetField(name, yaml.NewStringRNode(u.String()))
}

type IntOrString struct {
	intstr.IntOrString
}

func (is *IntOrString) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == yaml.NodeTagInt {
		is.Type = intstr.Int
		return value.Decode(&is.IntVal)
	}
	return value.Decode(&is.StrVal)
}
