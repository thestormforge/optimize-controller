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
	// The name of the objective.
	Name string `json:"name"`
	// Cost is used to identify which parts of the application impact the cost of running the application.
	Cost *CostObjective `json:"cost,omitempty"`
	// Latency is used to determine which measure of API performance to optimize.
	Latency *LatencyObjective `json:"latency,omitempty"`
}

// CostObjective is used to estimate the cost of running an application in a specific scenario.
type CostObjective struct {
	// Labels of the pods which should be considered when collecting cost information.
	Labels map[string]string `json:"labels,omitempty"`
	// The name of the pricing strategy to use.
	Pricing string `json:"pricing,omitempty"`
	// Explicit overrides to the pricing strategy.
	PriceList corev1.ResourceList `json:"priceList,omitempty"`
}

type LatencyType string

const (
	LatencyMin          LatencyType = "min"
	LatencyMax          LatencyType = "max"
	LatencyMean         LatencyType = "mean" // avg
	LatencyMedian       LatencyType = "median"
	LatencyPercentile95 LatencyType = "percentile_95" // p95
	LatencyPercentile99 LatencyType = "percentile_99" // p99
)

// LatencyObject is used to measure performance of an application under load.
type LatencyObjective struct {
	// The types of latencies to optimize.
	LatencyTypes []LatencyType `json:",inline"`
}

// GoogleCloudPlatform is used to configure details specific to applications hosted in GCP.
type GoogleCloudPlatform struct {
}

// AmazonWebServices is used to configure details specific to applications hosted in AWS.
type AmazonWebServices struct {
}

func init() {
	SchemeBuilder.Register(&Application{})
}
