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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestScenario_Default(t *testing.T) {
	cases := []struct {
		desc     string
		scenario Scenario
		expected Scenario
	}{
		{
			desc: "empty",
			expected: Scenario{
				Name: "default",
			},
		},
		{
			desc: "stormforger testcase",
			scenario: Scenario{
				StormForger: &StormForgerScenario{
					TestCase: "testCase",
				},
			},
			expected: Scenario{
				Name: "testcase",
				StormForger: &StormForgerScenario{
					TestCase: "testCase",
				},
			},
		},
		{
			desc: "stormforger testcasefile",
			scenario: Scenario{
				StormForger: &StormForgerScenario{
					TestCaseFile: "./cases/testCaseFile.js",
				},
			},
			expected: Scenario{
				Name: "testcasefile",
				StormForger: &StormForgerScenario{
					TestCaseFile: "./cases/testCaseFile.js",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual := c.scenario
			actual.Default()
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestGoal_Default(t *testing.T) {
	cases := []struct {
		desc     string
		goal     Goal
		expected Goal
	}{
		{
			desc: "empty",
		},
		{
			desc: "latency prefix",
			goal: Goal{
				Name: "latency-p95",
			},
			expected: Goal{
				Name: "latency-p95",
				Latency: &LatencyGoal{
					LatencyType: LatencyPercentile95,
				},
			},
		},
		{
			desc: "latency suffix",
			goal: Goal{
				Name: "avg-latency",
			},
			expected: Goal{
				Name: "avg-latency",
				Latency: &LatencyGoal{
					LatencyType: LatencyMean,
				},
			},
		},
		{
			desc: "latency generated name",
			goal: Goal{
				Latency: &LatencyGoal{
					LatencyType: LatencyMean,
				},
			},
			expected: Goal{
				Name: "latency-mean",
				Latency: &LatencyGoal{
					LatencyType: LatencyMean,
				},
			},
		},
		{
			desc: "requests missing weight",
			goal: Goal{
				Name: "requests",
				Requests: &RequestsGoal{
					Selector: "test=test",
				},
			},
			expected: Goal{
				Name: "requests",
				Requests: &RequestsGoal{
					Selector: "test=test",
					Weights: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1"),
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual := c.goal
			actual.Default()
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestFixLatency(t *testing.T) {
	cases := []struct {
		desc    string
		input   string
		latency LatencyType
	}{
		{
			desc:    "unknown",
			input:   "unknown",
			latency: "",
		},
		{
			desc:    "leading hyphen",
			input:   "-p99",
			latency: LatencyPercentile99,
		},
		{
			desc:    "strip non-alphanumerics",
			input:   "~~avg~~",
			latency: LatencyMean,
		},
		{
			desc:    "upper case",
			input:   "MEDIAN",
			latency: LatencyPercentile50,
		},
		{
			desc:    "mixed case",
			input:   "Average",
			latency: LatencyMean,
		},
		{
			desc:    "spaced",
			input:   "Percentile 95",
			latency: LatencyPercentile95,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			assert.Equal(t, c.latency, FixLatency(LatencyType(c.input)))
		})
	}
}
