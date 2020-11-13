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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: Application is not a spec/status style object, it contains possible file system references

// Application represents a description of an application to run experiments on.
// +kubebuilder:object:root=true
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Resources are references to application resources to consider in the generation of the experiment.
	// These strings are the same format as used by Kustomize.
	Resources []string `json:"resources,omitempty"`

	// Parameters specifies additional details about the experiment parameters.
	Parameters *Parameters `json:"parameters,omitempty"`

	// Ingress specifies how to find the entry point to the application.
	Ingress *Ingress `json:"ingress,omitempty"`

	// The list of scenarios to optimize the application for.
	Scenarios []Scenario `json:"scenarios,omitempty"`

	// The list of objectives to optimizat the application for.
	Objectives []Objective `json:"objectives,omitempty"`

	// StormForger allows you to configure StormForger to apply load on your application.
	StormForger *StormForger `json:"stormForger,omitempty"`
}

// Parameters describes the strategy for tuning the application.
type Parameters struct {
	// Information related to the discovery of container resources parameters like CPU and memory.
	ContainerResources *ContainerResources `json:"containerResources,omitempty"`
}

// ContainerResources specifies which resources in the application should have their container
// resources (CPU and memory) optimized.
type ContainerResources struct {
	// Labels of Kubernetes objects to consider when generating container resources patches.
	Labels map[string]string `json:"labels,omitempty"`
}

// Ingress describes the point of ingress to the application.
type Ingress struct {
	// The URL used to access the application from outside the cluster.
	URL string `json:"url,omitempty"`
}

// Scenario describes a specific pattern of load to optimize the application for.
type Scenario struct {
	// The name of scenario.
	Name string `json:"name"`
	// StormForger configuration for the scenario.
	StormForger *StormForgerScenario `json:"stormforger,omitempty"`
}

// StormForgerScenario is used to generate load using StormForger.
type StormForgerScenario struct {
	// Override the generated test case name.
	TestCase string `json:"testCase,omitempty"`
	// Path to the test case file.
	TestCaseFile string `json:"testCaseFile,omitempty"`
}

// Objective describes the goal of the optimization in terms of specific metrics.
type Objective struct {
	// The name of the objective. If no objective specific configuration is supplied, the name is
	// used to derive a configuration.
	Name string `json:"name"`
	// The upper bound for the objective.
	Max *resource.Quantity `json:"max,omitempty"`
	// The lower bound for the objective.
	Min *resource.Quantity `json:"min,omitempty"`
	// Flag indicating that this objective should optimized instead of monitored (default: true).
	Optimize *bool `json:"optimize,omitempty"`

	// Requests is used to optimize the resources consumed by an application.
	Requests *RequestsObjective `json:"requests,omitempty"`
	// Latency is used to optimize the responsiveness of an application.
	Latency *LatencyObjective `json:"latency,omitempty"`

	// Internal use field for marking objectives as having been implemented. For example,
	// it may be impossible to optimize for some objectives based on the current state.
	Implemented bool `json:"-"`
}

// RequestsObjective is used to optimize the resource requests of an application in a specific scenario.
type RequestsObjective struct {
	// Labels of the pods which should be considered when collecting cost information.
	Labels map[string]string `json:"labels,omitempty"`
	// Weights are used to determine which container resources should be optimized.
	Weights corev1.ResourceList `json:"weights,omitempty"`
}

// LatencyObject is used to optimize the responsiveness of an application in a specific scenario.
type LatencyObjective struct {
	// The latency to optimize.
	LatencyType
}

// UnmarshalJSON allows a latency objective to be specified as a simple string.
func (in *LatencyObjective) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &in.LatencyType)
}

// LatencyType describes a measure of latency.
type LatencyType string

const (
	LatencyMinimum      LatencyType = "minimum"
	LatencyMaximum      LatencyType = "maximum"
	LatencyMean         LatencyType = "mean"
	LatencyPercentile50 LatencyType = "percentile_50"
	LatencyPercentile95 LatencyType = "percentile_95"
	LatencyPercentile99 LatencyType = "percentile_99"
)

// StormForger describes global configuration related to StormForger.
type StormForger struct {
	// The name of the StormForger organization.
	Organization string `json:"org,omitempty"`
	// Configuration for the StormForger service account.
	AccessToken *StormForgerAccessToken `json:"accessToken,omitempty"`
}

// StormForgerAccessToken is used to configure a service account access token for the StormForger API.
type StormForgerAccessToken struct {
	// The path to the file that contains the service account access token.
	File string `json:"file,omitempty"`
	// A literal token value, this should only be used for testing as it is not secure.
	Literal string `json:"literal,omitempty"`
	// Reference to an existing secret key that contains the access token.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Application{})
}
