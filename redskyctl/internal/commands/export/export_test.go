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

package export

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/server"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestReadExperiment(t *testing.T) {
	_, tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	cfg := &config.RedSkyConfig{}
	cfg.Load()

	testCases := []struct {
		desc          string
		expectedError bool
		e             *ExportCommand
	}{
		{
			// no filename
			desc:          "default",
			expectedError: true,
			e:             &ExportCommand{},
		},
		{
			desc:          "default with file",
			expectedError: false,
			e:             &ExportCommand{filename: tmpfile.Name()},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			err := tc.e.ReadExperimentFile()
			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestLookupTrial(t *testing.T) {
	exp, tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	testCases := []struct {
		desc          string
		expectedError bool
		e             *ExportCommand
		trialName     string
	}{
		{
			desc:          "default",
			expectedError: false,
			e:             &ExportCommand{RedSkyAPI: &fakeRedSkyServer{}, experiment: exp},
			trialName:     "test-1234",
		},
		{
			desc:          "no api",
			expectedError: true,
			e:             &ExportCommand{experiment: exp},
		},
		{
			desc:          "bad trial",
			expectedError: true,
			e:             &ExportCommand{RedSkyAPI: &fakeRedSkyServer{}, experiment: exp},
			trialName:     "test-test",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			trialNames, err := experiments.ParseNames(append([]string{"trials", tc.trialName}))
			assert.NoError(t, err)

			trial, err := tc.e.GetTrial(context.Background(), trialNames[0])
			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, len(trial.Spec.Assignments), 2)
		})
	}
}

func TestPatches(t *testing.T) {
	exp, tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	trial := &redsky.Trial{}
	server.ToClusterTrial(trial, &wannabeTrial.TrialAssignments)
	trial.ObjectMeta.Labels = map[string]string{
		redsky.LabelExperiment: "test",
	}

	pgfile, err := ioutil.TempFile("", "deployment")
	require.NoError(t, err)

	_, err = pgfile.Write(pgDeployment)
	require.NoError(t, err)
	defer os.Remove(pgfile.Name())

	testCases := []struct {
		desc          string
		expectedError bool
		e             *ExportCommand
	}{
		{
			desc:          "default",
			expectedError: false,
			e:             &ExportCommand{Config: &fakeClient{filename: pgfile.Name()}, trial: trial, experiment: exp},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			pnt, err := tc.e.Patches(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, len(pnt), 1)

			// TODO this name doesnt seem right, need to get that sorted
			assert.Equal(t, "-Deployment-postgres", pnt[0].TargetName)
		})
	}
}

func TestRenderPatch(t *testing.T) {
	exp, tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	trial := &redsky.Trial{}
	server.ToClusterTrial(trial, &wannabeTrial.TrialAssignments)
	trial.ObjectMeta.Labels = map[string]string{
		redsky.LabelExperiment: "test",
	}

	pgfile, err := ioutil.TempFile("", "deployment")
	require.NoError(t, err)

	_, err = pgfile.Write(pgDeployment)
	require.NoError(t, err)
	defer os.Remove(pgfile.Name())

	testCases := []struct {
		desc          string
		expectedError bool
		e             *ExportCommand
	}{
		{
			desc:          "default",
			expectedError: false,
			e:             &ExportCommand{Config: &fakeClient{filename: pgfile.Name()}, trial: trial, experiment: exp},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			pnt, err := tc.e.Patches(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, len(pnt), 1)

			output, err := patchResource(pnt)
			assert.NoError(t, err)

			// With []byte from output, we should try to turn it into an actual deployment
			deployment := &appsv1.Deployment{}
			err = yaml.Unmarshal(output, deployment)
			assert.NoError(t, err)

			limits := deployment.Spec.Template.Spec.Containers[0].Resources.Limits
			/*
				yolo := limits[corev1.ResourceCPU]
				fmt.Println((&yolo).MilliValue())
				oloy := limits[corev1.ResourceMemory]
				fmt.Println((&oloy).MilliValue())
			*/

			// wannabe doesnt contain the actual units ( m/Mi/Gi/etc )
			// but since we're testing and know it, we'll just hardcode
			expectedCPU := resource.MustParse(fmt.Sprintf("%s%s", wannabeTrial.TrialAssignments.Assignments[0].Value, "m"))
			expectedMemory := resource.MustParse(fmt.Sprintf("%s%s", wannabeTrial.TrialAssignments.Assignments[1].Value, "Mi"))

			assert.True(t, limits[corev1.ResourceCPU].Equal(expectedCPU))
			assert.True(t, limits[corev1.ResourceMemory].Equal(expectedMemory))
		})
	}
}
