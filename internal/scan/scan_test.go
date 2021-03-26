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

package scan

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

func TestGenericSelector_Select(t *testing.T) {
	cases := []struct {
		desc     string
		selector GenericSelector
		input    string
		expected string
	}{
		{
			desc: "version regexp with group",
			selector: GenericSelector{
				Group:   "foo",
				Version: "v\\d",
			},
			input: `
apiVersion: v1
kind: foo
---
apiVersion: foo/v1
kind: foo
`,
			expected: `
apiVersion: foo/v1
kind: foo
`,
		},

		{
			desc: "version regexp",
			selector: GenericSelector{
				Version: "v\\d",
			},
			input: `
apiVersion: v1
kind: foo
---
apiVersion: foo/v1
kind: foo
`,
			expected: `
apiVersion: v1
kind: foo
---
apiVersion: foo/v1
kind: foo
`,
		},

		{
			desc: "group regexp",
			selector: GenericSelector{
				Group: "apps|extensions",
			},
			input: `
apiVersion: apps/v1
kind: Deployment
---
apiVersion: extensions/v1beta1
kind: Deployment
---
apiVersion: v1
kind: ConfigMap
`,
			expected: `
apiVersion: apps/v1
kind: Deployment
---
apiVersion: extensions/v1beta1
kind: Deployment
`,
		},

		{
			desc: "missing group edge case",
			selector: GenericSelector{
				Version: "v1",
			},
			input: `
apiVersion: /v1
kind: ConfigMap
`,
			expected: `
apiVersion: /v1
kind: ConfigMap
`,
		},

		{
			desc: "missing group",
			selector: GenericSelector{
				Group:   "[^/]?",
				Version: "[^/]?",
			},
			input: `
apiVersion: /
kind: ConfigMap
`,
			expected: `
apiVersion: /
kind: ConfigMap
`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			nodes, err := kio.FromBytes([]byte(c.input))
			require.NoError(t, err)

			nodes, err = c.selector.Select(nodes)
			require.NoError(t, err)

			actual, err := kio.StringAll(nodes)
			if assert.NoError(t, err) {
				assert.Equal(t, strings.TrimSpace(c.expected), strings.TrimSpace(actual))
			}
		})
	}
}
