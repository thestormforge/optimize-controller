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
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const testInput = `# TestInput
spec:
  foo:
    test: testing
    listing:
    - name: z
      value: ZZ
  bar:
  - name: a
    value: AA
  - name: b
    listValue:
    - id: x
      value: XX`

func TestTeeMatchedFilter_Filter_FieldPath(t *testing.T) {
	cases := []struct {
		desc               string
		path               []string
		input              *yaml.RNode
		expectedFieldPaths [][]string
	}{
		{
			desc:  "no path matches",
			path:  []string{"spec", "foo", "test"},
			input: yaml.MustParse(testInput),
			expectedFieldPaths: [][]string{
				{"spec", "foo", "test"},
			},
		},

		{
			desc:  "single path match",
			path:  []string{"spec", "foo", "listing", "[name=.*]", "value"},
			input: yaml.MustParse(testInput),
			expectedFieldPaths: [][]string{
				{"spec", "foo", "listing", "[name=z]", "value"},
			},
		},

		{
			// This test is important because it turns out PathMatcher records the matches in
			// reverse order (i.e. PathMatcher.Matches will have the "id" value followed by the "name" value)
			desc:  "multiple path matches",
			path:  []string{"spec", "bar", "[name=.*]", "listValue", "[id=.*]", "value"},
			input: yaml.MustParse(testInput),
			expectedFieldPaths: [][]string{
				{"spec", "bar", "[name=b]", "listValue", "[id=x]", "value"},
			},
		},

		{
			desc:  "multiple single path matches",
			path:  []string{"spec", "bar", "[name=.*]"},
			input: yaml.MustParse(testInput),
			expectedFieldPaths: [][]string{
				{"spec", "bar", "[name=a]"},
				{"spec", "bar", "[name=b]"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			// Create a filter to capture all of the field paths we encounter
			var actualFieldPaths [][]string
			captureFieldPathFilter := yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
				actualFieldPaths = append(actualFieldPaths, node.FieldPath())
				return node, nil
			})

			pathMatcher := yaml.PathMatcher{Path: c.path}
			_, err := TeeMatched(pathMatcher, captureFieldPathFilter).Filter(c.input)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expectedFieldPaths, actualFieldPaths)
			}
		})
	}
}
