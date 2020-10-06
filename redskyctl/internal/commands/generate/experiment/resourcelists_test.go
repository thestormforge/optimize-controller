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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func TestFieldPath_Build(t *testing.T) {
	cases := []struct {
		desc      string
		fieldPath fieldPath
		leaf      interface{}
		expected  strategicpatch.JSONMap
	}{
		{
			desc: "empty",
		},
		{
			desc:      "single element",
			fieldPath: []string{"x"},
			leaf:      "test",
			expected: map[string]interface{}{
				"x": "test",
			},
		},
		{
			desc:      "nested element",
			fieldPath: []string{"x", "y"},
			leaf:      "test",
			expected: map[string]interface{}{
				"x": map[string]interface{}{
					"y": "test",
				},
			},
		},
		{
			desc:      "nested list last", // NOTE: This is not supported, it produces unexpected results
			fieldPath: []string{"x", "y{a=b}"},
			leaf:      "test",
			expected: map[string]interface{}{
				"x": map[string]interface{}{
					"y": "test",
				},
			},
		},
		{
			desc:      "nested list",
			fieldPath: []string{"x", "y{a=b}", "z"},
			leaf:      "test",
			expected: map[string]interface{}{
				"x": map[string]interface{}{
					"y": []interface{}{
						map[string]interface{}{
							"a": "b",
							"z": "test",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			assert.Equal(t, c.expected, c.fieldPath.build(c.leaf))
		})
	}
}

func TestFieldPath_Read(t *testing.T) {
	cases := []struct {
		desc      string
		fieldPath fieldPath
		from      interface{}
		expected  interface{}
	}{
		{
			desc: "empty",
		},
		{
			desc:      "single element",
			fieldPath: []string{"x"},
			from: map[string]interface{}{
				"x": "test",
			},
			expected: "test",
		},
		{
			desc:      "nested element",
			fieldPath: []string{"x", "y"},
			from: map[string]interface{}{
				"x": map[string]interface{}{
					"y": "test",
				},
			},
			expected: "test",
		},
		{
			desc:      "nested list",
			fieldPath: []string{"x", "y{a=b}", "z"},
			from: map[string]interface{}{
				"x": map[string]interface{}{
					"y": []interface{}{
						map[string]interface{}{
							"a": "c",
							"z": "foobar",
						},
						map[string]interface{}{
							"a": "b",
							"z": "test",
						},
					},
				},
			},
			expected: "test",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			assert.Equal(t, c.expected, c.fieldPath.read(c.from))
		})
	}
}
