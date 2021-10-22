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

package export_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thestormforge/konjure/pkg/filters"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/export"
	"github.com/thestormforge/optimize-go/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"
)

func TestPatchExperiment(t *testing.T) {
	_, expBytes, expFile := createTempExperimentFile(t)
	defer os.Remove(expFile.Name())

	manifestFile := createTempManifests(t)
	defer os.Remove(manifestFile.Name())

	testCases := []struct {
		desc  string
		args  []string
		stdin io.Reader
	}{
		{
			desc: "exp file manifest file",
			args: []string{
				"--filename", expFile.Name(),
				"--filename", manifestFile.Name(),
				"sampleExperiment-1234",
			},
		},
		{
			desc: "exp stdin manifest file",
			args: []string{
				"--filename", "-",
				"--filename", manifestFile.Name(),
				"sampleExperiment-1234",
			},
			stdin: bytes.NewReader(expBytes),
		},
		{
			desc: "exp file manifest stdin",
			args: []string{
				"--filename", expFile.Name(),
				"--filename", "-",
				"sampleExperiment-1234",
			},
			stdin: bytes.NewReader(pgDeployment),
		},
		{
			desc: "exp stdin manifest stdin",
			args: []string{
				"--filename", "-",
				"sampleExperiment-1234",
			},
			stdin: bytes.NewReader(append(expBytes, pgDeployment...)),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			cfg := &config.OptimizeConfig{}

			opts := &export.Options{Config: cfg}
			opts.ExperimentsAPI = &fakeExperimentsAPI{}
			cmd := export.NewCommand(opts)
			commander.ConfigGlobals(cfg, cmd)

			// setup output
			var b bytes.Buffer
			cmd.SetOut(&b)

			// setup input
			if tc.stdin != nil {
				cmd.SetIn(tc.stdin)
			}

			// set command args
			if len(tc.args) > 0 {
				cmd.SetArgs(tc.args)
			}

			err := cmd.Execute()
			require.NoError(t, err)

			exp, err := extractDeployment(b.Bytes(), "postgres")
			require.NoError(t, err)

			cpu := wannabeTrial.TrialAssignments.Assignments[0]
			mem := wannabeTrial.TrialAssignments.Assignments[1]
			cpuLimits := exp.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"]
			memLimits := exp.Spec.Template.Spec.Containers[0].Resources.Limits["memory"]

			assert.Equal(t, fmt.Sprintf("%sm", cpu.Value.String()), (&cpuLimits).String())
			assert.Equal(t, fmt.Sprintf("%sMi", mem.Value.String()), (&memLimits).String())
		})
	}
}

func TestPatchApplication(t *testing.T) {
	// All of these files get created in the same tempdir ( neat-o )
	// so we can 'cheat' kustomize/krusty by passing in basename(manifests)
	// to use the relative path and not have to go through wonky hoops
	_, _, expFile := createTempExperimentFile(t)
	defer os.Remove(expFile.Name())

	manifestFile := createTempManifests(t)
	defer os.Remove(manifestFile.Name())

	_, appFileBytes, appFile := createTempApplication(t, manifestFile.Name())
	defer os.Remove(appFile.Name())

	os.Setenv("STORMFORGER_JWT", "funnyjwtjokehere")
	defer os.Unsetenv("STORMFORGER_JWT")

	testCases := []struct {
		desc  string
		args  []string
		stdin io.Reader
	}{
		{
			desc: "app file manifest file",
			args: []string{
				"--filename", appFile.Name(),
				"sampleApplication-how-do-you-make-a-tissue-dance-put-a-little-boogie-in-it-1234",
			},
		},
		{
			desc: "app stdin manifest stdin",
			args: []string{
				"--filename", "-",
				"sampleExperiment-1234",
			},
			stdin: bytes.NewReader(append(appFileBytes, pgDeployment...)),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			cfg := &config.OptimizeConfig{}

			fs := filesys.MakeFsInMemory()
			err := fs.WriteFile(filepath.Base(manifestFile.Name()), pgDeployment)
			require.NoError(t, err)

			opts := &export.Options{Config: cfg, Fs: fs}
			opts.ExperimentsAPI = &fakeExperimentsAPI{}
			cmd := export.NewCommand(opts)
			commander.ConfigGlobals(cfg, cmd)

			// setup output
			var b bytes.Buffer
			cmd.SetOut(&b)

			// setup input
			if tc.stdin != nil {
				cmd.SetIn(tc.stdin)
			}

			// set command args
			if len(tc.args) > 0 {
				cmd.SetArgs(tc.args)
			}

			err = cmd.Execute()
			require.NoError(t, err)

			exp, err := extractDeployment(b.Bytes(), "postgres")
			require.NoError(t, err)

			cpu := wannabeTrial.TrialAssignments.Assignments[0]
			mem := wannabeTrial.TrialAssignments.Assignments[1]
			cpuLimits := exp.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"]
			memLimits := exp.Spec.Template.Spec.Containers[0].Resources.Limits["memory"]

			assert.Equal(t, fmt.Sprintf("%sm", cpu.Value.String()), (&cpuLimits).String())
			assert.Equal(t, fmt.Sprintf("%sMi", mem.Value.String()), (&memLimits).String())
		})
	}
}

func extractDeployment(input []byte, name string) (*appsv1.Deployment, error) {
	var deploymentBuf bytes.Buffer

	deploymentInput := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(input)}},
		Filters: []kio.Filter{&filters.ResourceMetaFilter{Group: appsv1.GroupName, Kind: "Deployment", Name: name}},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &deploymentBuf}},
	}
	if err := deploymentInput.Execute(); err != nil {
		return nil, err
	}

	exp := &appsv1.Deployment{}
	if err := yaml.Unmarshal(deploymentBuf.Bytes(), exp); err != nil {
		return nil, err
	}

	return exp, nil
}
