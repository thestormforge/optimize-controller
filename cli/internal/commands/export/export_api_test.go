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
	applicationsv2 "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
)

var wannabeTrial = experimentsv1alpha1.TrialItem{
	// This is our target trial
	TrialAssignments: experimentsv1alpha1.TrialAssignments{
		Assignments: []experimentsv1alpha1.Assignment{
			{
				ParameterName: "deployment/postgres/postgres/resources/cpu",
				Value:         api.FromInt64(100),
			},
			{
				ParameterName: "deployment/postgres/postgres/resources/memory",
				Value:         api.FromInt64(200),
			},
		},
	},
	Number: 1234,
	Status: experimentsv1alpha1.TrialCompleted,
}

// Implement the api interface
type fakeExperimentsAPI struct{}

var _ experimentsv1alpha1.API = &fakeExperimentsAPI{}

func (f *fakeExperimentsAPI) CheckEndpoint(ctx context.Context) (api.Metadata, error) {
	return api.Metadata{}, nil
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
				Name: "deployment/postgres/postgres/resources/cpu",
				Type: experimentsv1alpha1.ParameterTypeInteger,
				Bounds: &experimentsv1alpha1.Bounds{
					Max: "4000",
					Min: "100",
				},
			},
			{
				Name: "deployment/postgres/postgres/resources/memory",
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
							ParameterName: "deployment/postgres/postgres/resources/cpu",
							Value:         api.FromInt64(999),
						},
						{
							ParameterName: "deployment/postgres/postgres/resources/memory",
							Value:         api.FromInt64(999),
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
							ParameterName: "deployment/postgres/postgres/resources/cpu",
							Value:         api.FromInt64(999),
						},
						{
							ParameterName: "deployment/postgres/postgres/resources/memory",
							Value:         api.FromInt64(999),
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

// Implement the api interface
type fakeApplicationsAPI struct{}

var _ applicationsv2.API = &fakeApplicationsAPI{}

func (f fakeApplicationsAPI) CheckEndpoint(ctx context.Context) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) ListApplications(ctx context.Context, q applicationsv2.ApplicationListQuery) (applicationsv2.ApplicationList, error) {
	return applicationsv2.ApplicationList{}, nil
}

func (f fakeApplicationsAPI) ListApplicationsByPage(ctx context.Context, u string) (applicationsv2.ApplicationList, error) {
	return applicationsv2.ApplicationList{}, nil
}

func (f fakeApplicationsAPI) CreateApplication(ctx context.Context, app applicationsv2.Application) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) GetApplication(ctx context.Context, u string) (applicationsv2.Application, error) {
	return applicationsv2.Application{}, nil
}

func (f fakeApplicationsAPI) GetApplicationByName(ctx context.Context, n applicationsv2.ApplicationName) (applicationsv2.Application, error) {
	return applicationsv2.Application{}, nil
}

func (f fakeApplicationsAPI) UpsertApplication(ctx context.Context, u string, app applicationsv2.Application) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) UpsertApplicationByName(ctx context.Context, n applicationsv2.ApplicationName, app applicationsv2.Application) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) DeleteApplication(ctx context.Context, u string) error {
	return nil
}

func (f fakeApplicationsAPI) ListScenarios(ctx context.Context, u string, q applicationsv2.ScenarioListQuery) (applicationsv2.ScenarioList, error) {
	return applicationsv2.ScenarioList{}, nil
}

func (f fakeApplicationsAPI) CreateScenario(ctx context.Context, u string, scn applicationsv2.Scenario) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) GetScenario(ctx context.Context, u string) (applicationsv2.Scenario, error) {
	return applicationsv2.Scenario{}, nil
}

func (f fakeApplicationsAPI) GetScenarioByName(ctx context.Context, u string, n applicationsv2.ScenarioName) (applicationsv2.Scenario, error) {
	return applicationsv2.Scenario{}, nil
}

func (f fakeApplicationsAPI) UpsertScenario(ctx context.Context, u string, scn applicationsv2.Scenario) (applicationsv2.Scenario, error) {
	return applicationsv2.Scenario{}, nil
}

func (f fakeApplicationsAPI) UpsertScenarioByName(ctx context.Context, u string, n applicationsv2.ScenarioName, scn applicationsv2.Scenario) (applicationsv2.Scenario, error) {
	return applicationsv2.Scenario{}, nil
}

func (f fakeApplicationsAPI) DeleteScenario(ctx context.Context, u string) error {
	return nil
}

func (f fakeApplicationsAPI) PatchScenario(ctx context.Context, u string, scn applicationsv2.Scenario) error {
	return nil
}

func (f fakeApplicationsAPI) GetTemplate(ctx context.Context, u string) (applicationsv2.Template, error) {
	return applicationsv2.Template{}, nil
}

func (f fakeApplicationsAPI) UpdateTemplate(ctx context.Context, u string, s applicationsv2.Template) error {
	return nil
}

func (f fakeApplicationsAPI) PatchTemplate(ctx context.Context, u string, s applicationsv2.Template) error {
	return nil
}

func (f fakeApplicationsAPI) ListActivity(ctx context.Context, u string, q applicationsv2.ActivityFeedQuery) (applicationsv2.ActivityFeed, error) {
	return applicationsv2.ActivityFeed{}, nil
}

func (f fakeApplicationsAPI) CreateActivity(ctx context.Context, u string, a applicationsv2.Activity) error {
	return nil
}

func (f fakeApplicationsAPI) DeleteActivity(ctx context.Context, u string) error {
	return nil
}

func (f fakeApplicationsAPI) PatchApplicationActivity(ctx context.Context, u string, a applicationsv2.ActivityFailure) error {
	return nil
}

func (f fakeApplicationsAPI) SubscribeActivity(ctx context.Context, q applicationsv2.ActivityFeedQuery) (applicationsv2.Subscriber, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) CreateRecommendation(ctx context.Context, u string) (api.Metadata, error) {
	return nil, nil
}

func (f fakeApplicationsAPI) GetRecommendation(ctx context.Context, u string) (applicationsv2.Recommendation, error) {
	return applicationsv2.Recommendation{}, nil
}

func (f fakeApplicationsAPI) ListRecommendations(ctx context.Context, u string) (applicationsv2.RecommendationList, error) {
	return applicationsv2.RecommendationList{}, nil
}

func (f fakeApplicationsAPI) PatchRecommendations(ctx context.Context, u string, details applicationsv2.RecommendationList) error {
	return nil
}

func (f fakeApplicationsAPI) GetCluster(ctx context.Context, u string) (applicationsv2.Cluster, error) {
	return applicationsv2.Cluster{}, nil
}

func (f fakeApplicationsAPI) GetClusterByName(ctx context.Context, n applicationsv2.ClusterName) (applicationsv2.Cluster, error) {
	return applicationsv2.Cluster{}, nil
}

func (f fakeApplicationsAPI) ListClusters(ctx context.Context, q applicationsv2.ClusterListQuery) (applicationsv2.ClusterList, error) {
	return applicationsv2.ClusterList{}, nil
}

func (f fakeApplicationsAPI) PatchCluster(ctx context.Context, u string, c applicationsv2.ClusterTitle) error {
	return nil
}

func (f fakeApplicationsAPI) DeleteCluster(ctx context.Context, u string) error {
	return nil
}
