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

package assets

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssets(t *testing.T) {
	testPhrase := "TheyreAlwaysAfterMeLuckyCharms"

	testCases := []struct {
		desc      string
		asset     Asset
		namespace string
		labels    map[string]string
	}{
		{
			desc:  "default manager ( deployment + namespace )",
			asset: Manager,
		},
		{
			desc:  "experiment crd",
			asset: RedskyopsDevExperiments,
		},
		{
			desc:  "trial crd",
			asset: RedskyopsDevTrials,
		},
		{
			desc:  "default role",
			asset: Role,
		},
		{
			desc:  "default role binding",
			asset: RbacRoleBinding,
		},
		{
			desc:      "manager ( custom namespace )",
			asset:     Manager,
			namespace: testPhrase,
		},
		{
			desc:   "manager ( custom labels)",
			asset:  Manager,
			labels: map[string]string{testPhrase: testPhrase},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			var (
				err    error
				output string
			)

			output, err = tc.asset.String()
			assert.NoError(t, err)

			output, err = tc.asset.InjectMetadata(tc.namespace, tc.labels)
			assert.NoError(t, err)

			if tc.namespace != "" {
				assert.Contains(t, output, tc.namespace)
			}

			if len(tc.labels) > 0 {
				assert.Contains(t, output, testPhrase)
			}
		})
	}
}
