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

package kustomize

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/api/types"
)

func Test(t *testing.T) {

	testCases := []struct {
		desc     string
		options  []Option
		expected struct {
			Namespace string
			Image     string
		}
	}{
		{
			desc:    "default",
			options: []Option{WithInstall()},
			expected: struct {
				Namespace string
				Image     string
			}{
				Namespace: "redsky-system",
				Image:     BuildImage,
			},
		},
		{
			desc:    "custom namespace",
			options: []Option{WithInstall(), WithNamespace("trololololo")},
			expected: struct {
				Namespace string
				Image     string
			}{
				Namespace: "trololololo",
				Image:     BuildImage,
			},
		},
		{
			desc:    "custom image",
			options: []Option{WithInstall(), WithImage("mycoolregistry.com/image:tag")},
			expected: struct {
				Namespace string
				Image     string
			}{
				Namespace: "redsky-system",
				Image:     "mycoolregistry.com/image:tag",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			k, err := NewKustomization(tc.options...)
			assert.NoError(t, err)

			res, err := k.Run(k.Base)
			assert.NoError(t, err)
			assert.Equal(t, res.Size(), 6)

			r, err := res.Select(types.Selector{Name: "redsky-controller-manager"})
			assert.NoError(t, err)
			assert.Len(t, r, 1)
			assert.Equal(t, r[0].GetNamespace(), tc.expected.Namespace)

			if tc.expected.Image != "" {
				assert.Contains(t, r[0].String(), tc.expected.Image)
			}
		})
	}
}
