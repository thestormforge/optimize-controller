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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

var one = resource.MustParse("1")

const costQueryFormat = `({{ cpuRequests . "%s" }} * %d) + ({{ memoryRequests . "%s" | GB }} * %d)`

func AddCostMetric(app *Application, exp *redskyv1beta1.Experiment) {
	if app.Cost == nil || len(app.Cost.Labels) == 0 {
		return
	}

	// The cost metric requires Prometheus
	ensurePrometheus(exp)

	lbl := labels.Set(app.Cost.Labels).String()
	var cpuWeight, memoryWeight *resource.Quantity

	if app.CloudProvider != nil {
		cpuWeight = app.CloudProvider.Cost.Cpu()
		memoryWeight = app.CloudProvider.Cost.Memory()
	}
	if cpuWeight == nil || cpuWeight.IsZero() {
		cpuWeight = &one
	}
	if memoryWeight == nil || memoryWeight.IsZero() {
		memoryWeight = &one
	}

	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     "cost",
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    fmt.Sprintf(costQueryFormat, lbl, cpuWeight.Value(), lbl, memoryWeight.Value()),
	})
}

func ensurePrometheus(exp *redskyv1beta1.Experiment) {
	// Return if we see the Prometheus setup task
	trialSpec := &exp.Spec.TrialTemplate.Spec
	for _, st := range trialSpec.SetupTasks {
		if st.Image == "" && len(st.Args) == 2 && st.Args[0] == "prometheus" && st.Args[1] == "$(MODE)" {
			return
		}
	}

	// Add the missing setup task
	trialSpec.SetupTasks = append(trialSpec.SetupTasks, redskyv1beta1.SetupTask{
		Name: "monitoring",
		Args: []string{"prometheus", "$(MODE)"},
	})
}
