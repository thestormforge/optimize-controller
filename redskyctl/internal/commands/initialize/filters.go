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

package initialize

import (
	"bytes"

	"github.com/redskyops/redskyops-controller/internal/version"
	"sigs.k8s.io/kustomize/api/filters/labels"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// labelFilter returns a filter that applies the configured labels
func (o *GeneratorOptions) labelFilter() kio.Filter {
	f := labels.Filter{
		Labels: map[string]string{
			"app.kubernetes.io/version": version.GetInfo().Version,
		},
		FsSlice: []types.FieldSpec{
			{
				Gvk: resid.Gvk{
					Kind: "Deployment",
				},
				Path:               "spec/template/metadata/labels",
				CreateIfNotPresent: true,
			},
			{
				Path:               "metadata/labels",
				CreateIfNotPresent: true,
			},
		},
	}

	for k, v := range o.labels {
		f.Labels[k] = v
	}

	return f
}

// clusterRoleBindingFilter returns a filter that removes cluster role bindings for namespaced deployments
func (o *GeneratorOptions) clusterRoleBindingFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		output := make([]*yaml.RNode, 0, len(nodes))
		for _, n := range nodes {
			m, err := n.GetMeta()
			if err != nil {
				return nil, err
			}

			if m.Kind == "ClusterRoleBinding" && m.APIVersion == "rbac.authorization.k8s.io/v1" {
				continue
			}

			output = append(output, n)
		}
		return output, nil
	})
}

// envFromSecretFilter comments out the `envFrom` field of the deployment
func (o *GeneratorOptions) envFromSecretFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, n := range nodes {
			m, err := n.GetMeta()
			if err != nil {
				return nil, err
			}

			if m.Kind != "Deployment" || m.Name != "redsky-controller-manager" {
				continue
			}

			if err := commentOutEnvFrom(n); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func commentOutEnvFrom(node *yaml.RNode) error {
	switch node.YNode().Kind {
	case yaml.MappingNode:
		// Recurse into each key/value in the map
		return node.VisitFields(func(node *yaml.MapNode) error {
			if err := commentOutEnvFrom(node.Value); err != nil {
				return err
			}

			// Only process the "containers" list
			if node.Key.YNode().Value != "containers" || node.Value.YNode().Kind != yaml.SequenceNode {
				return nil
			}

			// Iterate over the containers
			cs, _ := node.Value.Elements()
			for _, c := range cs {
				con := c.YNode().Content
				for i := 0; i < len(con); i = i + 2 {
					if con[i].Value == "envFrom" {
						// Convert the "envFrom" node into a YAML document and encode it
						var buf bytes.Buffer
						enc := yaml.NewEncoder(&buf)
						err := enc.Encode(&yaml.Node{
							Kind: yaml.DocumentNode,
							Content: []*yaml.Node{
								{
									Kind:    yaml.MappingNode,
									Content: con[i : i+2],
								},
							},
						})
						if err != nil {
							return err
						}

						// Set the encoded YAML as the comment on the next element
						con[i+2].HeadComment = buf.String()

						// Remove the real "envFrom"
						copy(con[i:], con[i+2:])
						c.YNode().Content = con[:len(con)-2]
						break
					}
				}
			}
			return nil
		})
	case yaml.SequenceNode:
		// Recurse into each element in the list
		return node.VisitElements(func(node *yaml.RNode) error {
			return commentOutEnvFrom(node)
		})
	default:
		return nil
	}
}
