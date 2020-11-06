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

	"github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

var zero = resource.MustParse("0")

const costQueryFormat = `({{ cpuRequests . "%s" }} * %d) + ({{ memoryRequests . "%s" | GB }} * %d)`

func (g *Generator) addObjectives(list *corev1.List) error {
	for _, obj := range g.Application.Objectives {
		switch {

		case obj.Requests != nil:
			addRequestsMetric(&obj, list)

		}
	}

	return nil
}

func addRequestsMetric(obj *v1alpha1.Objective, list *corev1.List) {
	lbl := labels.Set(obj.Requests.Labels).String()

	cpuWeight := obj.Requests.Weights.Cpu()
	if cpuWeight == nil {
		cpuWeight = &zero
	}

	memoryWeight := obj.Requests.Weights.Memory()
	if memoryWeight == nil {
		memoryWeight = &zero
	}

	// Add the cost metric to the experiment
	exp := findOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     obj.Name,
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    fmt.Sprintf(costQueryFormat, lbl, cpuWeight.Value(), lbl, memoryWeight.Value()),
		Max:      obj.Max,
		Min:      obj.Min,
	})

	// The cost metric requires Prometheus
	ensurePrometheus(list)
}
