package export_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	expapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/export"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLookupTrial(t *testing.T) {
	tmpfile := createTempExperimentFile(t)
	defer os.Remove(tmpfile.Name())

	testCases := []struct {
		desc          string
		expectedError bool
		e             *export.ExportCommand
		trialName     string
	}{
		{
			desc:          "default",
			expectedError: false,
			e:             &export.ExportCommand{RedSkyAPI: &fakeRedSkyServer{}},
			trialName:     "test-1234",
		},
		{
			desc:          "no api",
			expectedError: true,
			e:             &export.ExportCommand{},
		},
		{
			desc:          "bad trial",
			expectedError: true,
			e:             &export.ExportCommand{RedSkyAPI: &fakeRedSkyServer{}},
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

func NewFakeRedSkyServer() (*fakeRedSkyServer, error) {
	return &fakeRedSkyServer{}, nil
}

// Implement the api interface
// Not sure if this is needed, but might allow us a bit of flexibility
type fakeRedSkyServer struct{}

func (f *fakeRedSkyServer) Options(ctx context.Context) (expapi.ServerMeta, error) {
	return expapi.ServerMeta{}, nil
}

func (f *fakeRedSkyServer) GetAllExperiments(ctx context.Context, query *expapi.ExperimentListQuery) (expapi.ExperimentList, error) {
	return expapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetAllExperimentsByPage(ctx context.Context, notsure string) (expapi.ExperimentList, error) {
	return expapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetExperimentByName(ctx context.Context, name expapi.ExperimentName) (expapi.Experiment, error) {
	exp := expapi.Experiment{
		ExperimentMeta: expapi.ExperimentMeta{
			TrialsURL: "http://sometrial",
		},
		DisplayName: "postgres-example",
		Metrics: []expapi.Metric{
			{
				Name:     "cost",
				Minimize: true,
			},
			{
				Name:     "duration",
				Minimize: true,
			},
		},
		Observations: 320,
		Optimization: []expapi.Optimization{},
		Parameters: []expapi.Parameter{
			{
				Name: "cpu",
				Type: expapi.ParameterTypeInteger,
				Bounds: expapi.Bounds{
					Max: "4000",
					Min: "100",
				},
			},
			{
				Name: "memory",
				Type: expapi.ParameterTypeInteger,
				Bounds: expapi.Bounds{
					Max: "4000",
					Min: "500",
				},
			},
		},
	}

	return exp, nil
}

func (f *fakeRedSkyServer) GetExperiment(ctx context.Context, name string) (expapi.Experiment, error) {
	return expapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) CreateExperiment(ctx context.Context, name expapi.ExperimentName, exp expapi.Experiment) (expapi.Experiment, error) {
	return expapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) DeleteExperiment(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) GetAllTrials(ctx context.Context, name string, query *expapi.TrialListQuery) (expapi.TrialList, error) {
	// TODO implement some query filter magic if we really want to.
	// Otherwise, we should clean this up to mimic just the necessary output
	tl := expapi.TrialList{
		Trials: []expapi.TrialItem{
			wannabeTrial,
			// This one should not be used because number doesnt match up
			{
				TrialAssignments: expapi.TrialAssignments{
					Assignments: []expapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         "999",
						},
						{
							ParameterName: "memory",
							Value:         "999",
						},
					},
				},
				Number: 319,
				Status: expapi.TrialCompleted,
			},
			// This one should not be used because status is not completed
			{
				TrialAssignments: expapi.TrialAssignments{
					Assignments: []expapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         "999",
						},
						{
							ParameterName: "memory",
							Value:         "999",
						},
					},
				},
				Number: 320,
				Status: expapi.TrialFailed,
			},
		},
	}
	return tl, nil
}

func (f *fakeRedSkyServer) CreateTrial(ctx context.Context, name string, assignments expapi.TrialAssignments) (string, error) {
	return "", nil
}

func (f *fakeRedSkyServer) NextTrial(ctx context.Context, name string) (expapi.TrialAssignments, error) {
	return expapi.TrialAssignments{}, nil
}

func (f *fakeRedSkyServer) ReportTrial(ctx context.Context, name string, values expapi.TrialValues) error {
	return nil
}

func (f *fakeRedSkyServer) AbandonRunningTrial(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) LabelExperiment(ctx context.Context, name string, labels expapi.ExperimentLabels) error {
	return nil
}

func (f *fakeRedSkyServer) LabelTrial(ctx context.Context, name string, labels expapi.TrialLabels) error {
	return nil
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

func createTempExperimentFile(t *testing.T) *os.File {
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

	return tmpfile
}
