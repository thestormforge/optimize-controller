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

package application

import (
	"testing"

	"github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExperimentName(t *testing.T) {
	cases := []struct {
		name string
		app  v1alpha1.Application
	}{
		{
			name: "application-testcase-objective",
			app: v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "application"},
				Scenarios: []v1alpha1.Scenario{
					{
						// NOTE: This relies on the behavior of `Application.Default()`
						StormForger: &v1alpha1.StormForgerScenario{TestCase: "testCase"},
					},
				},
				Objectives: []v1alpha1.Objective{
					{
						Name: "objective",
					},
				},
			},
		},

		{
			name: "a-s-o",
			app: v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "a"},
				Scenarios: []v1alpha1.Scenario{
					{
						Name:        "s",
						StormForger: &v1alpha1.StormForgerScenario{TestCase: "testCase"},
					},
				},
				Objectives: []v1alpha1.Objective{
					{
						Name:     "o",
						Requests: &v1alpha1.RequestsObjective{},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.name, ExperimentName(&c.app))
		})
	}
}

func TestFilterByExperimentName(t *testing.T) {
	cases := []struct {
		experiment     string
		application    string
		scenarioNames  []string
		objectiveNames []string
		err            string
	}{
		{
			experiment:     "simple-default-one",
			application:    "simple",
			scenarioNames:  []string{"test", "default"},
			objectiveNames: []string{"one", "two"},
		},

		{
			experiment:     "a-a-s-s-o2-o1",
			application:    "a-a",
			scenarioNames:  []string{"s-s-s", "s-s"},
			objectiveNames: []string{"o1", "o2", "o3"},
		},

		{
			experiment:     "a-s-s-o",
			application:    "a",
			scenarioNames:  []string{"s", "s-s"},
			objectiveNames: []string{"s-o", "o"},
			err:            "ambiguous name 'a-s-s-o'",
		},
		{
			experiment:     "a-s-s-o",
			application:    "a",
			scenarioNames:  []string{"s", "s-s"},
			objectiveNames: []string{"s-o", "x"},
		},

		{
			experiment:     "case-myscenario-test2-test1",
			application:    "case",
			scenarioNames:  []string{"MyScenario"},
			objectiveNames: []string{"Test_2", "Test_1"},
		},

		{
			experiment:     "app-blackfriday-latency",
			application:    "app",
			scenarioNames:  []string{"cybermonday", "blackfriday"},
			objectiveNames: []string{"cost", "throughput"},
			err:            "invalid name 'app-blackfriday-latency', could not find cost, throughput",
		},
	}
	for _, c := range cases {
		t.Run(c.experiment, func(t *testing.T) {
			// Build an application with the all the necessary parts
			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: c.application},
			}
			for _, n := range c.scenarioNames {
				app.Scenarios = append(app.Scenarios, v1alpha1.Scenario{Name: n})
			}
			for _, n := range c.objectiveNames {
				app.Objectives = append(app.Objectives, v1alpha1.Objective{Name: n})
			}

			// Filter the experiment using the name and verify it produces the same name
			err := FilterByExperimentName(app, c.experiment)
			if c.err != "" {
				assert.EqualError(t, err, c.err)
			} else if assert.NoError(t, err) {
				assert.Equal(t, c.experiment, ExperimentName(app))
			}
		})
	}
}
