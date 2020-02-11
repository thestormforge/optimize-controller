/*
Copyright 2019 GramLabs, Inc.

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

package fake

import (
	"context"

	"github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
)

var _ v1alpha1.API = &FakeAPI{}

type FakeAPI struct {
	experiments map[string]v1alpha1.Experiment
}

func NewFakeAPI() v1alpha1.API {
	return &FakeAPI{
		experiments: make(map[string]v1alpha1.Experiment),
	}
}

func (f *FakeAPI) key(n v1alpha1.ExperimentName) string {
	return "http://example/com/api/experiments/" + n.Name()
}

func (f *FakeAPI) Options(context.Context) (v1alpha1.ServerMeta, error) {
	return v1alpha1.ServerMeta{}, nil
}

func (f *FakeAPI) GetAllExperiments(ctx context.Context, q *v1alpha1.ExperimentListQuery) (v1alpha1.ExperimentList, error) {
	return f.GetAllExperimentsByPage(ctx, q.Encode())
}

func (f *FakeAPI) GetAllExperimentsByPage(ctx context.Context, uri string) (v1alpha1.ExperimentList, error) {
	l := v1alpha1.ExperimentList{}
	for i := range f.experiments {
		l.Experiments = append(l.Experiments, v1alpha1.ExperimentItem{
			Experiment: f.experiments[i],
		})
	}
	return l, nil
}

func (f *FakeAPI) GetExperimentByName(ctx context.Context, n v1alpha1.ExperimentName) (v1alpha1.Experiment, error) {
	return f.GetExperiment(ctx, f.key(n))
}

func (f *FakeAPI) GetExperiment(ctx context.Context, uri string) (v1alpha1.Experiment, error) {
	if e, ok := f.experiments[uri]; ok {
		return e, nil
	} else {
		return v1alpha1.Experiment{}, &v1alpha1.Error{Type: v1alpha1.ErrExperimentNotFound}
	}
}

func (f *FakeAPI) CreateExperiment(ctx context.Context, n v1alpha1.ExperimentName, e v1alpha1.Experiment) (v1alpha1.Experiment, error) {
	e.Self = f.key(n)
	f.experiments[e.Self] = e
	return e, nil
}

func (f *FakeAPI) DeleteExperiment(ctx context.Context, uri string) error {
	delete(f.experiments, uri)
	return nil
}

func (f *FakeAPI) GetAllTrials(ctx context.Context, uri string, q *v1alpha1.TrialListQuery) (v1alpha1.TrialList, error) {
	panic("implement me")
}

func (f *FakeAPI) CreateTrial(ctx context.Context, uri string, a v1alpha1.TrialAssignments) (string, error) {
	panic("implement me")
}

func (f *FakeAPI) NextTrial(ctx context.Context, uri string) (v1alpha1.TrialAssignments, error) {
	panic("implement me")
}

func (f *FakeAPI) ReportTrial(ctx context.Context, uri string, v v1alpha1.TrialValues) error {
	panic("implement me")
}

func (h *FakeAPI) AbandonRunningTrial(ctx context.Context, u string) error {
	panic("implement me")
}
