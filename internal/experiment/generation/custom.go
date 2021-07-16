/*
Copyright 2021 GramLabs, Inc.

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
	"math"
	"strconv"
	"strings"
	"time"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomSource struct {
	Scenario    *optimizeappsv1alpha1.Scenario
	Objective   *optimizeappsv1alpha1.Objective
	Application *optimizeappsv1alpha1.Application
}

var _ ExperimentSource = &CustomSource{}
var _ MetricSource = &CustomSource{}

func (s *CustomSource) Update(exp *optimizev1beta2.Experiment) error {
	if s.Scenario == nil || s.Application == nil {
		return nil
	}

	if s.Scenario.Custom.PodTemplate != nil {
		s.Scenario.Custom.PodTemplate.DeepCopyInto(ensureTrialJobPod(exp))
	}

	if d := s.Scenario.Custom.InitialDelaySeconds; d > 0 {
		exp.Spec.TrialTemplate.Spec.InitialDelaySeconds = d
	}

	if rt := s.Scenario.Custom.ApproximateRuntimeSeconds; rt > 0 {
		exp.Spec.TrialTemplate.Spec.ApproximateRuntime = &metav1.Duration{Duration: time.Duration(rt) * time.Second}
	}

	if s.Scenario.Custom.Image != "" {
		pod := ensureTrialJobPod(exp)
		if len(pod.Spec.Containers) == 0 {
			pod.Spec.Containers = make([]corev1.Container, 1)
		}
		pod.Spec.Containers[0].Image = s.Scenario.Custom.Image
	}

	// It is possible we ended up in an invalid state, try to clean things up
	if exp.Spec.TrialTemplate.Spec.JobTemplate != nil {
		pod := ensureTrialJobPod(exp)
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == "" {
				name := pod.Spec.Containers[i].Image
				name = name[strings.LastIndex(name, "/")+1:]
				if pos := strings.Index(name, ":"); pos > 0 {
					name = name[0:pos]
				}
				pod.Spec.Containers[i].Name = name
			}
		}
	}

	return nil
}

func (s *CustomSource) Metrics() ([]optimizev1beta2.Metric, error) {
	var result []optimizev1beta2.Metric
	if s.Objective == nil {
		return result, nil
	}

	for i := range s.Objective.Goals {
		goal := &s.Objective.Goals[i]
		switch {

		case goal.Implemented:
			// Do nothing

		case goal.Requests != nil:
			if s.Scenario.Custom.UsePushGateway {
				continue
			}

			var weights []string
			for n, q := range goal.Requests.Weights {
				var scale float64 = 1
				if n == corev1.ResourceMemory {
					scale = 4 // Adjust memory weight from byte to gb
				}
				w := float64(q.Value()) / math.Pow(1000, scale)
				weights = append(weights, fmt.Sprintf("%s=%s", n, strconv.FormatFloat(w, 'f', -1, 64)))
			}
			query := fmt.Sprintf("{{ resourceRequests .Target %q }}", strings.Join(weights, ","))

			labelSelector, err := metav1.ParseToLabelSelector(goal.Requests.Selector)
			if err != nil {
				return nil, err
			}

			m := newGoalMetric(goal, query)
			m.Type = ""
			m.Target = &optimizev1beta2.ResourceTarget{
				APIVersion:    "v1",
				Kind:          "PodList",
				LabelSelector: labelSelector,
			}
			result = append(result, m)

		}
	}

	return result, nil
}
