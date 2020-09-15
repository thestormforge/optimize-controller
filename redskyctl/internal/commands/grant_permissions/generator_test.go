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

package grant_permissions_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/stretchr/testify/assert"
)

const (
	defaultClusterRole = "redsky-patching-role"
	builtinClusterRole = "redsky-builtin-role"
)

func TestGrantPermissions(t *testing.T) {

	testCases := []struct {
		desc               string
		args               []string
		expectedError      bool
		expectedPatterns   []string
		unexpectedPatterns []string
	}{
		{
			desc: "default",
			expectedPatterns: []string{
				defaultClusterRole,
				builtinClusterRole,
				"name: builtin",
			},
		},
		{
			desc: "skip builtin",
			args: []string{"--skip-builtin"},
			expectedPatterns: []string{
				defaultClusterRole,
			},
			unexpectedPatterns: []string{
				builtinClusterRole,
			},
		},
		{
			desc: "skip default",
			args: []string{"--skip-default"},
			expectedPatterns: []string{
				builtinClusterRole,
			},
			unexpectedPatterns: []string{
				defaultClusterRole,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			// Create a global configuration
			cfg := &config.RedSkyConfig{}
			cmd := grant_permissions.NewGeneratorCommand(&grant_permissions.GeneratorOptions{Config: cfg})
			commander.ConfigGlobals(cfg, cmd)

			var b bytes.Buffer
			cmd.SetOut(&b)
			if len(tc.args) > 0 {
				cmd.SetArgs(tc.args)
			}

			err := cmd.Execute()
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify output has what we're looking for
			for _, text := range tc.expectedPatterns {
				assert.Contains(t, b.String(), text)
			}

			// Verify output doesn't contain these
			for _, text := range tc.unexpectedPatterns {
				assert.NotContains(t, b.String(), text)
			}

		})
	}
}
