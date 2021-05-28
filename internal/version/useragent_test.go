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
package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgent(t *testing.T) {

	cases := []struct {
		desc        string
		product     string
		comment     string
		expected    string
		versionInfo *Info
	}{
		{
			desc:     "default",
			product:  "",
			comment:  "",
			expected: "Optimize/0.0.0-source",
		},
		{
			desc:    "version",
			product: "",
			comment: "",
			versionInfo: &Info{
				Version: "v1.2.3",
			},
			expected: "Optimize/1.2.3",
		},
		{
			desc:     "product",
			product:  "testProduct",
			comment:  "",
			expected: "testProduct/0.0.0-source",
		},
		{
			desc:     "comment",
			product:  "testComment",
			comment:  "fluffy comment",
			expected: "testComment/0.0.0-source (fluffy comment)",
		},
		{
			desc:     "empty comment",
			product:  "empty",
			comment:  " (  ) ",
			expected: "empty/0.0.0-source",
		},
		{
			desc:     "empty comment",
			product:  "empty",
			comment:  " ",
			expected: "empty/0.0.0-source",
		},
		{
			desc:     "white space comment",
			product:  "empty",
			comment:  " ( test )",
			expected: "empty/0.0.0-source (test)",
		},
		{
			desc:    "build meta comment",
			product: "meta",
			comment: "test",
			versionInfo: &Info{
				Version:       "v1.2.3-next",
				BuildMetadata: "build.123",
			},
			expected: "meta/1.2.3-next (build.123; test)",
		},
		{
			desc:    "build meta comment",
			product: "meta",
			comment: "(test)",
			versionInfo: &Info{
				Version:       "v1.2.3",
				BuildMetadata: "build.123",
			},
			expected: "meta/1.2.3 (test)",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			defer resetVersion()

			if c.versionInfo != nil {
				Version = c.versionInfo.Version
				BuildMetadata = c.versionInfo.BuildMetadata
			}

			ua := UserAgent(c.product, c.comment, nil)

			assert.Equal(t, c.expected, ua.(*Transport).userAgent())
		})
	}
}
