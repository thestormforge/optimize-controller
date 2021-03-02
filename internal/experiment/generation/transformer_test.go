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

package generation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestParameterNames(t *testing.T) {
	cases := []struct {
		desc     string
		selected []pnode
		expected []string
	}{
		{
			desc: "empty",
		},

		{
			desc: "one deployment one container",
			selected: []pnode{
				{
					meta:      meta("Deployment", "test"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test]", "resources"},
				},
			},
			expected: []string{
				"cpu",
				"memory",
			},
		},

		{
			desc: "one deployment two containers",
			selected: []pnode{
				{
					meta:      meta("Deployment", "test"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test1]", "resources"},
				},
				{
					meta:      meta("Deployment", "test"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test2]", "resources"},
				},
			},
			expected: []string{
				"test1_cpu",
				"test1_memory",
				"test2_cpu",
				"test2_memory",
			},
		},

		{
			desc: "two deployments one container",
			selected: []pnode{
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test]", "resources"},
				},
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test]", "resources"},
				},
			},
			expected: []string{
				"test1_cpu",
				"test1_memory",
				"test2_cpu",
				"test2_memory",
			},
		},

		{
			desc: "two deployments two containers",
			selected: []pnode{
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test1]", "resources"},
				},
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test2]", "resources"},
				},
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test]", "resources"},
				},
			},
			expected: []string{
				"test1_test1_cpu",
				"test1_test1_memory",
				"test1_test2_cpu",
				"test1_test2_memory",
				"test2_cpu",
				"test2_memory",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var selected []interface{}
			for i := range c.selected {
				selected = append(selected, &c.selected[i])
			}
			namer := parameterNamer(selected)

			var actual []string
			for _, sel := range c.selected {
				actual = append(actual, namer(sel.meta, sel.fieldPath, "cpu"))
				actual = append(actual, namer(sel.meta, sel.fieldPath, "memory"))
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}

func meta(kind, name string) yaml.ResourceMeta {
	return yaml.ResourceMeta{
		TypeMeta: yaml.TypeMeta{
			Kind: kind,
		},
		ObjectMeta: yaml.ObjectMeta{
			NameMeta: yaml.NameMeta{
				Name: name,
			},
		},
	}
}
