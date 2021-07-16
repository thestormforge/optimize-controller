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

	"github.com/stretchr/testify/assert"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExperimentName(t *testing.T) {
	cases := []struct {
		applicationName string
		scenario        string
		objective       string
		expected        string
	}{
		{
			applicationName: "this-is-a-really-really-long-name-that-is-exactly-63-characters",
			scenario:        "xxx",
			objective:       "xxx",
			expected:        "this-is-a-really-really-long-name-that-is-exactly-63-c-018f4d7f",
		},
		{
			applicationName: "scenario-and-objective-are-normalized",
			scenario:        "FOO?", // These are not a valid scenario/objective names
			objective:       "BAR!",
			expected:        "scenario-and-objective-are-normalized-8843d7f9",
		},
		{
			applicationName: "scenario-and-objective-are-normalized",
			scenario:        "foo",
			objective:       "bar",
			expected:        "scenario-and-objective-are-normalized-8843d7f9", // This matches the hash of the invalid names
		},
	}
	for _, c := range cases {
		t.Run(c.applicationName, func(t *testing.T) {
			actual := ExperimentName(&optimizeappsv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name: c.applicationName,
				},
			}, c.scenario, c.objective)
			assert.Equal(t, c.expected, actual)
			assert.LessOrEqual(t, len(actual), 63)
		})
	}
}
