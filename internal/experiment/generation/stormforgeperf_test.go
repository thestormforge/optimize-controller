/*
Copyright 2021 GramLabs, Inc.

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

package generation

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
)

// This is basically the default StormForge Performance test definition, because we
// support putting test definitions inline in the `testCaseFile` field (as long as there is a newline).
const testCaseDefinition = `
definition.setTarget("http://testapp.loadtest.party");
definition.setArrivalPhases([{duration: 5 * 60, rate: 1.0}]);
definition.setTestOptions({cluster: {sizing: "preflight"}});
definition.session("hello world", function(session) {
  session.get("/", { tag: "root" });
});
`

func TestStormForgePerformanceSource_TestCaseFile(t *testing.T) {
	cases := []struct {
		name     string // Double dip on the Go test name and the scenario name
		scenario optimizeappsv1alpha1.StormForgeScenario
		expected string
	}{
		{
			name: "no-test-case-file",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase: "my-test",
			},
		},
		{
			name: "relative-path",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase:     "my-test",
				TestCaseFile: "some/path/to/some/file.js",
			},
			expected: "/forge-init.d/my-test.js",
		},
		{
			name: "url",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase:     "my-test",
				TestCaseFile: "https://example.invalid/test-cases/file.js",
			},
			expected: "/forge-init.d/my-test.js",
		},
		{
			name: "data-url",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase:     "my-test",
				TestCaseFile: "data:," + url.PathEscape(testCaseDefinition),
			},
			expected: "/forge-init.d/my-test.js",
		},
		{
			name: "inline-javascript",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase:     "my-test",
				TestCaseFile: testCaseDefinition,
			},
			expected: "/forge-init.d/my-test.js",
		},
		{
			name: "explicit-org",
			scenario: optimizeappsv1alpha1.StormForgeScenario{
				TestCase:     "walrus-denies-contract/my-test",
				TestCaseFile: "coo/coo/ca/choo/file.js",
			},
			expected: "/forge-init.d/my-test.js",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &StormForgePerformanceSource{
				Scenario: &optimizeappsv1alpha1.Scenario{
					Name:       c.name,
					StormForge: &c.scenario,
				},
			}
			assert.Equal(t, c.expected, src.testCaseFile())
		})
	}
}
