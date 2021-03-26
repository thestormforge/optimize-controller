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

package scan

import (
	"github.com/thestormforge/konjure/pkg/filters"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Selector is used to filter (reduce) a list of resource nodes and then
// map them into something that can be consumed by a transformer.
type Selector interface {
	Select([]*yaml.RNode) ([]*yaml.RNode, error)
	Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error)
}

// GenericSelector can be used to implement the "Select" part of the Selector
// interface. The "*Selector" fields are treated as Kubernetes selectors, all
// other fields are regular expressions for matching metadata.
type GenericSelector filters.ResourceMetaFilter

// Select reduces the supplied resource node slice by only returning those
// nodes which match this selector.
func (g *GenericSelector) Select(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	return (*filters.ResourceMetaFilter)(g).Filter(nodes)
}

// Transformer consumes the aggregated outputs from the selectors and
// turns them back into resource nodes. The original resource nodes used
// to generate the inputs are also made available.
type Transformer interface {
	Transform(nodes []*yaml.RNode, selected []interface{}) ([]*yaml.RNode, error)
}

// Scanner is used to map resource nodes to an intermediate type and back to
// (a likely different collection of) resource nodes. For example, a scanner
// could be used to "select" all the "name" labels from a slice of resource
// nodes, the "transform" those names back into a ConfigMap resource node that
// contains all of the discovered values.
type Scanner struct {
	// The list of selectors that produce intermediate representations.
	Selectors []Selector
	// The mapper that produces resource nodes from the intermediate representations.
	Transformer Transformer
}

var _ kio.Filter = &Scanner{}

// Filter performs the "selection" and "transformation" of resource nodes to
// intermediate representations and back to resource nodes.
func (s *Scanner) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	var selected []interface{}
	for _, sel := range s.Selectors {
		snodes, err := sel.Select(nodes)
		if err != nil {
			return nil, err
		}

		for _, node := range snodes {
			meta, err := node.GetMeta()
			if err != nil {
				return nil, err
			}

			mapped, err := sel.Map(node, meta)
			if err != nil {
				return nil, err
			}

			selected = append(selected, mapped...)
		}
	}

	return s.Transformer.Transform(nodes, selected)
}
