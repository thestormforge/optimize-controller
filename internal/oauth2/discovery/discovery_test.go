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

package discovery

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWellKnownURI(t *testing.T) {
	baseURL := "http://example.com"
	name := "foo"

	testCases := []struct {
		desc     string
		id       string
		name     string
		expected string
	}{
		{
			desc:     "default",
			id:       baseURL,
			name:     "",
			expected: "http://example.com/.well-known/",
		},
		{
			desc:     "default",
			id:       baseURL,
			name:     name,
			expected: "http://example.com/.well-known/foo",
		},
		{
			desc:     "default",
			id:       baseURL + "/",
			name:     name,
			expected: "http://example.com/.well-known/foo",
		},
		{
			desc:     "default",
			id:       baseURL + "/x",
			name:     name,
			expected: "http://example.com/.well-known/foo/x",
		},
		{
			desc:     "default",
			id:       "",
			name:     "",
			expected: "/.well-known/",
		},
		{
			desc:     "default",
			id:       "",
			name:     name,
			expected: "/.well-known/foo",
		},
		{
			desc:     "default",
			id:       "/",
			name:     name,
			expected: "/.well-known/foo",
		},
		{
			desc:     "default",
			id:       "/x",
			name:     name,
			expected: "/.well-known/foo/x",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			assert.Equal(t, tc.expected, WellKnownURI(tc.id, tc.name))
		})
	}
}
