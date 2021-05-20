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

import "sigs.k8s.io/kustomize/kyaml/yaml"

// PreserveFieldMatcherPath wraps a field matcher so the resulting nodes have
// their field path set accordingly (incoming path + field name).
func PreserveFieldMatcherPath(fieldMatcher yaml.FieldMatcher) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		result, err := fieldMatcher.Filter(node)
		if err != nil {
			return nil, err
		}

		result.AppendToFieldPath(node.FieldPath()...)
		result.AppendToFieldPath(fieldMatcher.Name)
		return result, nil
	})
}
