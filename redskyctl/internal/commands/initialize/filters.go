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

			manager, err := yaml.Lookup("spec", "template", "spec", "containers", "[name=manager]").Filter(n)
			if err != nil {
				return nil, err
			}

			if err := commentOutField(manager, "envFrom"); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func commentOutField(node *yaml.RNode, fieldName string) error {
	if err := yaml.ErrorIfInvalid(node, yaml.MappingNode); err != nil {
		return err
	}

	con := node.YNode().Content
	for i := 0; i < len(con); i = i + 2 {
		if con[i].Value == fieldName {
			// Encode the field name and it's value as a YAML string
			comment, err := encodeMappingNode(con[i], con[i+1])
			if err != nil {
				return err
			}

			// Stick the encoded YAML in the comment of the next element
			// FIXME You can't comment out the last element
			con[i+2].HeadComment = comment

			// Remove the real nodes
			copy(con[i:], con[i+2:])
			node.YNode().Content = con[:len(con)-2]
			return nil
		}
	}
	return nil
}

func encodeMappingNode(key, value *yaml.Node) (string, error) {
	// Convert the node into a YAML document and encode it
	var buf bytes.Buffer
	err := yaml.NewEncoder(&buf).Encode(&yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{
				Kind:    yaml.MappingNode,
				Content: []*yaml.Node{key, value},
			},
		},
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
