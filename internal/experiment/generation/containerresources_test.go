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

package generation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestToIntWithRange(t *testing.T) {
	cases := []struct {
		name     corev1.ResourceName
		input    string
		baseline int32
		min      int32
		max      int32
	}{
		{
			name:     corev1.ResourceCPU,
			input:    "0.1",
			baseline: 100,
			min:      50,
			max:      200,
		},
		{
			name:     corev1.ResourceCPU,
			input:    "250m",
			baseline: 250,
			min:      120,
			max:      500,
		},
		{
			name:     corev1.ResourceCPU,
			input:    "2505m",
			baseline: 2505,
			min:      1250,
			max:      4000,
		},
		{
			name:     corev1.ResourceCPU,
			input:    "500m",
			baseline: 500,
			min:      250,
			max:      1000,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "64Mi",
			baseline: 68,
			min:      32,
			max:      256,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "128Mi",
			baseline: 135,
			min:      64,
			max:      512,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "64M",
			baseline: 64,
			min:      32,
			max:      128,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "128M",
			baseline: 128,
			min:      64,
			max:      256,
		},
		{
			name:     corev1.ResourceCPU,
			input:    "4.0",
			baseline: 4000,
			min:      2000,
			max:      4000,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "4000Mi",
			baseline: 4195,
			min:      2048,
			max:      4195,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "4000M",
			baseline: 4000,
			min:      1024,
			max:      4096,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "4Gi",
			baseline: 4295,
			min:      2048,
			max:      4295,
		},
		{
			name:     corev1.ResourceMemory,
			input:    "4G",
			baseline: 4000,
			min:      1024,
			max:      4096,
		},
	}
	for _, c := range cases {
		t.Run(string(c.name)+c.input, func(t *testing.T) {
			resources := map[corev1.ResourceName]resource.Quantity{c.name: resource.MustParse(c.input)}
			baseline, min, max := toIntWithRange(resources, c.name)
			assert.Equal(t, c.baseline, baseline.IntVal, "baseline")
			assert.Equal(t, c.min, min, "minimum")
			assert.Equal(t, c.max, max, "maximum")

			// Check the value against what we are going to render into the template
			switch c.name {
			case corev1.ResourceMemory:
				inputValue := resource.MustParse(c.input)
				templateValue := resource.MustParse(fmt.Sprintf("%dM", baseline.IntVal))
				assert.Equal(t, inputValue.ScaledValue(resource.Mega), templateValue.ScaledValue(resource.Mega))
			case corev1.ResourceCPU:
				inputValue := resource.MustParse(c.input)
				templateValue := resource.MustParse(fmt.Sprintf("%dm", baseline.IntVal))
				assert.Equal(t, inputValue.ScaledValue(resource.Milli), templateValue.ScaledValue(resource.Milli))
			}
		})
	}
}
