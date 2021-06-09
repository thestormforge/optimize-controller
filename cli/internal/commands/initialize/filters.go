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
	"github.com/thestormforge/optimize-controller/v2/internal/version"
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
