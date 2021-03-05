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
	"strconv"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var zero = resource.MustParse("0")

const requestsQueryFormat = `({{ cpuRequests . %s }} * %d) + ({{ memoryRequests . %s | GB }} * %d)`

type RequestsMetricsSource struct {
	Objective *redskyappsv1alpha1.Objective
}

var _ MetricSource = &RequestsMetricsSource{}

func (s *RequestsMetricsSource) Metrics() ([]redskyv1beta1.Metric, error) {
	var result []redskyv1beta1.Metric
	if s.Objective.Implemented {
		return result, nil
	}

	// Generate the query
	ms := strconv.Quote(s.Objective.Requests.MetricSelector)

	cpuWeight := s.Objective.Requests.Weights.Cpu()
	if cpuWeight == nil {
		cpuWeight = &zero
	}

	memoryWeight := s.Objective.Requests.Weights.Memory()
	if memoryWeight == nil {
		memoryWeight = &zero
	}

	query := fmt.Sprintf(requestsQueryFormat, ms, cpuWeight.Value(), ms, memoryWeight.Value())
	result = append(result, newObjectiveMetric(s.Objective, query))

	// If the name contains "cost" and the weights are non-zero, add non-optimized metrics for each request
	if strings.Contains(s.Objective.Name, "cost") &&
		!cpuWeight.IsZero() && !memoryWeight.IsZero() &&
		(s.Objective.Optimize == nil || *s.Objective.Optimize) {

		nonOptimized := false
		result = append(result,
			newObjectiveMetric(&redskyappsv1alpha1.Objective{
				Name:     result[0].Name + "-cpu-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ cpuRequests . %s }}", ms)),
			newObjectiveMetric(&redskyappsv1alpha1.Objective{
				Name:     result[0].Name + "-memory-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ memoryRequests . %s | GB }}", ms)),
		)
	}

	return result, nil
}
