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
				"deployment/test/test/cpu",
				"deployment/test/test/memory",
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
				"deployment/test/test1/cpu",
				"deployment/test/test1/memory",
				"deployment/test/test2/cpu",
				"deployment/test/test2/memory",
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
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "replicas"},
				},
			},
			expected: []string{
				"deployment/test1/test/cpu",
				"deployment/test1/test/memory",
				"deployment/test2/test/cpu",
				"deployment/test2/test/memory",
				"deployment/test1/replicas",
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
				"deployment/test1/test1/cpu",
				"deployment/test1/test1/memory",
				"deployment/test1/test2/cpu",
				"deployment/test1/test2/memory",
				"deployment/test2/test/cpu",
				"deployment/test2/test/memory",
			},
		},

		{
			desc: "pythagorean",
			selected: []pnode{
				{
					meta:      meta("Deployment", "a-b"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=c]", "resources"},
				},
				{
					meta:      meta("Deployment", "a"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=b]", "resources"},
				},
				{
					meta:      meta("Deployment", "a"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=c]", "resources"},
				},
				{
					meta:      meta("Deployment", "a-b"),
					fieldPath: []string{"spec", "replicas"},
				},
				{
					meta:      meta("Statefulset", "a-b"),
					fieldPath: []string{"spec", "replicas"},
				},
			},
			expected: []string{
				"deployment/a-b/c/cpu",
				"deployment/a-b/c/memory",
				"deployment/a/b/cpu",
				"deployment/a/b/memory",
				"deployment/a/c/cpu",
				"deployment/a/c/memory",
				"deployment/a-b/replicas",
				"statefulset/a-b/replicas",
			},
		},

		{
			desc: "env",
			selected: []pnode{
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test2]", "env", "[name=MY_ENV_VAR]"},
				},
				{
					meta:      meta("Deployment", "test1"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test2]", "env", "[name=MY_SECOND_ENV_VAR]"},
				},
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=test2]", "env", "[name=MY_ENV_VAR]"},
				},
			},
			expected: []string{
				"deployment/test1/test2/env/my_env_var",
				"deployment/test1/test2/env/my_second_env_var",
				"deployment/test2/test2/env/my_env_var",
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
				switch sel.fieldPath[len(sel.fieldPath)-1] {
				case "resources":
					actual = append(actual, namer(sel.meta, sel.fieldPath, "cpu"))
					actual = append(actual, namer(sel.meta, sel.fieldPath, "memory"))
				case "replicas":
					actual = append(actual, namer(sel.meta, sel.fieldPath, "replicas"))
				default:
					// We'll assume this is for environment variables
					actual = append(actual, namer(sel.meta, sel.fieldPath, ""))
				}
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
