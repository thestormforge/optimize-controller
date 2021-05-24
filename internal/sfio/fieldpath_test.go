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
)

func TestFieldPath(t *testing.T) {
	cases := []struct {
		desc     string
		path     string
		data     map[string]string
		expected []string
	}{
		{
			desc: "empty",
		},
		{
			desc: "tricky empty",
			path: `/{ .ItsATrap }`,
			data: map[string]string{"ItsATrap": "/"},
		},
		{
			desc:     "simple",
			path:     `/spec/replicas`,
			expected: []string{"spec", "replicas"},
		},
		{
			desc:     "label",
			path:     `/metadata/labels/app.kubernetes.io\/name`,
			expected: []string{"metadata", "labels", "app.kubernetes.io/name"},
		},
		{
			desc:     "parameter",
			path:     `/spec/containers/[name={.ContainerName}]/image`,
			data:     map[string]string{"ContainerName": "testing"},
			expected: []string{"spec", "containers", "[name=testing]", "image"},
		},
		{
			desc:     "leading slash is optional",
			path:     `spec/replicas`,
			expected: []string{"spec", "replicas"},
		},
		{
			desc:     "run of leading slashes",
			path:     `//spec/replicas`,
			expected: []string{"spec", "replicas"},
		},
		{
			desc:     "no data",
			path:     `/spec/{ .WhichWayDidHeGo }`,
			expected: []string{"spec", ""},
		},
		{
			desc:     "consistent with Kustomize",
			path:     `/a\/b/c???d`,
			expected: []string{"a/b", "c/d"}, // Should be {"a/b", "c???d"}
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := FieldPath(c.path, c.data)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}
