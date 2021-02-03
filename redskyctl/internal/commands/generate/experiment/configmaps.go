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
	"strconv"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ConfigMapKeySelector identifies a ConfigMap data entry.
type ConfigMapKeySelector struct {
	// Namespace of the resources to consider.
	Namespace string `json:"namespace,omitempty"`
	// Name of the resources to consider.
	Name string `json:"name,omitempty"`
	// Annotation selector of resources to consider.
	AnnotationSelector string `json:"annotationSelector,omitempty"`
	// Label selector of resources to consider.
	LabelSelector string `json:"labelSelector,omitempty"`
	// Data key in the ConfigMap to update.
	Key string `json:"key,omitempty"`
	// Create the config map entry even if the original object does not contain it.
	CreateIfNotPresent bool `json:"create,omitempty"`

	// Specification of a numeric value to optimize. Mutually exclusive with the string specification.
	NumericValue *redskyappsv1alpha1.NumericParameter `json:"numericValue,omitempty"`
	// Specification of possible string values. Mutually exclusive with the numeric specification.
	StringValue redskyappsv1alpha1.StringParameter `json:"stringValue,omitempty"`
}

// selector returns this selector as a Kustomize Selector.
func (rs *ConfigMapKeySelector) selector() types.Selector {
	return types.Selector{
		Gvk:                resid.Gvk{Version: "v1", Kind: "ConfigMap"},
		Namespace:          rs.Namespace,
		Name:               rs.Name,
		AnnotationSelector: rs.AnnotationSelector,
		LabelSelector:      rs.LabelSelector,
	}
}

// configMapValue represents details about the value of entry in a ConfigMap.
type configMapValue struct {
	Prefix string
	Suffix string
	Min    int32
	Max    int32
	Values []string

	Value string
	Tag   string
	Style yaml.Style
}

// cleanValue returns the value without the configured prefix and suffix.
func (v *configMapValue) cleanValue() string {
	vv := v.Value
	vv = strings.TrimPrefix(vv, v.Prefix)
	vv = strings.TrimSuffix(vv, v.Suffix)
	return vv
}

// hasValue checks to see if the current value is in the list of values.
func (v *configMapValue) hasValue(s string) bool {
	for _, vv := range v.Values {
		if vv == s {
			return true
		}
	}
	return false
}

// scanForConfigMapKeys scans the supplied resource map for config map keys matching the selector.
func (g *Generator) scanForConfigMapKeys(ars []*applicationResource, rm resmap.ResMap) ([]*applicationResource, error) {
	for _, sel := range g.ConfigMapKeySelectors {
		// Select the matching resources
		resources, err := rm.Select(sel.selector())
		if err != nil {
			return nil, err
		}

		for _, r := range resources {
			// Get the YAML tree representation of the resource
			node, err := filtersutil.GetRNode(r)
			if err != nil {
				return nil, err
			}

			// Scan the document tree for information to add to the application resource
			ar := &applicationResource{}
			if err := ar.saveTargetReference(node); err != nil {
				return nil, err
			}
			if err := ar.saveConfigMapKeys(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return nil, err
			}
			if len(ar.configMapPaths) == 0 {
				continue
			}

			// Make sure we only get the newly discovered parts
			ars = mergeOrAppend(ars, ar)
		}
	}

	return ars, nil
}

// saveConfigMapKeys extracts the paths to the data values from the supplied node.
func (r *applicationResource) saveConfigMapKeys(node *yaml.RNode, sel ConfigMapKeySelector) error {
	// NOTE: We do not consider the `binaryData` field of the ConfigMap
	path := []string{"data"}
	return node.PipeE(
		yaml.Lookup(path...),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			if mn := node.Field(sel.Key); mn != nil || sel.CreateIfNotPresent {
				cmv := configMapValue{}

				if mn != nil {
					cmv.Value = mn.Value.YNode().Value
					cmv.Tag = mn.Value.YNode().Tag
					cmv.Style = mn.Value.YNode().Style
				}

				// We need to verify that value we discovered in fact matches the specification requested. In theory
				// non-matching keys could just be silently ignored, but that may lead to unexpected behavior in
				// terms of missing parameters on the generated experiment: it is safer just to fail outright.

				if sel.NumericValue != nil {
					cmv.Prefix = sel.NumericValue.Prefix
					cmv.Suffix = sel.NumericValue.Suffix
					cmv.Min = sel.NumericValue.Min
					cmv.Max = sel.NumericValue.Max

					if v := cmv.cleanValue(); v != "" {
						if d, err := strconv.ParseInt(v, 10, 32); err != nil {
							return nil, fmt.Errorf("expected %q to be a numeric value", v)
						} else if int32(d) < cmv.Min || int32(d) > cmv.Max {
							return nil, fmt.Errorf("expected %q to be a numeric value in the range %d to %d", v, cmv.Min, cmv.Max)
						}
					}
				} else {
					cmv.Values = sel.StringValue

					if v := cmv.cleanValue(); v != "" && !cmv.hasValue(v) {
						return nil, fmt.Errorf("expected %q to be one of: %s", v, strings.Join(cmv.Values, ", "))
					}
				}

				r.configMapPaths = append(r.configMapPaths, append(path, sel.Key))
				r.configMapValues = append(r.configMapValues, cmv)
			}
			return nil, nil
		}))
}

// configMapParameters returns the parameters required for optimizing the discovered resources sections.
func (r *applicationResource) configMapParameters(name nameGen) []redskyv1beta1.Parameter {
	parameters := make([]redskyv1beta1.Parameter, 0, len(r.configMapPaths))
	for i := range r.configMapPaths {
		var baseline *intstr.IntOrString
		if v := r.configMapValues[i].cleanValue(); v != "" {
			baseline = new(intstr.IntOrString)
			*baseline = intstr.Parse(v)
		}

		parameters = append(parameters, redskyv1beta1.Parameter{
			Name:     name(&r.targetRef, nil, r.configMapPaths[i][1]),
			Baseline: baseline,
			Min:      r.configMapValues[i].Min,
			Max:      r.configMapValues[i].Max,
			Values:   r.configMapValues[i].Values,
		})
	}
	return parameters
}
