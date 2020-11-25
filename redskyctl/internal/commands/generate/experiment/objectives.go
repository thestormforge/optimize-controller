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

package experiment

import (
	"fmt"
	"strconv"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/generate/experiment/k8s"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/generate/experiment/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var zero = resource.MustParse("0")

const requestsQueryFormat = `({{ cpuRequests . %s }} * %d) + ({{ memoryRequests . %s | GB }} * %d)`

func (g *Generator) addObjectives(list *corev1.List) error {
	for i := range g.Application.Objectives {
		// Skip over objectives which have already been implemented using scenario specific logic
		if g.Application.Objectives[i].Implemented {
			continue
		}

		obj := &g.Application.Objectives[i]
		switch {

		case obj.Requests != nil:
			if err := addRequestsMetric(obj, list); err != nil {
				return err
			}

		case obj.Duration != nil:
			if err := addDurationMetric(obj, list); err != nil {
				return err
			}

		}
	}

	return nil
}

func addRequestsMetric(obj *redskyappsv1alpha1.Objective, list *corev1.List) error {
	// Generate the query
	ms := strconv.Quote(obj.Requests.MetricSelector)

	cpuWeight := obj.Requests.Weights.Cpu()
	if cpuWeight == nil {
		cpuWeight = &zero
	}

	memoryWeight := obj.Requests.Weights.Memory()
	if memoryWeight == nil {
		memoryWeight = &zero
	}

	query := fmt.Sprintf(requestsQueryFormat, ms, cpuWeight.Value(), ms, memoryWeight.Value())

	// Add the cost metric to the experiment
	req := k8s.NewObjectiveMetric(obj, query)
	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, req)

	// If the name contains "cost" and the weights are non-zero, add non-optimized metrics for each request
	if strings.Contains(obj.Name, "cost") && !cpuWeight.IsZero() && !memoryWeight.IsZero() && (obj.Optimize == nil || *obj.Optimize) {
		nonOptimized := false
		exp.Spec.Metrics = append(exp.Spec.Metrics,
			k8s.NewObjectiveMetric(&redskyappsv1alpha1.Objective{
				Name:     req.Name + "-cpu-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ cpuRequests . %s }}", ms)),
			k8s.NewObjectiveMetric(&redskyappsv1alpha1.Objective{
				Name:     req.Name + "-memory-requests",
				Optimize: &nonOptimized,
			}, fmt.Sprintf("{{ memoryRequests . %s | GB }}", ms)),
		)
	}

	// The cost metric requires Prometheus
	prometheus.AddSetupTask(list)

	return nil
}

func addDurationMetric(obj *redskyappsv1alpha1.Objective, list *corev1.List) error {
	if obj.Duration.DurationType != redskyappsv1alpha1.DurationTrial {
		return nil
	}

	m := k8s.NewObjectiveMetric(obj, `{{ duration .StartTime .CompletionTime }}`)
	m.Type = ""

	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, m)

	return nil
}
