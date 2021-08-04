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

package controllers

import (
	"context"
	"encoding/json"

	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
)

var _ applications.API = &fakeAPI{}

type fakeAPI struct {
	template         applications.Template
	templateUpdateCh chan struct{}
	failureCh        chan (applications.ActivityFailure)
}

func (f *fakeAPI) CheckEndpoint(ctx context.Context) (api.Metadata, error) { return nil, nil }

func (f *fakeAPI) ListApplications(ctx context.Context, q applications.ApplicationListQuery) (applications.ApplicationList, error) {
	return applications.ApplicationList{}, nil
}
func (f *fakeAPI) ListApplicationsByPage(ctx context.Context, u string) (applications.ApplicationList, error) {
	return applications.ApplicationList{}, nil
}
func (f *fakeAPI) CreateApplication(ctx context.Context, app applications.Application) (api.Metadata, error) {
	return nil, nil
}
func (f *fakeAPI) GetApplication(ctx context.Context, u string) (applications.Application, error) {
	applicationBytes := []byte(`{
  "_metadata": {
    "Last-Modified": "2021-07-21T17:33:26.29947Z",
    "Link": [
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9>; rel=\"self\"",
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/>; rel=\"https://stormforge.io/rel/scenarios\""
    ],
    "Title": "Awesome Application 1"
  },
  "name": "01FB5227WVRE4RFFDJ4TCZY8Z9",
  "createdAt": "2021-07-21T17:33:26.29947Z",
  "title": "Awesome Application 1",
  "resources": [
    {
      "kubernetes": {
        "selector": "app.kubernetes.io/name=app-1",
        "namespace": "engineering"
      }
    }
  ],
  "scenarioCount": 1
}`)

	app := applications.Application{}
	err := json.Unmarshal(applicationBytes, &app)
	return app, err
}
func (f *fakeAPI) GetApplicationByName(ctx context.Context, n applications.ApplicationName) (applications.Application, error) {
	return applications.Application{}, nil
}
func (f *fakeAPI) UpsertApplication(ctx context.Context, u string, app applications.Application) (api.Metadata, error) {
	return nil, nil
}
func (f *fakeAPI) UpsertApplicationByName(ctx context.Context, n applications.ApplicationName, app applications.Application) (api.Metadata, error) {
	return nil, nil
}
func (f *fakeAPI) DeleteApplication(ctx context.Context, u string) error { return nil }
func (f *fakeAPI) ListScenarios(ctx context.Context, u string, q applications.ScenarioListQuery) (applications.ScenarioList, error) {
	return applications.ScenarioList{}, nil
}
func (f *fakeAPI) CreateScenario(ctx context.Context, u string, scn applications.Scenario) (api.Metadata, error) {
	return nil, nil
}

func (f *fakeAPI) GetScenario(ctx context.Context, u string) (applications.Scenario, error) {
	scenarioBytes := []byte(`{
  "_metadata": {
    "Last-Modified": "2021-07-21T17:33:28.349473Z",
    "Link": [
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/01FB5229WXA976GSYX1BSJNM8G>; rel=\"self\"",
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9>; rel=\"up\"",
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/01FB5229WXA976GSYX1BSJNM8G/template>; rel=\"https://stormforge.io/rel/template\"",
      "</v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/01FB5229WXA976GSYX1BSJNM8G/experiments>; rel=\"https://stormforge.io/rel/experiments\""
    ],
    "Title": "Awesome Scenario 1"
  },
  "name": "01FB5229WXA976GSYX1BSJNM8G",
  "title": "Awesome Scenario 1",
  "stormforger": {
    "testCase": "myorg/large-load-test"
  },
  "objective": [
    {
      "name": "cost"
    },
    {
      "name": "p95"
    }
  ],
  "configuration": [
    {
      "containerResources": {
        "selector": "component in (api,db,worker)"
      }
    },
    {
      "replicas": {
        "selector": "component in (api,worker)"
      }
    }
  ]
}`)

	scenario := applications.Scenario{}
	err := api.UnmarshalJSON(scenarioBytes, &scenario)

	return scenario, err
}

func (f *fakeAPI) UpsertScenario(ctx context.Context, u string, scn applications.Scenario) (applications.Scenario, error) {
	return applications.Scenario{}, nil
}
func (f *fakeAPI) DeleteScenario(ctx context.Context, u string) error { return nil }
func (f *fakeAPI) PatchScenario(ctx context.Context, u string, scn applications.Scenario) error {
	return nil
}
func (f *fakeAPI) GetTemplate(ctx context.Context, u string) (applications.Template, error) {
	return f.template, nil
}
func (f *fakeAPI) UpdateTemplate(ctx context.Context, u string, s applications.Template) error {
	f.template = s
	close(f.templateUpdateCh)
	return nil
}
func (f *fakeAPI) PatchTemplate(ctx context.Context, u string, s applications.Template) error {
	return nil
}
func (f *fakeAPI) ListActivity(ctx context.Context, u string, q applications.ActivityFeedQuery) (applications.ActivityFeed, error) {
	return applications.ActivityFeed{}, nil
}
func (f *fakeAPI) CreateActivity(ctx context.Context, u string, a applications.Activity) error {
	return nil
}
func (f *fakeAPI) DeleteActivity(ctx context.Context, u string) error { return nil }
func (f *fakeAPI) GetApplicationActivity(ctx context.Context, u string) (applications.Activity, error) {
	return applications.Activity{}, nil
}
func (f *fakeAPI) UpdateApplicationActivity(ctx context.Context, u string, a applications.Activity) error {
	return nil
}

func (f *fakeAPI) PatchApplicationActivity(ctx context.Context, u string, a applications.ActivityFailure) error {
	f.failureCh <- a
	return nil
}

func (f *fakeAPI) SubscribeActivity(ctx context.Context, q applications.ActivityFeedQuery) (applications.Subscriber, error) {
	v := ctx.Value("tag")
	switch v {
	case applications.TagScan:
		return fakeScanSubscriber{}, nil
	case applications.TagRun:
		// Prepopulate f.template so when we fetch template we have expected data
		return fakeRunSubscriber{}, nil
	default:
		return fakeScanSubscriber{}, nil
	}
}

var _ applications.Subscriber = fakeScanSubscriber{}

type fakeScanSubscriber struct{}

func (f fakeScanSubscriber) Subscribe(ctx context.Context, ch chan<- applications.ActivityItem) {
	sampleActivity := applications.ActivityItem{
		ID:    "01FB5DMRW14JNNC7R9EWJEB5FM",
		URL:   "http://localhost:8113/v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/01FB5229WXA976GSYX1BSJNM8G",
		Title: "Sample scan task",
		Tags:  []string{applications.TagScan},
	}

	ch <- sampleActivity
	close(ch)
}

var _ applications.Subscriber = fakeRunSubscriber{}

type fakeRunSubscriber struct{}

func (f fakeRunSubscriber) Subscribe(ctx context.Context, ch chan<- applications.ActivityItem) {
	sampleActivity := applications.ActivityItem{
		ID:    "01FB5DMRW14JNNC7R9EWJEB5FM",
		URL:   "http://localhost:8113/v2/applications/01FB5227WVRE4RFFDJ4TCZY8Z9/scenarios/01FB5229WXA976GSYX1BSJNM8G",
		Title: "Sample scan task",
		Tags:  []string{applications.TagRun},
	}

	ch <- sampleActivity
	close(ch)
}
