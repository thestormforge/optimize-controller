package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/server"
	expapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
func TestExport(t *testing.T) {
	samplePatch := `spec:
  template:
    spec:
      containers:
        - name: postgres
          resources:
          limits:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"
          requests:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"`

	sampleExperiment := &redsky.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sampleExperiment",
			Namespace: "default",
		},
		Spec: redsky.ExperimentSpec{
			Parameters: []redsky.Parameter{},
			Metrics:    []redsky.Metric{},
			Patches: []redsky.PatchTemplate{
				{
					TargetRef: &corev1.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Name:       "postgres",
					},
					Patch: samplePatch,
				},
			},
			TrialTemplate: redsky.TrialTemplateSpec{},
		},
		Status: redsky.ExperimentStatus{},
	}

	tmpfile, err := ioutil.TempFile("", "experiment")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	b, err := json.Marshal(sampleExperiment)
	require.NoError(t, err)
	_, err = tmpfile.Write(b)
	require.NoError(t, err)

	// TODO add filename: parameter so we can mock
	cfg := &config.RedSkyConfig{}
	cfg.Load()

	testCases := []struct {
		desc string
		args []string
	}{
		{
			desc: "default",
			args: []string{
				"als-368",
				"--filename",
				tmpfile.Name(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {

			var b bytes.Buffer
			cmd := NewCommand(cfg)
			cmd.SetOut(&b)

			if len(tc.args) > 0 {
				cmd.SetArgs(tc.args)
			}

			err := cmd.Execute()
			assert.NoError(t, err)
			log.Println(b.String())
		})
	}
}
*/

func TestReadExperiment(t *testing.T) {
	_, tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	// TODO add filename: parameter so we can mock
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

func createTempExperimentFile(t *testing.T) (*redsky.Experiment, *os.File) {
	samplePatch := `spec:
  template:
    spec:
      containers:
        - name: postgres
          resources:
          limits:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"
          requests:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"`

	sampleExperiment := &redsky.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sampleExperiment",
			Namespace: "default",
		},
		Spec: redsky.ExperimentSpec{
			Parameters: []redsky.Parameter{},
			Metrics:    []redsky.Metric{},
			Patches: []redsky.PatchTemplate{
				{
					TargetRef: &corev1.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Name:       "postgres",
					},
					Patch: samplePatch,
				},
			},
			TrialTemplate: redsky.TrialTemplateSpec{},
		},
		Status: redsky.ExperimentStatus{},
	}

	tmpfile, err := ioutil.TempFile("", "experiment")
	require.NoError(t, err)

	b, err := json.Marshal(sampleExperiment)
	require.NoError(t, err)

	_, err = tmpfile.Write(b)
	require.NoError(t, err)

	return sampleExperiment, tmpfile
}

var wannabeTrial = expapi.TrialItem{
	// This is our target trial
	TrialAssignments: expapi.TrialAssignments{
		Assignments: []expapi.Assignment{
			{
				ParameterName: "cpu",
				Value:         "100",
			},
			{
				ParameterName: "memory",
				Value:         "200",
			},
		},
	},
	Number: 1234,
	Status: expapi.TrialCompleted,
}

type fakeClient struct {
	filename string
}

func (f *fakeClient) Endpoints() (config.Endpoints, error) { return nil, nil }

func (f *fakeClient) Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error) {
	return nil, nil
}

func (f *fakeClient) SystemNamespace() (string, error) { return "", nil }

func (f *fakeClient) Kubectl(ctx context.Context, arg ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/paste", f.filename)
	return cmd, nil
}
