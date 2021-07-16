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
	"strings"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"k8s.io/apimachinery/pkg/api/resource"
)

var zero = resource.MustParse("0")

const requestsQueryFormat = `({{ cpuRequests . %q }} * %d) + ({{ memoryRequests . %q | GB }} * %d)`

type RequestsMetricsSource struct {
	Goal *optimizeappsv1alpha1.Goal
}

var _ MetricSource = &RequestsMetricsSource{}

func (s *RequestsMetricsSource) Metrics() ([]optimizev1beta2.Metric, error) {
	var result []optimizev1beta2.Metric
	if s.Goal == nil || s.Goal.Implemented {
		return result, nil
	}

	cpuWeight := s.Goal.Requests.Weights.Cpu()
	if cpuWeight == nil {
		cpuWeight = &zero
	}

	memoryWeight := s.Goal.Requests.Weights.Memory()
	if memoryWeight == nil {
		memoryWeight = &zero
	}

	query := fmt.Sprintf(requestsQueryFormat, s.Goal.Requests.Selector, cpuWeight.Value(), s.Goal.Requests.Selector, memoryWeight.Value())
	result = append(result, newGoalMetric(s.Goal, query))

	// If the name contains "cost" and the weights are non-zero, add non-optimized metrics for each request
	if strings.Contains(s.Goal.Name, "cost") &&
		!cpuWeight.IsZero() && !memoryWeight.IsZero() &&
		(s.Goal.Optimize == nil || *s.Goal.Optimize) {

		nonOptimized := false
		result = append(result,
			newGoalMetric(&optimizeappsv1alpha1.Goal{
				Name:     result[0].Name + "-cpu-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ cpuRequests . %q }}", s.Goal.Requests.Selector)),
			newGoalMetric(&optimizeappsv1alpha1.Goal{
				Name:     result[0].Name + "-memory-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ memoryRequests . %q | GB }}", s.Goal.Requests.Selector)),
		)
	}

	return result, nil
}
