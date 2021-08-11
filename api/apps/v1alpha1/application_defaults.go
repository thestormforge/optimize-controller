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

package v1alpha1

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(RegisterDefaults)
}

const (
	StormForgePerformanceAccessTokenSecretName = "stormforge-perf-service-accounts"
	defaultName                                = "default"
)

// RegisterDefaults registers the defaulting function for the application root object.
func RegisterDefaults(s *runtime.Scheme) error {
	s.AddTypeDefaultingFunc(&Application{}, func(obj interface{}) { obj.(*Application).Default() })
	return nil
}

func (in *Application) Default() {
	for i := range in.Scenarios {
		in.Scenarios[i].Default()
	}

	for i := range in.Objectives {
		in.Objectives[i].Default()
	}
}

func (in *Scenario) Default() {
	if in.Name == "" {
		switch {
		case in.StormForge != nil:
			in.Name = defaultScenarioName(in.StormForge.TestCase, in.StormForge.TestCaseFile)
		case in.Locust != nil:
			in.Name = defaultScenarioName(in.Locust.Locustfile)
		case in.Custom != nil:
			in.Name = defaultCustomScenarioName(in.Custom)
		default:
			in.Name = defaultName
		}
	}
}

func (in *Objective) Default() {
	for i := range in.Goals {
		in.Goals[i].Default()
	}

	if in.Name == "" {
		switch len(in.Goals) {
		case 1:
			in.Name = in.Goals[0].Name
		case 2:
			in.Name = fmt.Sprintf("%s-vs-%s", in.Goals[0].Name, in.Goals[1].Name)
		default:
			in.Name = defaultName
		}
	}
}

func (in *Goal) Default() {
	// If there is no explicit configuration, create it by parsing the name
	if in.Name != "" && isEmptyConfig(in) {
		name := strings.Map(toName, in.Name)
		switch name {

		case "error-rate", "error-ratio", "errors":
			defaultErrorRateGoal(in, ErrorRateRequests)

		case "duration", "time", "time-elapsed", "elapsed-time":
			defaultDurationGoal(in, DurationTrial)

		default:
			if w := DefaultCostWeights(name); w != nil {
				defaultRequestsGoalWeights(in, w)
			}

			latencyName := strings.ReplaceAll(name, "latency", "")
			if l := FixLatency(LatencyType(latencyName)); l != "" {
				defaultLatencyGoal(in, l)
			}
		}
	}

	// The request may have a selector but still needs weights
	if in.Requests != nil && in.Requests.Weights == nil {
		w := DefaultCostWeights(in.Name)
		if w == nil {
			// If there are no explicit request weights, use 1
			w = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1"),
			}
		}
		defaultRequestsGoalWeights(in, w)
	}

	// Default the name only after the rest of the state is consistent
	if in.Name == "" {
		switch {
		case in.Requests != nil:
			in.Name = defaultObjectiveName("requests")
		case in.Latency != nil:
			in.Name = defaultObjectiveName("latency", string(in.Latency.LatencyType))
		case in.ErrorRate != nil:
			in.Name = defaultObjectiveName("error-rate")
		case in.Duration != nil:
			in.Name = defaultObjectiveName("duration")
		default:
			// Do nothing, an empty goal is allowed to have an empty name
		}
	}
}

// isEmptyConfig tests the goal to see if at least one configuration section is specified.
func isEmptyConfig(goal *Goal) bool {
	return goal.Requests == nil &&
		goal.Latency == nil &&
		goal.ErrorRate == nil &&
		goal.Duration == nil &&
		goal.Prometheus == nil &&
		goal.Datadog == nil
}

func defaultRequestsGoalWeights(goal *Goal, weights corev1.ResourceList) {
	if goal.Requests == nil {
		goal.Requests = &RequestsGoal{}
	}

	if goal.Requests.Weights == nil {
		goal.Requests.Weights = make(corev1.ResourceList)
	}

	for k, v := range weights {
		if _, ok := goal.Requests.Weights[k]; !ok {
			goal.Requests.Weights[k] = v
		}
	}
}

func defaultLatencyGoal(goal *Goal, latency LatencyType) {
	if goal.Latency == nil {
		goal.Latency = &LatencyGoal{}
	}

	if goal.Latency.LatencyType == "" {
		goal.Latency.LatencyType = latency
	}
}

func defaultErrorRateGoal(goal *Goal, errorRate ErrorRateType) {
	if goal.ErrorRate == nil {
		goal.ErrorRate = &ErrorRateGoal{}
	}

	if goal.ErrorRate.ErrorRateType == "" {
		goal.ErrorRate.ErrorRateType = errorRate
	}
}

func defaultDurationGoal(goal *Goal, duration DurationType) {
	if goal.Duration == nil {
		goal.Duration = &DurationGoal{}
	}

	if goal.Duration.DurationType == "" {
		goal.Duration.DurationType = duration
	}
}

func defaultScenarioName(values ...string) string {
	for _, in := range values {
		name := filepath.Base(in)
		if name == "." || name == "locustfile.py" {
			continue
		}

		name = strings.TrimSuffix(name, filepath.Ext(name))
		name = strings.Map(toName, name)

		return name
	}

	return defaultName
}

func defaultCustomScenarioName(custom *CustomScenario) string {
	image := custom.Image

	// Check the pod name or the first container
	if custom.PodTemplate != nil {
		if custom.PodTemplate.Name != "" {
			return custom.PodTemplate.Name
		}

		containers := custom.PodTemplate.Spec.Containers
		if len(containers) > 0 {
			if containers[0].Name != "" {
				return containers[0].Name
			}
			if image == "" {
				image = containers[0].Image
			}
		}
	}

	// Try to take the basename of the image
	if image != "" {
		name := image
		name = name[strings.LastIndex(name, "/")+1:]
		if pos := strings.Index(name, ":"); pos > 0 {
			name = name[0:pos]
		}
		return name
	}

	return defaultName
}

func defaultObjectiveName(values ...string) string {
	nonEmpty := values[:0]
	for _, v := range values {
		if v != "" {
			nonEmpty = append(nonEmpty, v)
		}
	}

	if len(nonEmpty) > 0 {
		return strings.Join(nonEmpty, "-")
	}

	return defaultName
}

func toName(r rune) rune {
	// TODO Other special characters?
	if r == '_' {
		return '-'
	}
	return unicode.ToLower(r)
}
