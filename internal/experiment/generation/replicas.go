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
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ReplicaSelector identifies zero or more replica specifications.
type ReplicaSelector struct {
	scan.GenericSelector
	// Path to the replica field.
	Path string `json:"path,omitempty"`
	// Create container resource specifications even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
}

var _ scan.Selector = &ReplicaSelector{}

func (s *ReplicaSelector) Default() {
	if s.Kind == "" {
		s.Group = "apps|extensions"
		s.Kind = "Deployment|StatefulSet"
	}
	if s.Path == "" {
		s.Path = "/spec/replicas"
	}
}

func (s *ReplicaSelector) Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error) {
	var result []interface{}

	path, err := sfio.FieldPath(s.Path, nil)
	if err != nil {
		return nil, err
	}

	return result, node.PipeE(
		&yaml.PathGetter{Path: path, Create: yaml.ScalarNode},
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			value := node.YNode()
			if value.Value == "" {
				if !s.CreateIfNotPresent {
					return node, nil
				}
				value = &yaml.Node{Kind: yaml.ScalarNode, Value: "1"}
			}

			result = append(result, &replicaParameter{pnode: pnode{
				meta:      meta,
				fieldPath: node.FieldPath(),
				value:     value,
			}})

			return node, nil
		}))
}

type replicaParameter struct {
	pnode
}

var _ PatchSource = &replicaParameter{}
var _ ParameterSource = &replicaParameter{}

func (p *replicaParameter) Patch(name ParameterNamer) (yaml.Filter, error) {
	value := yaml.NewScalarRNode("{{ .Values." + name(p.meta, p.fieldPath, "replicas") + " }}")
	value.YNode().Tag = yaml.NodeTagInt
	return yaml.Tee(
		&yaml.PathGetter{Path: p.fieldPath, Create: yaml.ScalarNode},
		yaml.FieldSetter{Value: value, OverrideStyle: true},
	), nil
}

func (p *replicaParameter) Parameters(name ParameterNamer) ([]optimizev1beta2.Parameter, error) {
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

	return []optimizev1beta2.Parameter{{
		Name:     name(p.meta, p.fieldPath, "replicas"),
		Min:      minReplicas,
		Max:      maxReplicas,
		Baseline: &baselineReplicas,
	}}, nil
}
