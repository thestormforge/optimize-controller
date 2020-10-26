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

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

var one = resource.MustParse("1")

const costQueryFormat = `({{ cpuRequests . "%s" }} * %d) + ({{ memoryRequests . "%s" | GB }} * %d)`

func addApplicationMetrics(app *Application, list *corev1.List) error {
	// Add a cost metric
	if app.Cost != nil && len(app.Cost.Labels) > 0 {
		addCostMetric(app, list)
	}

	return nil
}

func addCostMetric(app *Application, list *corev1.List) {
	lbl := labels.Set(app.Cost.Labels).String()

	// Compute the cloud provider specific cost weights
	cost := computeCost(app.CloudProvider)
	cpuWeight := cost.Cpu()
	if cpuWeight == nil || cpuWeight.IsZero() {
		cpuWeight = &one
	}
	memoryWeight := cost.Memory()
	if memoryWeight == nil || memoryWeight.IsZero() {
		memoryWeight = &one
	}

	// Add the cost metric to the experiment
	exp := findOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     "cost",
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    fmt.Sprintf(costQueryFormat, lbl, cpuWeight.Value(), lbl, memoryWeight.Value()),
	})

	// The cost metric requires Prometheus
	ensurePrometheus(list)
}

// computeCost returns the cost weightings for a cloud provider.
// CPU weights are $/vCPU, Memory weights are $/GB
func computeCost(cp *CloudProvider) corev1.ResourceList {
	if cp != nil && cp.GenericCloudProvider != nil {
		return genericCost(cp.GenericCloudProvider)
	}
	if cp != nil && cp.GCP != nil {
		return gcpCost(cp.GCP)
	}
	if cp != nil && cp.AWS != nil {
		return awsCost(cp.AWS)
	}
	return genericCost(nil)
}

func gcpCost(gcp *GoogleCloudPlatform) corev1.ResourceList {
	cost := gcp.Cost
	addDefaultCost(&cost, corev1.ResourceCPU, "22")
	addDefaultCost(&cost, corev1.ResourceMemory, "3")
	return cost
}

func awsCost(aws *AmazonWebServices) corev1.ResourceList {
	cost := aws.Cost
	addDefaultCost(&cost, corev1.ResourceCPU, "22")
	addDefaultCost(&cost, corev1.ResourceMemory, "3")
	return cost
}

func genericCost(p *GenericCloudProvider) corev1.ResourceList {
	cost := corev1.ResourceList{}
	if p != nil && p.Cost != nil {
		cost = p.Cost
	}
	addDefaultCost(&cost, corev1.ResourceCPU, "22")
	addDefaultCost(&cost, corev1.ResourceMemory, "3")
	return cost
}

func addDefaultCost(r *corev1.ResourceList, name corev1.ResourceName, value string) {
	if v, ok := (*r)[name]; !ok || v.IsZero() {
		(*r)[name] = resource.MustParse(value)
	}
}
