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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	localSchemeBuilder.Register(RegisterDefaults)
}

const (
	StormForgerAccessTokenSecretName = "stormforger-service-account"
	StormForgerAccessTokenSecretKey  = "accessToken"
)

// Register the defaulting function for the application root object.
func RegisterDefaults(s *runtime.Scheme) error {
	s.AddTypeDefaultingFunc(&Application{}, func(obj interface{}) { obj.(*Application).Default() })
	return nil
}

var _ admission.Defaulter = &Application{}

func (in *Application) Default() {
	for i := range in.Scenarios {
		in.Scenarios[i].Default()
	}

	for i := range in.Objectives {
		in.Objectives[i].Default()
	}

	in.StormForger.Default()

	in.initialObjectiveCount = len(in.Objectives)
}

func (in *Scenario) Default() {
	if in.Name == "" {
		if in.StormForger != nil && in.StormForger.TestCase != "" {
			in.Name = cleanName(in.StormForger.TestCase)
		} else if in.StormForger != nil && in.StormForger.TestCaseFile != "" {
			in.Name = cleanName(in.StormForger.TestCaseFile)
		} else if in.Locust != nil && in.Locust.Locustfile != "" {
			in.Name = cleanName(in.Locust.Locustfile)
			if in.Name == "locustfile" {
				in.Name = "default"
			}
		} else {
			in.Name = "default"
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
	switch strings.ToLower(in.Name) {

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

	default:

		latency := LatencyType(strings.ReplaceAll(in.Name, "latency", ""))
		latency = FixLatency(latency)
		if latency != "" {
			defaultLatencyObjective(in, latency)
		}

		if in.Requests != nil && in.Requests.Weights == nil {
			defaultRequestsObjectiveWeights(in, corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1"),
			})
		}

		if in.Name == "" {
			defaultObjectiveName(in)
		}
	}
}

func defaultRequestsObjectiveWeights(obj *Objective, weights corev1.ResourceList) {
	if obj.Requests == nil {
		if countConfigs(obj) != 0 {
			return
		}
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
		if countConfigs(obj) != 0 {
			return
		}
		obj.Latency = &LatencyObjective{}
	}

	if obj.Latency.LatencyType == "" {
		obj.Latency.LatencyType = latency
	}
}

func defaultErrorRateObjective(obj *Objective, errorRate ErrorRateType) {
	if obj.ErrorRate == nil {
		if countConfigs(obj) != 0 {
			return
		}
		obj.ErrorRate = &ErrorRateObjective{}
	}

	if obj.ErrorRate.ErrorRateType == "" {
		obj.ErrorRate.ErrorRateType = errorRate
	}
}

func defaultObjectiveName(obj *Objective) {
	switch {
	case obj.Requests != nil:
		obj.Name = "requests"
	case obj.Latency != nil:
		obj.Name = "latency-" + string(obj.Latency.LatencyType)
	case obj.ErrorRate != nil:
		obj.Name = "error-rate"
	}
}

func countConfigs(obj *Objective) int {
	var c int
	if obj.Requests != nil {
		c++
	}
	if obj.Latency != nil {
		c++
	}
	if obj.ErrorRate != nil {
		c++
	}
	return c
}

func cleanName(in string) string {
	name := filepath.Base(in)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.Map(func(r rune) rune {
		// TODO Other special characters?
		return unicode.ToLower(r)
	}, name)
	return name
}
