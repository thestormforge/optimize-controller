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

package generation

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ParameterNamer is used to compute the name of an optimization parameter.
type ParameterNamer func(meta yaml.ResourceMeta, path []string, name string) string

// ExperimentSource allows selectors to modify the experiment directly. Note that
// for adding parameters, patches, or metrics, the appropriate source should be
// used instead.
type ExperimentSource interface {
	Update(exp *optimizev1beta2.Experiment) error
}

// ParameterSource allows selectors to add parameters to an experiment. In
// general PatchSources should also be ParameterSources to ensure the parameters
// used in the generated patches are configured on the experiment.
type ParameterSource interface {
	Parameters(name ParameterNamer) ([]optimizev1beta2.Parameter, error)
}

// PatchSource allows selectors to contribute changes to the patch of a
// particular resource. In general, ParameterSource should also be implemented
// to add any parameters referenced by the generated patches.
type PatchSource interface {
	TargetRef() *corev1.ObjectReference
	Patch(name ParameterNamer) (yaml.Filter, error)
}

// MetricSource allows selectors to contribute metrics to an experiment.
type MetricSource interface {
	Metrics() ([]optimizev1beta2.Metric, error)
}

// Transformer is used to convert all of the output from the selectors, only selector output
// matching the "*Source" interfaces are supported.
type Transformer struct {
	// Flag indicating the all of the resources that were scanned should also be included in the output.
	IncludeApplicationResources bool
}

var _ scan.Transformer = &Transformer{}

// Transform converts a scan of the supplied nodes into an experiment definition.
func (t *Transformer) Transform(nodes []*yaml.RNode, selected []interface{}) ([]*yaml.RNode, error) {
	var result []*yaml.RNode

	// Parameter names need to be computed based on what resources were selected by the scan
	name := parameterNamer(selected)

	// Start with a new experiment and collect the scan results into it
	exp := optimizev1beta2.Experiment{}
	patches := make(map[corev1.ObjectReference][]yaml.Filter)
	for _, sel := range selected {
		// DO NOT use a type switch, there may be multiple implementations

		if e, ok := sel.(ExperimentSource); ok {
			if err := e.Update(&exp); err != nil {
				return nil, err
			}
		}

		if ps, ok := sel.(ParameterSource); ok {
			params, err := ps.Parameters(name)
			if err != nil {
				return nil, err
			}
			exp.Spec.Parameters = append(exp.Spec.Parameters, params...)
		}

		if ps, ok := sel.(PatchSource); ok {
			ref := ps.TargetRef()
			f, err := ps.Patch(name)
			if err != nil {
				return nil, err
			}
			patches[*ref] = append(patches[*ref], f)
		}

		if ms, ok := sel.(MetricSource); ok {
			metrics, err := ms.Metrics()
			if err != nil {
				return nil, err
			}
			exp.Spec.Metrics = append(exp.Spec.Metrics, metrics...)
		}

		// Also allow direct additions to the resource stream
		if r, ok := sel.(kio.Reader); ok {
			nodes, err := r.Read()
			if err != nil {
				return nil, err
			}
			result = append(result, nodes...)
		}
	}

	// Render patches into the experiment
	if err := t.renderPatches(patches, &exp); err != nil {
		return nil, err
	}

	// Perform some simple validation
	if err := t.checkExperiment(&exp, nodes); err != nil {
		return nil, err
	}

	// Serialize the experiment as a YAML node
	if expNode, err := (sfio.ObjectSlice{&exp}).Read(); err != nil {
		return nil, err
	} else {
		result = append(expNode, result...) // Put the experiment at the front
	}

	// If requested, append the actual application resources to the output
	if t.IncludeApplicationResources {
		appResources, err := kio.FilterAll(yaml.SetAnnotation(filters.FmtAnnotation, filters.FmtStrategyNone)).Filter(nodes)
		if err != nil {
			return nil, err
		}
		result = append(appResources, result...)
	}

	return result, nil
}

// renderPatches converts accumulated patch contributes (in the form of yaml.Filter instances) into
// actual patch templates on an experiment.
func (t *Transformer) renderPatches(patches map[corev1.ObjectReference][]yaml.Filter, exp *optimizev1beta2.Experiment) error {
	for ref, fs := range patches {
		// Start with an empty node
		patch := yaml.NewRNode(&yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		})

		// Add of the patch contributes by executing the filters
		if err := patch.PipeE(fs...); err != nil {
			return err
		}

		// Render the result as YAML
		var buf bytes.Buffer
		if err := yaml.NewEncoder(&buf).Encode(patch.Document()); err != nil {
			return err
		}

		// Since the patch template doesn't need to be valid YAML we can cleanup tagged integers
		data := regexp.MustCompile(`!!int '(.*)'`).ReplaceAll(buf.Bytes(), []byte("$1"))

		// Add the actual patch to the experiment
		exp.Spec.Patches = append(exp.Spec.Patches, optimizev1beta2.PatchTemplate{
			Patch:     string(data),
			TargetRef: ref.DeepCopy(),
		})
	}

	return nil
}

func (t *Transformer) checkExperiment(exp *optimizev1beta2.Experiment, nodes []*yaml.RNode) error {
	// If there are no parameters or metrics, the experiment isn't valid
	if len(exp.Spec.Parameters) == 0 {
		// Only report this error if we were going to fail anyway
		if len(nodes) == 0 {
			return fmt.Errorf("the application did not match any resources")
		}

		// If we didn't have any parameters, it was probably because we didn't
		// have any of the right kind of resource to scan (it could have been
		// bad selectors or objects just don't exist).
		return fmt.Errorf("invalid experiment, no parameters found while scanning %d resources", len(nodes))
	}
	if len(exp.Spec.Metrics) == 0 {
		return fmt.Errorf("invalid experiment, no metrics found")
	}

	// Make sure either all baselines are set or none are
	for i := range exp.Spec.Parameters {
		if exp.Spec.Parameters[i].Baseline == nil {
			for j := range exp.Spec.Parameters {
				exp.Spec.Parameters[j].Baseline = nil
			}
			break
		}
	}

	return nil
}

// pnode is the location and current state of something to parameterize in an application resource.
type pnode struct {
	meta      yaml.ResourceMeta // TODO Do we need the labels? Can this just be ResourceIdentifier?
	fieldPath []string
	value     *yaml.Node
}

// TargetRef returns the reference to the resource this parameter node belongs to.
func (p *pnode) TargetRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: p.meta.APIVersion,
		Kind:       p.meta.Kind,
		Name:       p.meta.Name,
		Namespace:  p.meta.Namespace,
	}
}

// parameterNamer returns a name generation function for parameters based on scan results.
func parameterNamer(selected []interface{}) ParameterNamer {
	// Index the object references by kind and name
	type targeted interface {
		TargetRef() *corev1.ObjectReference
	}
	needsPath := make(map[string]map[string]int)
	for _, sel := range selected {
		t, ok := sel.(targeted)
		if !ok {
			continue
		}
		targetRef := t.TargetRef()
		if ns := needsPath[targetRef.Kind]; ns == nil {
			needsPath[targetRef.Kind] = make(map[string]int)
		}
		needsPath[targetRef.Kind][targetRef.Name]++
	}

	// Determine which prefixes we need
	needsKind := len(needsPath) > 1
	needsName := false
	for _, v := range needsPath {
		needsName = needsName || len(v) > 1
	}

	return func(meta yaml.ResourceMeta, path []string, name string) string {
		var parts []string

		if needsKind {
			parts = append(parts, meta.Kind)
		}

		if needsName {
			parts = append(parts, meta.Name)
		}

		if needsPath[meta.Kind][meta.Name] > 1 {
			for _, p := range path {
				if yaml.IsListIndex(p) {
					if _, value, _ := yaml.SplitIndexNameValue(p); value != "" {
						parts = append(parts, value)
					}
				}
			}
		}

		if name != "" {
			parts = append(parts, name)
		}

		// Explainer: Parameter names are used in Go Templates which are executed
		// against Go structs, if the template parser encounters a token that is
		// not a valid Go field name, parsing fails (e.g. "bad character U+002D '-'").

		parameterName := strings.Join(parts, "_")
		parameterName = strings.ReplaceAll(parameterName, "-", "_")
		parameterName = strings.ToLower(parameterName)
		return parameterName
	}
}
