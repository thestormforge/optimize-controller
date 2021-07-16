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

package experiment

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
)

func TestScanResources(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			r := &Runner{}
			ad := applications.Application{}
			err := json.Unmarshal(appData, &ad)
			assert.NoError(t, err)

			res, err := r.scanResources(ad)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
		})
	}
}

func TestScanParameters(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			r := &Runner{}
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := r.scanParameters(sd)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(res))
			assert.NotNil(t, res[0].ContainerResources)
			assert.NotNil(t, res[1].Replicas)
		})
	}
}

func TestScanObjectives(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			r := &Runner{}
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := r.scanObjectives(sd)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(res))
		})
	}
}

func TestScanScenarios(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			r := &Runner{}
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, _, err := r.scanScenarios(sd)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
			assert.NotNil(t, res[0].StormForger)
		})
	}
}

func TestScan(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			r := &Runner{}

			ad := applications.Application{}
			err := json.Unmarshal(appData, &ad)
			assert.NoError(t, err)

			sd := applications.Scenario{}
			err = json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := r.scan(ad, sd)
			assert.NoError(t, err)
			assert.NotNil(t, res.Name)
			// Uncomment when we figure out what to do
			// assert.NotNil(t, res.Namespace)
			assert.NotNil(t, res.Resources)
			assert.NotNil(t, res.Parameters)
			assert.NotNil(t, res.Scenarios)
			assert.NotNil(t, res.Objectives)
			// TODO do we need ingress?
			// assert.NotNil(t,  res.Ingress)
		})
	}
}

var scenarioData = []byte(`
{
  "_metadata": {
    "Last-Modified": "2021-07-16T16:52:07.977747Z",
    "Link": [
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V/scenarios/01FAR3Q0N9M1SPM4Z3V07TE7QD>; rel=\"self\"",
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V>; rel=\"up\"",
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V/scenarios/01FAR3Q0N9M1SPM4Z3V07TE7QD/template>; rel=\"https://stormforge.io/rel/template\"",
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V/scenarios/01FAR3Q0N9M1SPM4Z3V07TE7QD/experiments>; rel=\"https://stormforge.io/rel/experiments\""
    ],
    "Title": "Awesome Scenario 1"
  },
  "name": "01FAR3Q0N9M1SPM4Z3V07TE7QD",
  "title": "Awesome Scenario 1",
  "stormforger": {
    "testCase": "myorg/large-load-test"
  },
  "objective": [
    {
      "name": "cost"
    },
    {
      "name": "p95-latency"
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

var appData = []byte(`
{
  "_metadata": {
    "Last-Modified": "2021-07-16T16:52:06.653927Z",
    "Link": [
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V>; rel=\"self\"",
      "</v2/applications/01FAR3PZBXA1RCXSH7MTA7T30V/scenarios/>; rel=\"https://stormforge.io/rel/scenarios\""
    ],
    "Title": "Awesome Application 1"
  },
  "name": "01FAR3PZBXA1RCXSH7MTA7T30V",
  "createdAt": "2021-07-16T16:52:06.653927Z",
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
