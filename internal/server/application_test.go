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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestClusterExperimentToAPITemplate(t *testing.T) {
	one := intstr.FromInt(1)
	two := intstr.FromInt(2)
	three := intstr.FromString("three")

	exp := &optimizev1beta2.Experiment{
		Spec: optimizev1beta2.ExperimentSpec{
			Parameters: []optimizev1beta2.Parameter{
				{Name: "one", Min: 111, Max: 222, Baseline: &one},
				{Name: "two", Min: 1111, Max: 2222, Baseline: &two},
				{Name: "three", Min: 11111, Max: 22222, Baseline: &three},
				{Name: "test_case", Min: 1, Max: 1},
			},
			Metrics: []optimizev1beta2.Metric{
				{Name: "one", Minimize: true},
				{Name: "two", Minimize: false},
				{Name: "three", Minimize: true},
			},
		},
	}

	testCases := []struct {
		desc string
	}{
		{
			desc: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			template, err := ClusterExperimentToAPITemplate(exp)
			assert.NoError(t, err)
			assert.NotNil(t, template.Parameters)
			assert.NotNil(t, template.Metrics)

			// test_case is silently filters/dropped because min==max
			assert.Equal(t, 3, len(template.Parameters))
			assert.Equal(t, 3, len(template.Metrics))
		})
	}
}
