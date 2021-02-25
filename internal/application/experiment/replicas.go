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
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ReplicaSelector identifies zero or more replica specifications.
// NOTE: This object is basically a combination of a Kustomize FieldSpec and a Selector.
type ReplicaSelector struct {
	// Type information of the resources to consider.
	resid.Gvk `json:",inline,omitempty"`
	// Namespace of the resources to consider.
	Namespace string `json:"namespace,omitempty"`
	// Name of the resources to consider.
	Name string `json:"name,omitempty"`
	// Annotation selector of resources to consider.
	AnnotationSelector string `json:"annotationSelector,omitempty"`
	// Label selector of resources to consider.
	LabelSelector string `json:"labelSelector,omitempty"`
	// Path to the replica field.
	Path string `json:"path,omitempty"`
	// Create container resource specifications even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
}

// fieldSpec returns this selector as a Kustomize FieldSpec.
func (rs *ReplicaSelector) fieldSpec() types.FieldSpec {
	return types.FieldSpec{
		Gvk:                rs.Gvk,
		Path:               rs.Path,
		CreateIfNotPresent: rs.CreateIfNotPresent,
	}
}

// selector resturns this selector as a Kustomize Selector.
func (rs *ReplicaSelector) selector() types.Selector {
	return types.Selector{
		Gvk:                rs.Gvk,
		Namespace:          rs.Namespace,
		Name:               rs.Name,
		AnnotationSelector: rs.AnnotationSelector,
		LabelSelector:      rs.LabelSelector,
	}
}

func (rs *ReplicaSelector) findParameters(node *yaml.RNode) ([]applicationResourceParameter, error) {
	var result []applicationResourceParameter

	path := rs.fieldSpec().PathSlice()
	err := node.PipeE(
		&yaml.PathGetter{Path: path, Create: yaml.ScalarNode},
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			if node.YNode().Value == "" && !rs.CreateIfNotPresent {
				return node, nil
			}

			result = append(result, &replicaParameter{
				anode: anode{
					fieldPath: node.FieldPath(),
					value:     node.YNode(),
				},
			})

			return node, nil
		}))
	if err != nil {
		return nil, err
	}

	return result, nil
}

type replicaParameter struct {
	anode
}

func (p *replicaParameter) patch(name resNameGen) (yaml.Filter, error) {
	value := yaml.NewScalarRNode("{{ .Values." + name(p.fieldPath, "replicas") + " }}")
	value.YNode().Tag = yaml.NodeTagInt
	return yaml.Tee(
		&yaml.PathGetter{Path: p.fieldPath, Create: yaml.ScalarNode},
		yaml.FieldSetter{Value: value, OverrideStyle: true},
	), nil
}

func (p *replicaParameter) parameters(name resNameGen) ([]redskyv1beta1.Parameter, error) {
	var v int
	if err := p.value.Decode(&v); err != nil {
		return nil, err
	}
	if v <= 0 {
		return nil, nil
	}

	baselineReplicas := intstr.FromInt(v)
	var minReplicas, maxReplicas int32 = 1, 5

	// Only adjust the max replica count if necessary
	if baselineReplicas.IntVal > maxReplicas {
		maxReplicas = baselineReplicas.IntVal
	}

	return []redskyv1beta1.Parameter{{
		Name:     name(p.fieldPath, "replicas"),
		Min:      minReplicas,
		Max:      maxReplicas,
		Baseline: &baselineReplicas,
	}}, nil
}
