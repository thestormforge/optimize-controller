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

package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestClusterExperimentToAPITemplate(t *testing.T) {
	one := intstr.FromInt(200)
	two := intstr.FromInt(2000)
	three := intstr.FromInt(20000)
	four := intstr.FromString("four")
	pFalse := false

	testCases := []struct {
		desc            string
		expectedParams  int
		expectedMetrics int
		exp             *optimizev1beta2.Experiment
	}{
		{
			desc:            "default",
			expectedParams:  4,
			expectedMetrics: 3,
			exp: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Parameters: []optimizev1beta2.Parameter{
						{Name: "one", Min: 111, Max: 222, Baseline: &one},
						{Name: "two", Min: 1111, Max: 2222, Baseline: &two},
						{Name: "three", Min: 11111, Max: 22222, Baseline: &three},
						{Name: "four", Values: []string{"one", "two", "three", "four"}, Baseline: &four},
						{Name: "test_case", Min: 1, Max: 1},
					},
					Metrics: []optimizev1beta2.Metric{
						{Name: "one", Minimize: true},
						{Name: "two", Minimize: false},
						{Name: "three", Optimize: &pFalse},
					},
				},
			},
		},

		{
			// TODO
			// I'm expecting this to fail ( in that we have a parameter without a baseline )
			// but it doesnt, should we catch this here or later?
			desc:            "one param missing baseline",
			expectedParams:  4,
			expectedMetrics: 3,
			exp: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Parameters: []optimizev1beta2.Parameter{
						{Name: "one", Min: 111, Max: 222, Baseline: &one},
						{Name: "two", Min: 1111, Max: 2222, Baseline: &two},
						{Name: "three", Min: 11111, Max: 22222, Baseline: &three},
						{Name: "four", Values: []string{"one", "two", "three", "four"}},
						{Name: "test_case", Min: 1, Max: 1},
					},
					Metrics: []optimizev1beta2.Metric{
						{Name: "one", Minimize: true},
						{Name: "two", Minimize: false},
						{Name: "three", Minimize: true},
					},
				},
			},
		},

		{
			desc:            "no valid params",
			expectedParams:  0,
			expectedMetrics: 1,
			exp: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Parameters: []optimizev1beta2.Parameter{
						{Name: "test_case", Min: 1, Max: 1},
					},
					Metrics: []optimizev1beta2.Metric{
						{Name: "one"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			template, err := ClusterExperimentToAPITemplate(tc.exp)
			assert.NoError(t, err)
			assert.NotNil(t, template.Parameters)
			assert.NotNil(t, template.Metrics)

			// test_case is silently filters/dropped because min==max
			assert.Equal(t, tc.expectedParams, len(template.Parameters))
			assert.Equal(t, tc.expectedMetrics, len(template.Metrics))

			for _, templateParam := range template.Parameters {
				for _, expParam := range tc.exp.Spec.Parameters {
					if templateParam.Name != expParam.Name {
						continue
					}

					assert.Equal(t, expParam.Baseline.String(), templateParam.Baseline.String())
				}
			}

		})
	}
}

func TestAPITemplateToClusterExperiment(t *testing.T) {
	templateOne := api.FromInt64(500)
	templateTwo := api.FromInt64(2000)
	templateThree := api.FromInt64(20000)
	expOne := intstr.FromString("500")
	expTwo := intstr.FromString("2000")
	expThree := intstr.FromString("20000")
	pTrue := true

	testCases := []struct {
		desc       string
		experiment *optimizev1beta2.Experiment
		template   *applications.Template
		expected   *optimizev1beta2.Experiment
	}{
		{
			desc: "params",
			experiment: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{},
			},
			template: &applications.Template{
				Parameters: []applications.TemplateParameter{
					{
						Name: "one",
						Bounds: &applications.TemplateParameterBounds{
							Min: json.Number("1"),
							Max: json.Number("1000"),
						},
						Type:     "int",
						Baseline: &templateOne,
					},
					{
						Name: "two",
						Bounds: &applications.TemplateParameterBounds{
							Min: json.Number("1111"),
							Max: json.Number("2222"),
						},
						Type:     "int",
						Baseline: &templateTwo,
					},
					{
						Name: "three",
						Bounds: &applications.TemplateParameterBounds{
							Min: json.Number("11111"),
							Max: json.Number("22222"),
						},
						Type:     "int",
						Baseline: &templateThree,
					},
					// TODO this might be an edge case we need to handle
					// {Name: "test_case", Min: 1, Max: 1},
				},
			},
			expected: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Parameters: []optimizev1beta2.Parameter{
						{Name: "one", Min: 1, Max: 1000, Baseline: &expOne},
						{Name: "two", Min: 1111, Max: 2222, Baseline: &expTwo},
						{Name: "three", Min: 11111, Max: 22222, Baseline: &expThree},
					},
				},
			},
		},
		{
			desc: "metrics",
			experiment: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Metrics: []optimizev1beta2.Metric{
						{Name: "one", Minimize: true},
						{Name: "two", Minimize: false},
						{Name: "three", Minimize: true},
					},
				},
			},
			template: &applications.Template{
				Metrics: []applications.TemplateMetric{
					{
						Name:     "one",
						Minimize: true,
						Bounds: &applications.TemplateMetricBounds{
							Min: 1,
							Max: 10,
						},
					},
					{
						Name:     "two",
						Minimize: false,
						Bounds: &applications.TemplateMetricBounds{
							Min: 2,
							Max: 5,
						},
					},
					{
						Name:     "three",
						Minimize: false,
						Optimize: &pTrue,
					},
				},
			},
			expected: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Parameters: []optimizev1beta2.Parameter{},
					Metrics: []optimizev1beta2.Metric{
						{Name: "one", Minimize: true, Min: resource.NewQuantity(1, resource.DecimalSI), Max: resource.NewQuantity(10, resource.DecimalSI)},
						{Name: "two", Minimize: false, Min: resource.NewQuantity(2, resource.DecimalSI), Max: resource.NewQuantity(5, resource.DecimalSI)},
						{Name: "three", Minimize: false, Optimize: &pTrue},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			err := APITemplateToClusterExperiment(tc.experiment, tc.template)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, tc.experiment)
		})
	}
}

func TestAPIResources(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			ad := applications.Application{}
			err := json.Unmarshal(appData, &ad)
			assert.NoError(t, err)

			res, err := apiResources(ad)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
		})
	}
}

func TestAPIParameters(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := apiParameters(sd)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(res))
			assert.NotNil(t, res[0].ContainerResources)
			assert.NotNil(t, res[1].Replicas)
		})
	}
}

func TestAPIObjectives(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := apiObjectives(sd)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(res))
			assert.Equal(t, 2, len(res[0].Goals))
		})
	}
}

func TestAPIScenarios(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			sd := applications.Scenario{}
			err := json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := apiScenarios(sd)
			assert.NoError(t, err)
			require.Equal(t, 1, len(res))
			assert.NotNil(t, res[0].StormForge)
		})
	}
}

func TestAPIApplicationToClusterApplication(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			ad := applications.Application{}
			err := json.Unmarshal(appData, &ad)
			assert.NoError(t, err)

			sd := applications.Scenario{}
			err = json.Unmarshal(scenarioData, &sd)
			assert.NoError(t, err)

			res, err := APIApplicationToClusterApplication(ad, sd)
			assert.NoError(t, err)
			assert.NotNil(t, res.Name)
			// Uncomment when we figure out what to do
			// assert.NotNil(t, res.Namespace)
			assert.NotNil(t, res.Resources)
			assert.NotNil(t, res.Configuration)
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
  "stormforgePerf": {
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
