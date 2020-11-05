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

	// GoogleCloudPlatform allows you to configure hosting details specific to GCP.
	GoogleCloudPlatform *GoogleCloudPlatform `json:"googleCloudPlatform,omitempty"`

	// AmazonWebServices allows you to configure hosting details specific to AWS.
	AmazonWebServices *AmazonWebServices `json:"amazonWebServices,omitempty"`

	// TODO We should have a qualityOfService: section were you can specify things like
	// the percentage of the max that resources are expected to use (then we add both limits and requests and a constraint)
	// or the "max latency" for the application (we could add a non-optimized metric to capture it).
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

	// Requests is used to optimize the resources consumed by an application.
	Requests *RequestsObjective `json:"requests,omitempty"`
	// Latency is used to optimize the responsiveness of an application.
	Latency *LatencyObjective `json:"latency,omitempty"`
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

// GoogleCloudPlatform is used to configure details specific to applications hosted in GCP.
type GoogleCloudPlatform struct {
}

// AmazonWebServices is used to configure details specific to applications hosted in AWS.
type AmazonWebServices struct {
}

func init() {
	SchemeBuilder.Register(&Application{})
}
