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

package locust

import (
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/generate/experiment/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// addLocustObjectives adds metrics for objectives supported by Locust.
func addLocustObjectives(app *redskyappsv1alpha1.Application, list *corev1.List) error {
	for i := range app.Objectives {
		obj := &app.Objectives[i]
		switch {

		case obj.Latency != nil:
			addLocustLatencyMetric(obj, list)

		case obj.ErrorRate != nil:
			addLocustErrorRateMetric(obj, list)

		}
	}

	return nil
}

func addLocustLatencyMetric(obj *redskyappsv1alpha1.Objective, list *corev1.List) {
	var m string
	switch redskyappsv1alpha1.FixLatency(obj.Latency.LatencyType) {
	case redskyappsv1alpha1.LatencyMinimum:
		m = "min_response_time"
	case redskyappsv1alpha1.LatencyMaximum:
		m = "max_response_time"
	case redskyappsv1alpha1.LatencyMean:
		m = "average_response_time"
	case redskyappsv1alpha1.LatencyPercentile50:
		m = "p50"
	case redskyappsv1alpha1.LatencyPercentile95:
		m = "p95"
	case redskyappsv1alpha1.LatencyPercentile99:
		m = "p99"
	default:
		// This is not a latency measure that Locust can produce, skip it
		return
	}

	// Filter the metric to match what was sent to the Push Gateway
	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     obj.Name,
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Port:     intstr.FromInt(9090),
		Query:    `scalar(` + m + `{job="trialRun",instance="{{ .Trial.Name }}"})`,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	})
	obj.Implemented = true
}

func addLocustErrorRateMetric(obj *redskyappsv1alpha1.Objective, list *corev1.List) {
	if obj.ErrorRate.ErrorRateType != redskyappsv1alpha1.ErrorRateRequests {
		// This is not an error rate that Locust can produce, skip it
		return
	}

	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     obj.Name,
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Port:     intstr.FromInt(9090),
		Query:    `scalar(failure_count{job="trialRun",instance="{{ .Trial.Name }}"} / request_count{job="trialRun",instance="{{ .Trial.Name }}"})`,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	})
	obj.Implemented = true
}
