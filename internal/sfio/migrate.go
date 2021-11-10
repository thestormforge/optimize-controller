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
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/thestormforge/konjure/pkg/filters"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			if node.IsNilOrEmpty() {
				return nil
			}
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
	ApplicationsAPI applications.API
}

// Filter applies migration changes to all recognized experiment nodes in the supplied list.
func (f *ExperimentMigrationFilter) Filter(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Application
		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   "apps.redskyops.dev",
				Version: "v1alpha1",
				Kind:    "Application",
			}),
			yaml.FilterFunc(f.MigrateRSOApplicationV1alpha1),
		),

		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   optimizeappsv1alpha1.GroupVersion.Group,
				Version: optimizeappsv1alpha1.GroupVersion.Version,
				Kind:    "Application",
			}),
			yaml.FilterFunc(f.MigrateApplicationV1alpha1),
		),

		// Experiment
		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group: "redskyops.dev",
				Kind:  "Experiment",
			}),
			yaml.FilterFunc(f.MayNeedConversionFromV1alpha1),
		),

		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   "redskyops.dev",
				Version: "v1alpha1",
				Kind:    "Experiment",
			}),
			yaml.FilterFunc(f.MigrateExperimentV1alpha1),
		),

		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   "redskyops.dev",
				Version: "v1beta1",
				Kind:    "Experiment",
			}),
			yaml.FilterFunc(f.MigrateExperimentV1beta1),
		),

		yaml.Tee(
			filters.FilterOne(&filters.ResourceMetaFilter{
				Group:   optimizev1beta2.GroupVersion.Group,
				Version: optimizev1beta2.GroupVersion.Version,
				Kind:    "Experiment",
			}),
			yaml.FilterFunc(f.MigrateExperimentV1beta2),
		),
	)
}

// MigrateApplicationV1alpha1 converts a resource node from a v1alpha1 Application to the latest format.
func (f *ExperimentMigrationFilter) MigrateApplicationV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe()
}

// MigrateRSOApplicationV1alpha1 converts a resource node from an apps.redskyops.dev/v1alpha1 Application to a v1alpha1 Application.
func (f *ExperimentMigrationFilter) MigrateRSOApplicationV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Update the API version to match the new group
		yaml.Tee(
			yaml.SetField("apiVersion", yaml.NewStringRNode(optimizeappsv1alpha1.GroupVersion.String())),
		),

		// Rename fields
		yaml.Tee(RenameField("parameters", "configuration")),
		yaml.Tee(
			yaml.Lookup("scenarios"),
			yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
				return node, node.VisitElements(func(node *yaml.RNode) error {
					return node.PipeE(RenameField("stormforger", "stormforgePerf"))
				})
			}),
		),

		// Remove fields
		yaml.Tee(yaml.Clear("stormForger")),
	)
}

// MigrateExperimentV1beta2 converts a resource node from a v1beta1 Experiment to the latest format.
func (f *ExperimentMigrationFilter) MigrateExperimentV1beta2(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Ensure the application and scenario labels are set correctly
		yaml.Tee(
			yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
				labelApplication, labelScenario, err := f.appLabels(node)
				if err != nil {
					return nil, err
				}
				return node.Pipe(
					yaml.Tee(yaml.SetLabel(optimizeappsv1alpha1.LabelApplication, labelApplication)),
					yaml.Tee(yaml.SetLabel(optimizeappsv1alpha1.LabelScenario, labelScenario)),
				)
			}),
		),
	)
}

// MigrateExperimentV1beta1 converts a resource node from a v1beta1 Experiment to a v1beta2 Experiment.
func (f *ExperimentMigrationFilter) MigrateExperimentV1beta1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		// Fix all the labels on the experiment itself
		yaml.Tee(
			&MetadataMigrationFilter{},
		),

		// Fix all the nested labels and annotations on the experiment
		yaml.Tee(
			yaml.Lookup("spec", "trialTemplate"), &MetadataMigrationFilter{},
			yaml.Lookup("spec", "jobTemplate"), &MetadataMigrationFilter{},
			yaml.Lookup("spec", "template"), &MetadataMigrationFilter{},
		),

		// Replace the prefix on any readiness gates
		TeeMatched(
			yaml.PathMatcher{Path: []string{"spec", "patches", "[patch=]", "readinessGates", "[conditionType=redskyops\\.dev/.*]", "conditionType"}},
			&PrefixClearer{Value: "redskyops.dev/"},
			&yaml.PrefixSetter{Value: "stormforge.io/"},
		),
		TeeMatched(
			yaml.PathMatcher{Path: []string{"spec", "trialTemplate", "spec", "readinessGates", "[kind=]", "conditionTypes", "[=redskyops\\.dev/.*]"}},
			&PrefixClearer{Value: "redskyops.dev/"},
			&yaml.PrefixSetter{Value: "stormforge.io/"},
		),

		// Finally, set the apiVersion
		yaml.Tee(
			yaml.SetField("apiVersion", yaml.NewStringRNode(optimizev1beta2.GroupVersion.String())),
		),
	)
}

// MayNeedConversionFromV1alpha1 mimics the legacy conversion behavior of checking for a "spec.template" field
// to determine if an Experiment should be treated as if the version were "v1alpha1".
func (f *ExperimentMigrationFilter) MayNeedConversionFromV1alpha1(node *yaml.RNode) (*yaml.RNode, error) {
	return node.Pipe(
		yaml.Tee(
			Has(yaml.Lookup("spec", "template")),
			yaml.SetField("apiVersion", yaml.NewStringRNode("redskyops.dev/v1alpha1")),
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
			yaml.SetField("apiVersion", yaml.NewStringRNode("redskyops.dev/v1beta1")),
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
				yaml.Set(yaml.NewStringRNode(string(optimizev1beta2.MetricKubernetes))),
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

func (f *ExperimentMigrationFilter) appLabels(node *yaml.RNode) (labelApplication string, labelScenario string, err error) {
	// Extract the metadata from the node
	var md yaml.ResourceMeta
	md, err = node.GetMeta()
	if err != nil {
		return
	}
	labelApplication = md.Labels[optimizeappsv1alpha1.LabelApplication]
	labelScenario = md.Labels[optimizeappsv1alpha1.LabelScenario]

	// Try to migrate the display names (ignore API errors)
	if f.ApplicationsAPI != nil && labelApplication != "" {
		labelApplication, labelScenario = f.resolveDisplayNames(labelApplication, labelScenario)
	}

	// Guess an application name using the experiment name
	if labelApplication == "" {
		labelApplication = "default"

		// Filter out buzzwords, and pure integers
		var appName []string
		drop := regexp.MustCompile("^(example|experiment|final|yolo|[0-9]+)$")
		for _, p := range strings.Split(strings.ToLower(md.Name), "-") {
			if p != "" && !drop.MatchString(p) {
				appName = append(appName, p)
			}
		}

		if len(appName) > 0 {
			// TODO We could look at `md.Annotations[kioutil.PathAnnotation]`
			labelApplication = strings.Join(appName, "-")
		}
	}

	// Guess the scenario name
	if labelScenario == "" {
		labelScenario = "default"

		// Try to get the first container name off the trial job
		nameNode, err := node.Pipe(yaml.Lookup("spec", "trialTemplate", "spec", "jobTemplate", "spec", "template", "spec", "containers", "0", "name"))
		if err == nil && !nameNode.IsNilOrEmpty() && nameNode.YNode().Value != "" {
			labelScenario = nameNode.YNode().Value
		}
	}

	return
}

// resolveDisplayNames returns the supplied values unless they match the display name
// of a recognized application and scenario. If the display name is ambiguous then
// no guarantee is made as to which corresponding name will be used.
func (f *ExperimentMigrationFilter) resolveDisplayNames(appTitle, scnTitle string) (string, string) {
	ctx := context.Background()
	l := applications.Lister{API: f.ApplicationsAPI}

	app, err := l.GetApplicationByNameOrTitle(ctx, appTitle)
	if err != nil {
		return appTitle, scnTitle
	}

	scn, err := l.GetScenarioByNameOrTitle(ctx, app, scnTitle)
	if err != nil {
		return app.Name.String(), scnTitle
	}

	return app.Name.String(), scn.Name
}

// metric represents the parts of a v1alpha1 metric required for producing a v1beta1 metric URL.
type metric struct {
	Name     string                `yaml:"name"`
	Type     string                `yaml:"type"`
	Scheme   string                `yaml:"scheme"`
	Selector *metav1.LabelSelector `yaml:"selector"`
}

// setURLField returns a filter that will set the named field with the metric URL.
func (m *metric) setURLField(name string) yaml.Filter {
	switch m.Type {
	case "datadog":
		// Migrate the Datadog URL. The "scheme" field was overloaded in v1alpha1 to
		// be the "aggregator" (e.g. min,max,avg); starting in v1beta1 we put that
		// information in the "aggregator" query parameter of the URL (the URL is
		// is otherwise unused for Datadog since the API library makes the determination
		// of which endpoint to talk to).
		u := url.URL{RawQuery: url.Values{"aggregator": []string{m.Scheme}}.Encode()}
		return yaml.SetField(name, yaml.NewStringRNode(u.String()))

	case "prometheus":
		// In v1alpha1 the Prometheus server was determined by matching a Service
		// using the metrics selector.
		if m.Selector == nil {
			// There is no selector meaning the intent was to get the default
			// behavior, to achieve that we need to clear the URL field.
			return yaml.Clear(name)
		}

		if len(m.Selector.MatchLabels) == 1 && m.Selector.MatchLabels["app"] == "prometheus" {
			// There is an explicit selector, however the intent is still to get
			// the default behavior. Now we need to clear both the selector and
			// the URL to get the desired behavior.
			return yaml.Tee(yaml.Clear("selector"), yaml.Clear(name))
		}

		// We cannot reliably migrate the Prometheus metric because we cannot
		// produce a valid URL, we don't know the name of the service that matches
		// the label selector. Force a failure.
		return yaml.FilterFunc(func(*yaml.RNode) (*yaml.RNode, error) {
			return nil, fmt.Errorf("the Prometheus metric %q cannot be migrated, "+
				"you must manually set the `url` field to the address of the Prometheus server", m.Name)
		})

	case "jsonpath":
		// We cannot reliably migrate JSON Path metric because we cannot produce
		// a valid URL: in v1alpha1, the host name was not available until runtime.
		// We need to fail and have the user manually intervene.
		return yaml.FilterFunc(func(*yaml.RNode) (*yaml.RNode, error) {
			return nil, fmt.Errorf("the JSON Path metric %q cannot be migrated, "+
				"you must manually set the `url` field to JSON endpoint", m.Name)
		})

	default:

		// The URL field should not be used in this case, make sure it is cleared if set.
		return yaml.Clear(name)
	}
}
