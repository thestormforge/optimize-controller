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

package patch_test

import (
	"context"

	experimentsapi "github.com/redskyops/redskyops-go/pkg/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-go/pkg/redskyapi/experiments/v1alpha1/numstr"
)

var wannabeTrial = experimentsapi.TrialItem{
	// This is our target trial
	TrialAssignments: experimentsapi.TrialAssignments{
		Assignments: []experimentsapi.Assignment{
			{
				ParameterName: "cpu",
				Value:         numstr.FromInt64(100),
			},
			{
				ParameterName: "memory",
				Value:         numstr.FromInt64(200),
			},
		},
	},
	Number: 1234,
	Status: experimentsapi.TrialCompleted,
}

// Implement the api interface
type fakeRedSkyServer struct{}

func (f *fakeRedSkyServer) Options(ctx context.Context) (experimentsapi.ServerMeta, error) {
	return experimentsapi.ServerMeta{}, nil
}

func (f *fakeRedSkyServer) GetAllExperiments(ctx context.Context, query *experimentsapi.ExperimentListQuery) (experimentsapi.ExperimentList, error) {
	return experimentsapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetAllExperimentsByPage(ctx context.Context, notsure string) (experimentsapi.ExperimentList, error) {
	return experimentsapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetExperimentByName(ctx context.Context, name experimentsapi.ExperimentName) (experimentsapi.Experiment, error) {
	exp := experimentsapi.Experiment{
		ExperimentMeta: experimentsapi.ExperimentMeta{
			TrialsURL: "http://sometrial",
		},
		DisplayName: "postgres-example",
		Metrics: []experimentsapi.Metric{
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
		Optimization: []experimentsapi.Optimization{},
		Parameters: []experimentsapi.Parameter{
			{
				Name: "cpu",
				Type: experimentsapi.ParameterTypeInteger,
				Bounds: &experimentsapi.Bounds{
					Max: "4000",
					Min: "100",
				},
			},
			{
				Name: "memory",
				Type: experimentsapi.ParameterTypeInteger,
				Bounds: &experimentsapi.Bounds{
					Max: "4000",
					Min: "500",
				},
			},
		},
	}

	return exp, nil
}

func (f *fakeRedSkyServer) GetExperiment(ctx context.Context, name string) (experimentsapi.Experiment, error) {
	return experimentsapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) CreateExperiment(ctx context.Context, name experimentsapi.ExperimentName, exp experimentsapi.Experiment) (experimentsapi.Experiment, error) {
	return experimentsapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) DeleteExperiment(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) GetAllTrials(ctx context.Context, name string, query *experimentsapi.TrialListQuery) (experimentsapi.TrialList, error) {
	// TODO implement some query filter magic if we really want to.
	// Otherwise, we should clean this up to mimic just the necessary output
	tl := experimentsapi.TrialList{
		Trials: []experimentsapi.TrialItem{
			wannabeTrial,
			// This one should not be used because number doesnt match up
			{
				TrialAssignments: experimentsapi.TrialAssignments{
					Assignments: []experimentsapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         numstr.FromInt64(999),
						},
						{
							ParameterName: "memory",
							Value:         numstr.FromInt64(999),
						},
					},
				},
				Number: 319,
				Status: experimentsapi.TrialCompleted,
			},
			// This one should not be used because status is not completed
			{
				TrialAssignments: experimentsapi.TrialAssignments{
					Assignments: []experimentsapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         numstr.FromInt64(999),
						},
						{
							ParameterName: "memory",
							Value:         numstr.FromInt64(999),
						},
					},
				},
				Number: 320,
				Status: experimentsapi.TrialFailed,
			},
		},
	}
	return tl, nil
}

func (f *fakeRedSkyServer) CreateTrial(ctx context.Context, name string, assignments experimentsapi.TrialAssignments) (experimentsapi.TrialAssignments, error) {
	return experimentsapi.TrialAssignments{}, nil
}

func (f *fakeRedSkyServer) NextTrial(ctx context.Context, name string) (experimentsapi.TrialAssignments, error) {
	return experimentsapi.TrialAssignments{}, nil
}

func (f *fakeRedSkyServer) ReportTrial(ctx context.Context, name string, values experimentsapi.TrialValues) error {
	return nil
}

func (f *fakeRedSkyServer) AbandonRunningTrial(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) LabelExperiment(ctx context.Context, name string, labels experimentsapi.ExperimentLabels) error {
	return nil
}

func (f *fakeRedSkyServer) LabelTrial(ctx context.Context, name string, labels experimentsapi.TrialLabels) error {
	return nil
}
