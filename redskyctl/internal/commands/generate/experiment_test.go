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

package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExperimentOptions_DefaultName(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir := strings.ToLower(filepath.Base(wd))

	cases := []struct {
		filename string
		expected string
	}{
		{
			filename: "",
			expected: dir,
		},
		{
			filename: "/dev/fd/3",
			expected: dir,
		},
		{
			filename: "app.yaml",
			expected: dir,
		},
		{
			filename: "application.yaml",
			expected: dir,
		},
		{
			filename: "foo/app.yml",
			expected: "foo",
		},
		{
			filename: "/foo/bar/app.yml",
			expected: "bar",
		},
		{
			filename: "/foo/bar/application.yaml",
			expected: "bar",
		},
		{
			filename: "/foo/bar/my-application.yaml",
			expected: "my-application",
		},
		{
			filename: "/foo/bar/my-app.yaml",
			expected: "my-app",
		},
		{
			filename: "/foo/bar/x&#$@z.yaml",
			expected: "xz",
		},
		{
			filename: "foo-.-.-.-.yml",
			expected: "foo",
		},
		{
			filename: "/app.yaml",
			expected: "default",
		},
		{
			filename: "/app/application/app.yaml",
			expected: "default",
		},
	}
	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			actual := (&ExperimentOptions{Filename: c.filename}).defaultName()
			assert.Equal(t, c.expected, actual)
		})
	}
}
