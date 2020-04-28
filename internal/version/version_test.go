/*
Copyright 2019 GramLabs, Inc.

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

func TestVersionInfo(t *testing.T) {
	cases := []struct {
		desc            string
		versionInfo     *Info
		expectedVersion string
	}{
		{
			desc:            "default version",
			expectedVersion: defaultVersion,
		},
		{
			desc: "custom pre-release version",
			versionInfo: &Info{
				Version:       "v1.2.3-",
				BuildMetadata: "test",
			},
			expectedVersion: "v1.2.3-+test",
		},
		{
			desc: "custom release version",
			versionInfo: &Info{
				Version:       "v1.2.3",
				BuildMetadata: "test",
			},
			expectedVersion: "v1.2.3",
		},
		{
			desc:            "wonky version info",
			versionInfo:     &Info{},
			expectedVersion: defaultVersion,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			defer resetVersion()

			if c.versionInfo != nil {
				Version = c.versionInfo.Version
				BuildMetadata = c.versionInfo.BuildMetadata
			}
			testVersion := GetInfo()

			assert.Equal(t, c.expectedVersion, testVersion.String())
		})
	}
}

func resetVersion() {
	// Reset version info to defaults
	Version = defaultVersion
	BuildMetadata = ""
	GitCommit = ""
}
