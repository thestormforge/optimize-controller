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
				"deployment/test/test/resources/cpu",
				"deployment/test/test/resources/memory",
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
				"deployment/test/test1/resources/cpu",
				"deployment/test/test1/resources/memory",
				"deployment/test/test2/resources/cpu",
				"deployment/test/test2/resources/memory",
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
				"deployment/test1/test/resources/cpu",
				"deployment/test1/test/resources/memory",
				"deployment/test2/test/resources/cpu",
				"deployment/test2/test/resources/memory",
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
				"deployment/test1/test1/resources/cpu",
				"deployment/test1/test1/resources/memory",
				"deployment/test1/test2/resources/cpu",
				"deployment/test1/test2/resources/memory",
				"deployment/test2/test/resources/cpu",
				"deployment/test2/test/resources/memory",
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
				"deployment/a-b/c/resources/cpu",
				"deployment/a-b/c/resources/memory",
				"deployment/a/b/resources/cpu",
				"deployment/a/b/resources/memory",
				"deployment/a/c/resources/cpu",
				"deployment/a/c/resources/memory",
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
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=env]", "env", "[name=MY_ENV_VAR]"},
				},
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=env]", "env", "[name=cpu]"},
				},
				{
					meta:      meta("Deployment", "test2"),
					fieldPath: []string{"spec", "template", "spec", "containers", "[name=env]", "env", "[name=env]"},
				},
			},
			expected: []string{
				"deployment/test1/test2/env/MY_ENV_VAR",
				"deployment/test1/test2/env/MY_SECOND_ENV_VAR",
				"deployment/test2/test2/env/MY_ENV_VAR",
				"deployment/test2/env/env/MY_ENV_VAR", // :troll:
				"deployment/test2/env/env/cpu",        // :troll:
				"deployment/test2/env/env/env",        // :troll:
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			namer := parameterNamer()

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
