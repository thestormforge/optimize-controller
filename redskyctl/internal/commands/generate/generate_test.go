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

package generate_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	experimentFile, err := ioutil.TempFile("", "trial")
	require.NoError(t, err)
	_, err = experimentFile.Write(experiment)
	require.NoError(t, err)

	defer os.Remove(experimentFile.Name())

	rsConfig, err := ioutil.TempFile("", "rsConfig")
	require.NoError(t, err)
	_, err = rsConfig.Write(configData)
	require.NoError(t, err)

	defer os.Remove(rsConfig.Name())

	testCases := []struct {
		desc               string
		args               []string
		expectedError      bool
		expectedPatterns   []string
		unexpectedPatterns []string
	}{
		{
			// TODO: I'd expect this to exit 1, but it doesn't?
			desc:          "no args",
			expectedError: false,
		},
		{
			desc:          "gen install",
			args:          []string{"install"},
			expectedError: false,
			expectedPatterns: []string{
				"kind: CustomResourceDefinition",
				"kind: Namespace",
				"kind: Deployment",
				"kind: ClusterRole",
				"kind: ClusterRoleBinding",
			},
		},
		{
			desc: "gen install custom namespace",
			args: []string{
				"install",
				"--redskyconfig",
				rsConfig.Name(),
			},
			expectedError: false,
			expectedPatterns: []string{
				"namespace: testns",
			},
			unexpectedPatterns: []string{
				"namespace: redsky-system",
			},
		},
		{
			desc: "gen install custom image",
			args: []string{
				"install",
				"--image",
				"gcr.io/redskyops/funsies:latest",
			},
			expectedError: false,
			expectedPatterns: []string{
				"image: gcr.io/redskyops/funsies:latest",
			},
			unexpectedPatterns: []string{
				fmt.Sprintf("%s: %s", "image", kustomize.BuildImage),
			},
		},
		// TODO: Revisit gen secret after we get errors surfacing to redskyctl/main.go
		// calling commander.ExitOnError interrupts the test on failures
		/*
			{
				desc:          "gen rbac (no args)",
				args:          []string{"rbac"},
				expectedError: false,
			},
		*/
		{
			// TODO: Need to get an experiment that actually requires additional rbac
			desc: "gen rbac",
			args: []string{
				"rbac",
				"--filename", experimentFile.Name(),
			},
			expectedError: false,
		},
		// TODO: Revisit gen secret after we get errors surfacing to redskyctl/main.go
		// calling commander.ExitOnError interrupts the test on failures
		/*
			{
				desc: "gen secret (no config)",
				args: []string{
					"secret",
					"--redskyconfig",
					"/dev/null",
				},
				expectedError: true,
			},
		*/
		{
			desc:          "gen trial (no args)",
			args:          []string{"trial"},
			expectedError: true,
		},
		{
			desc: "gen trial",
			args: []string{
				"trial",
				"--filename", experimentFile.Name(),
				"--assign", "memory=500",
				"--assign", "cpu=500",
			},
			expectedError: false,
			expectedPatterns: []string{
				"value: 500",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			// Create a global configuration
			cfg := &config.RedSkyConfig{}
			cmd := generate.NewCommand(&generate.Options{Config: cfg})
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

var experiment = []byte(`apiVersion: redskyops.dev/v1beta1
kind: Experiment
metadata:
  name: postgres-example
spec:
  parameters:
  - name: memory
    min: 500
    max: 4000
  - name: cpu
    min: 100
    max: 4000`)

var configData = []byte(`
authorizations:
- authorization:
    credential:
      access_token: "123"
      expiry: "123"
      refresh_token: "123"
      token_type: Bearer
  name: dev
contexts:
- context:
    authorization: dev
    server: dev
  name: dev
controllers:
- controller:
    registration_client_uri:
    namespace: testns
  name: dev
current-context: dev`)
