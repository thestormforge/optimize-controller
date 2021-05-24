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

import (
	"strings"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// FieldPath evaluates a path template using the supplied context and then
// splits it into individual path segments (honoring escaped slashes).
func FieldPath(p string, data map[string]string) ([]string, error) {
	// Evaluate the path as a Go Template
	t, err := template.New("path").
		Delims("{", "}").
		Option("missingkey=zero").
		Parse(p)
	if err != nil {
		return nil, err
	}
	var pathBuf strings.Builder
	if err := t.Execute(&pathBuf, data); err != nil {
		return nil, err
	}

	// Remove the leading slash to prevent empty elements
	path := strings.TrimLeft(pathBuf.String(), "/")
	if path == "" {
		return nil, nil
	} else if !strings.Contains(path, `\/`) {
		return strings.Split(path, "/"), nil
	}

	// Handle escaped slashes using a temporary placeholder
	// NOTE: This just mimics the logic of the equivalent code in the Kustomize FieldPath
	var result []string
	for _, pp := range strings.Split(strings.ReplaceAll(path, `\/`, `???`), "/") {
		result = append(result, strings.ReplaceAll(pp, `???`, `/`))
	}
	return result, nil
}

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
