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
	"context"
	"fmt"

	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1/numstr"
)

var wannabeTrial = experimentsv1alpha1.TrialItem{
	// This is our target trial
	TrialAssignments: experimentsv1alpha1.TrialAssignments{
		Assignments: []experimentsv1alpha1.Assignment{
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
	Status: experimentsv1alpha1.TrialCompleted,
}

// Implement the api interface
type fakeExperimentsAPI struct{}

var _ experimentsv1alpha1.API = &fakeExperimentsAPI{}

func (f *fakeExperimentsAPI) Options(ctx context.Context) (experimentsv1alpha1.Server, error) {
	return experimentsv1alpha1.Server{}, nil
}

func (f *fakeExperimentsAPI) GetAllExperiments(ctx context.Context, query experimentsv1alpha1.ExperimentListQuery) (experimentsv1alpha1.ExperimentList, error) {
	return experimentsv1alpha1.ExperimentList{}, nil
}

func (f *fakeExperimentsAPI) GetAllExperimentsByPage(ctx context.Context, notsure string) (experimentsv1alpha1.ExperimentList, error) {
	return experimentsv1alpha1.ExperimentList{}, nil
}

func (f *fakeExperimentsAPI) GetExperimentByName(ctx context.Context, name experimentsv1alpha1.ExperimentName) (experimentsv1alpha1.Experiment, error) {
	exp := experimentsv1alpha1.Experiment{
		Metadata: api.Metadata{
			"Link": {fmt.Sprintf("<http://sometrial>;rel=%s", api.RelationTrials)},
		},
		DisplayName: "postgres-example",
		Labels: map[string]string{
			// NOTE: If the application label is not present, we will accept any application
			"application": "sampleApplication",
			"scenario":    "how-do-you-make-a-tissue-dance",
		},
		Metrics: []experimentsv1alpha1.Metric{
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
		Optimization: []experimentsv1alpha1.Optimization{},
		Parameters: []experimentsv1alpha1.Parameter{
			{
				Name: "cpu",
				Type: experimentsv1alpha1.ParameterTypeInteger,
				Bounds: &experimentsv1alpha1.Bounds{
					Max: "4000",
					Min: "100",
				},
			},
			{
				Name: "memory",
				Type: experimentsv1alpha1.ParameterTypeInteger,
				Bounds: &experimentsv1alpha1.Bounds{
					Max: "4000",
					Min: "500",
				},
			},
		},
	}

	return exp, nil
}

func (f *fakeExperimentsAPI) GetExperiment(ctx context.Context, name string) (experimentsv1alpha1.Experiment, error) {
	return experimentsv1alpha1.Experiment{}, nil
}

func (f *fakeExperimentsAPI) CreateExperimentByName(ctx context.Context, name experimentsv1alpha1.ExperimentName, experiment experimentsv1alpha1.Experiment) (experimentsv1alpha1.Experiment, error) {
	return experimentsv1alpha1.Experiment{}, nil
}

func (f *fakeExperimentsAPI) CreateExperiment(ctx context.Context, s string, experiment experimentsv1alpha1.Experiment) (experimentsv1alpha1.Experiment, error) {
	return experimentsv1alpha1.Experiment{}, nil
}

func (f *fakeExperimentsAPI) DeleteExperiment(ctx context.Context, name string) error {
	return nil
}

func (f *fakeExperimentsAPI) GetAllTrials(ctx context.Context, name string, query experimentsv1alpha1.TrialListQuery) (experimentsv1alpha1.TrialList, error) {
	// TODO implement some query filter magic if we really want to.
	// Otherwise, we should clean this up to mimic just the necessary output
	tl := experimentsv1alpha1.TrialList{
		Trials: []experimentsv1alpha1.TrialItem{
			wannabeTrial,
			// This one should not be used because number doesnt match up
			{
				TrialAssignments: experimentsv1alpha1.TrialAssignments{
					Assignments: []experimentsv1alpha1.Assignment{
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
				Status: experimentsv1alpha1.TrialCompleted,
			},
			// This one should not be used because status is not completed
			{
				TrialAssignments: experimentsv1alpha1.TrialAssignments{
					Assignments: []experimentsv1alpha1.Assignment{
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
				Status: experimentsv1alpha1.TrialFailed,
			},
		},
	}
	return tl, nil
}

func (f *fakeExperimentsAPI) CreateTrial(ctx context.Context, name string, assignments experimentsv1alpha1.TrialAssignments) (experimentsv1alpha1.TrialAssignments, error) {
	return experimentsv1alpha1.TrialAssignments{}, nil
}

func (f *fakeExperimentsAPI) NextTrial(ctx context.Context, name string) (experimentsv1alpha1.TrialAssignments, error) {
	return experimentsv1alpha1.TrialAssignments{}, nil
}

func (f *fakeExperimentsAPI) ReportTrial(ctx context.Context, name string, values experimentsv1alpha1.TrialValues) error {
	return nil
}

func (f *fakeExperimentsAPI) AbandonRunningTrial(ctx context.Context, name string) error {
	return nil
}

func (f *fakeExperimentsAPI) LabelExperiment(ctx context.Context, name string, labels experimentsv1alpha1.ExperimentLabels) error {
	return nil
}

func (f *fakeExperimentsAPI) LabelTrial(ctx context.Context, name string, labels experimentsv1alpha1.TrialLabels) error {
	return nil
}
