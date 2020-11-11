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

func TestFilterByExperimentName(t *testing.T) {
	cases := []struct {
		experiment     string
		application    string
		scenario       string
		objectives     []string
		scenarioNames  []string
		objectiveNames []string
	}{
		{
			experiment:     "simple-default-one",
			application:    "simple",
			scenario:       "default",
			objectives:     []string{"one"},
			scenarioNames:  []string{"test", "default"},
			objectiveNames: []string{"one", "two"},
		},

		{
			experiment:     "a-a-s-s-o2-o1",
			application:    "a-a",
			scenario:       "s-s",
			objectives:     []string{"o2", "o1"},
			scenarioNames:  []string{"s-s-s", "s-s"},
			objectiveNames: []string{"o1", "o2", "o3"},
		},

		{
			experiment:     "a-s-s-o",
			application:    "a",
			scenario:       "s",
			objectives:     []string{"s-o"},
			scenarioNames:  []string{"s", "s-s"},
			objectiveNames: []string{"s-o", "x"},
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
			if assert.NoError(t, err) {
				assert.Equal(t, c.experiment, ExperimentName(app))
			}
		})
	}
}
