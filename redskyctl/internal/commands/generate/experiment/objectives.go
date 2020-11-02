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

var one = resource.MustParse("1")

const costQueryFormat = `({{ cpuRequests . "%s" }} * %d) + ({{ memoryRequests . "%s" | GB }} * %d)`

// priceList defines the price weights to use in the cost query.
var priceList = map[string]corev1.ResourceList{
	"default": {
		corev1.ResourceCPU:    resource.MustParse("17"),
		corev1.ResourceMemory: resource.MustParse("3"),
	},
	"gcp": {
		corev1.ResourceCPU:    resource.MustParse("17"),
		corev1.ResourceMemory: resource.MustParse("2"),
	},
	"aws": {
		corev1.ResourceCPU:    resource.MustParse("18"),
		corev1.ResourceMemory: resource.MustParse("5"),
	},
}

func init() {
	// Add some aliases
	priceList["googleCloudProvider"] = priceList["gcp"]
	priceList["amazonWebServices"] = priceList["aws"]
}

func (g *Generator) addObjectives(list *corev1.List) error {
	for i := range g.Application.Objectives {
		switch {

		case g.Application.Objectives[i].Cost != nil:
			addCostMetric(&g.Application, &g.Application.Objectives[i], list)

		}
	}

	return nil
}

func addCostMetric(app *v1alpha1.Application, obj *v1alpha1.Objective, list *corev1.List) {
	pricing := obj.Cost.Pricing
	if pricing == "" {
		switch {
		case app.GoogleCloudPlatform != nil:
			pricing = "gcp"
		case app.AmazonWebServices != nil:
			pricing = "aws"
		default:
			pricing = "default"
		}
	}

	cost := priceList[pricing]
	for k, v := range obj.Cost.PriceList {
		cost[k] = v
	}
	cpuWeight := cost.Cpu()
	if cpuWeight == nil || cpuWeight.IsZero() {
		cpuWeight = &one
	}
	memoryWeight := cost.Memory()
	if memoryWeight == nil || memoryWeight.IsZero() {
		memoryWeight = &one
	}

	lbl := labels.Set(obj.Cost.Labels).String()

	// Add the cost metric to the experiment
	exp := findOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     obj.Name,
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    fmt.Sprintf(costQueryFormat, lbl, cpuWeight.Value(), lbl, memoryWeight.Value()),
	})

	// The cost metric requires Prometheus
	ensurePrometheus(list)
}
