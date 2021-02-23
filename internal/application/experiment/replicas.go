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
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
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

// DefaultReplicaSelectors returns the default replica selectors. These selectors match
// the default role created by the `grant_permissions` code.
func DefaultReplicaSelectors() []ReplicaSelector {
	return []ReplicaSelector{
		{
			Gvk:                resid.Gvk{Group: "apps", Kind: "Deployment"},
			Path:               "/spec/replicas",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "extensions", Kind: "Deployment"},
			Path:               "/spec/replicas",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "apps", Kind: "StatefulSet"},
			Path:               "/spec/replicas",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "extensions", Kind: "StatefulSet"},
			Path:               "/spec/replicas",
			CreateIfNotPresent: true,
		},
	}
}

func (g *Generator) scanForReplicas(ars []*applicationResource, rm resmap.ResMap) ([]*applicationResource, error) {
	for _, sel := range g.ReplicaSelectors {
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
			if err := ar.saveReplicaPaths(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return nil, err
			}
			if len(ar.replicaPaths) == 0 {
				continue
			}

			// Make sure we only get the newly discovered parts
			ars = mergeOrAppend(ars, ar)
		}
	}

	return ars, nil
}

// saveReplicaPaths extracts the paths to the replicas fields from the supplied node.
func (r *applicationResource) saveReplicaPaths(node *yaml.RNode, sel ReplicaSelector) error {
	path := sel.fieldSpec().PathSlice()
	return node.PipeE(
		&yaml.PathGetter{Path: path, Create: yaml.ScalarNode},
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			if node.YNode().Value == "" && !sel.CreateIfNotPresent {
				return node, nil
			}

			var replicas int32 = 1
			_ = node.YNode().Decode(&replicas)

			r.replicaPaths = append(r.replicaPaths, node.FieldPath())
			r.replicas = append(r.replicas, replicas)
			return node, nil
		}))
}

// replicasParameters returns the parameters required for optimizing the discovered replicas.
func (r *applicationResource) replicasParameters(name nameGen) []redskyv1beta1.Parameter {
	parameters := make([]redskyv1beta1.Parameter, 0, len(r.replicaPaths))
	for i := range r.replicaPaths {
		baselineReplicas := intstr.FromInt(int(r.replicas[i]))
		var minReplicas, maxReplicas int32 = 1, 5

		// Do not explicitly enable something that was disabled
		if baselineReplicas.IntVal <= 0 {
			continue
		}

		// Only adjust the max replica count if necessary
		if baselineReplicas.IntVal > maxReplicas {
			maxReplicas = baselineReplicas.IntVal
		}

		parameters = append(parameters, redskyv1beta1.Parameter{
			Name:     name(&r.targetRef, r.replicaPaths[i], "replicas"),
			Min:      minReplicas,
			Max:      maxReplicas,
			Baseline: &baselineReplicas,
		})
	}
	return parameters
}
