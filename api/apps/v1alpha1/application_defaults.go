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
	StormForgerAccessTokenSecretName = "stormforger-service-account"
	StormForgerAccessTokenSecretKey  = "accessToken"
	defaultName                      = "default"
)

// Register the defaulting function for the application root object.
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

	in.StormForger.Default()

	// Count the number of objectives, this is necessary to accurately compute experiment names
	in.InitialObjectiveCount = len(in.Objectives)
}

func (in *Scenario) Default() {
	if in.Name == "" {
		switch {
		case in.StormForger != nil:
			in.Name = defaultScenarioName(in.StormForger.TestCase, in.StormForger.TestCaseFile)
		case in.Locust != nil:
			in.Name = defaultScenarioName(in.Locust.Locustfile)
		default:
			in.Name = defaultName
		}
	}
}

func (in *StormForger) Default() {
	if in == nil {
		return
	}

	in.AccessToken.Default()
}

func (in *StormForgerAccessToken) Default() {
	if in == nil {
		return
	}

	if in.File == "" && in.Literal == "" && in.SecretKeyRef == nil {
		in.SecretKeyRef = &corev1.SecretKeySelector{}
	}

	if in.SecretKeyRef != nil {
		if in.SecretKeyRef.Name == "" {
			in.SecretKeyRef.Name = StormForgerAccessTokenSecretName
		}
		if in.SecretKeyRef.Key == "" {
			in.SecretKeyRef.Key = StormForgerAccessTokenSecretKey
		}
	}
}

func (in *Objective) Default() {
	// If there is no explicit configuration, create it by parsing the name
	if in.Name != "" && in.needsConfig() {
		switch strings.Map(toName, in.Name) {

		case "cost":
			// TODO This should be smart enough to know if there is application wide cloud provider configuration
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("17"),
				corev1.ResourceMemory: resource.MustParse("3"),
			})

		case "cost-gcp", "gcp-cost":
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("17"),
				corev1.ResourceMemory: resource.MustParse("2"),
			})

		case "cost-aws", "aws-cost":
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("18"),
				corev1.ResourceMemory: resource.MustParse("5"),
			})

		case "cpu-requests", "cpu":
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			})

		case "memory-requests", "memory":
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1"),
			})

		case "error-rate", "error-ratio", "errors":
			defaultErrorRateObjective(in, ErrorRateRequests)

		case "duration", "time", "time-elapsed", "elapsed-time":
			defaultDurationObjective(in, DurationTrial)

		default:
			latencyName := strings.ReplaceAll(strings.Map(toName, in.Name), "latency", "")
			if l := FixLatency(LatencyType(latencyName)); l != "" {
				defaultLatencyObjective(in, l)
			}
		}
	}

	// If there are no explicit request weights, use 1
	if in.Requests != nil && in.Requests.Weights == nil {
		defaultRequestsObjectiveWeights(in, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1"),
		})
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
			// Do nothing, unlike a scenario, an empty objective is allowed to have an empty name
		}
	}
}

// needsConfig tests the objective to see if at least one configuration section is specified.
func (in *Objective) needsConfig() bool {
	return in.Requests == nil &&
		in.Latency == nil &&
		in.ErrorRate == nil &&
		in.Duration == nil
}

func defaultRequestsObjectiveWeights(obj *Objective, weights corev1.ResourceList) {
	if obj.Requests == nil {
		obj.Requests = &RequestsObjective{}
	}

	if obj.Requests.Weights == nil {
		obj.Requests.Weights = make(corev1.ResourceList)
	}

	for k, v := range weights {
		if _, ok := obj.Requests.Weights[k]; !ok {
			obj.Requests.Weights[k] = v
		}
	}
}

func defaultLatencyObjective(obj *Objective, latency LatencyType) {
	if obj.Latency == nil {
		obj.Latency = &LatencyObjective{}
	}

	if obj.Latency.LatencyType == "" {
		obj.Latency.LatencyType = latency
	}
}

func defaultErrorRateObjective(obj *Objective, errorRate ErrorRateType) {
	if obj.ErrorRate == nil {
		obj.ErrorRate = &ErrorRateObjective{}
	}

	if obj.ErrorRate.ErrorRateType == "" {
		obj.ErrorRate.ErrorRateType = errorRate
	}
}

func defaultDurationObjective(obj *Objective, duration DurationType) {
	if obj.Duration == nil {
		obj.Duration = &DurationObjective{}
	}

	if obj.Duration.DurationType == "" {
		obj.Duration.DurationType = duration
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
