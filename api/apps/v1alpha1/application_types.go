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

	"github.com/thestormforge/konjure/pkg/konjure"
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
	Resources konjure.Resources `json:"resources,omitempty"`

	// Parameters specifies additional details about the experiment parameters.
	Parameters []Parameter `json:"parameters,omitempty"`

	// Ingress specifies how to find the entry point to the application.
	Ingress *Ingress `json:"ingress,omitempty"`

	// The list of scenarios to optimize the application for.
	Scenarios []Scenario `json:"scenarios,omitempty"`

	// The list of objectives to optimize the application for.
	Objectives []Objective `json:"objectives,omitempty"`

	// StormForger allows you to configure StormForger to apply load on your application.
	StormForger *StormForger `json:"stormForger,omitempty"`
}

// Parameter describes the strategy for tuning the application.
type Parameter struct {
	// Information related to the discovery of container resources parameters like CPU and memory.
	ContainerResources *ContainerResources `json:"containerResources,omitempty"`
	// Information related to the discovery of replica parameters.
	Replicas *Replicas `json:"replicas,omitempty"`
	// Information related to the discovery of environment variables.
	EnvironmentVariable *EnvironmentVariable `json:"environmentVariable,omitempty"`
}

// ContainerResources specifies which resources in the application should have their container
// resources (CPU and memory) optimized.
type ContainerResources struct {
	// Label selector of Kubernetes objects to consider when generating container resources patches.
	Selector string `json:"selector,omitempty"`
	// The names of the resources to optimize. Defaults to ["memory", "cpu"].
	Resources []corev1.ResourceName `json:"resources,omitempty"`
}

// Replicas specifies which resources in the application should have their replica count optimized.
type Replicas struct {
	// Label selector of Kubernetes objects to consider when generating replica patches.
	Selector string `json:"selector,omitempty"`
}

// EnvironmentVariable specifies which environment variables in the application should have their value optimized.
type EnvironmentVariable struct {
	// Label selector of Kubernetes objects to consider when looking for environment variables.
	Selector string `json:"selector,omitempty"`
	// The name of the environment variable to optimize.
	Name string `json:"name,omitempty"`
	// The prefix of the value to use when setting the environment variable.
	Prefix string `json:"prefix,omitempty"`
	// The suffix of the value to use when setting the environment variable.
	Suffix string `json:"suffix,omitempty"`
	// The discrete values of the environment variable.
	Values []string `json:"values,omitempty"`
}

// Ingress describes the point of ingress to the application.
type Ingress struct {
	// The URL used to access the application from outside the cluster.
	URL string `json:"url,omitempty"`
}

// Scenario describes a specific pattern of load to optimize the application for.
type Scenario struct {
	// The name of scenario.
	Name string `json:"name,omitempty"`
	// StormForger configuration for the scenario.
	StormForger *StormForgerScenario `json:"stormforger,omitempty"`
	// Locust configuration for the scenario.
	Locust *LocustScenario `json:"locust,omitempty"`
	// Custom configuration for the scenario.
	Custom *CustomScenario `json:"custom,omitempty"`
}

// StormForgerScenario is used to generate load using StormForger.
type StormForgerScenario struct {
	// The test case can be used to specify an existing test case in the StormForger API or
	// it can be used to override the generated test case name when specified in conjunction
	// with the local test case file. The organization is optional if it is configured globally.
	TestCase string `json:"testCase,omitempty"`
	// Path to a local test case file used to define a new test case in the StormForger API.
	TestCaseFile string `json:"testCaseFile,omitempty"`
}

// LocustScenario is used to generate load using Locust.
type LocustScenario struct {
	// Path to a Python module file to import.
	Locustfile string `json:"locustfile,omitempty"`
	// Number of concurrent Locust users.
	Users *int `json:"users,omitempty"`
	// The rate per second in which users are spawned.
	SpawnRate *int `json:"spawnRate,omitempty"`
	// Stop after the specified amount of time.
	RunTime *metav1.Duration `json:"runTime,omitempty"`
}

// CustomScenario is used for advanced cases where more flexibility is required.
type CustomScenario struct {
	// Enables Prometheus Push Gateway support for objectives that require it.
	// The `PUSHGATEWAY_URL` environment variable will be added to all
	// containers when the trial job starts.
	UsePushGateway bool `json:"pushGateway,omitempty"`
	// The default specification of a pod to use for executing a trial.
	PodTemplate *corev1.PodTemplateSpec `json:"podTemplate,omitempty"`
	// Additional delay before starting the trial pod.
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// The estimated amount of time the trial should last.
	ApproximateRuntimeSeconds int32 `json:"approximateRuntimeSeconds,omitempty"`
	// Override the image of the first container in the trial pod.
	Image string `json:"image,omitempty"`
}

// Objective describes the goals of the optimization in terms of specific metrics.
type Objective struct {
	// The name of the objective. If omitted, a default name will be generated
	// based on the goals.
	Name string `json:"name,omitempty"`
	// The list of goals for the objective.
	Goals []Goal `json:"goals,omitempty"`
}

// Goal is an individual component of an objective.
type Goal struct {
	// The name of the goal. If no specific configuration is supplied, the name is
	// used to derive a configuration. For example, any valid latency (prefixed or
	// suffixed with "latency") will configure a default latency goal.
	Name string `json:"name,omitempty"`
	// The upper bound for the objective.
	Max *resource.Quantity `json:"max,omitempty"`
	// The lower bound for the objective.
	Min *resource.Quantity `json:"min,omitempty"`
	// Flag indicating that this objective should optimized instead of monitored (default: true).
	Optimize *bool `json:"optimize,omitempty"`

	// Requests is used to optimize the resources consumed by an application.
	Requests *RequestsGoal `json:"requests,omitempty"`
	// Latency is used to optimize the responsiveness of an application.
	Latency *LatencyGoal `json:"latency,omitempty"`
	// ErrorRate is used to optimize the failure rate of an application.
	ErrorRate *ErrorRateGoal `json:"errorRate,omitempty"`
	// Duration is used to optimize the elapsed time of an application performing a fixed amount of work.
	Duration *DurationGoal `json:"duration,omitempty"`
	// Prometheus is used to optimize against a Prometheus metric.
	Prometheus *PrometheusGoal `json:"prometheus,omitempty"`
	// Datadog is used to optimize against a Datadog metric.
	Datadog *DatadogGoal `json:"datadog,omitempty"`

	// IMPORTANT: Remember to update `isEmptyConfig` when adding new goal types

	// Internal use field for marking objectives as having been implemented. For example,
	// it may be impossible to optimize for some objectives based on the current state.
	Implemented bool `json:"-"`
}

// RequestsGoal is used to optimize the resource requests of an application in a specific scenario.
type RequestsGoal struct {
	// Label selector of the pods which should be considered when collecting cost information.
	Selector string `json:"selector,omitempty"`
	// Weights are used to determine which container resources should be optimized.
	Weights corev1.ResourceList `json:"weights,omitempty"`
}

// LatencyGoal is used to optimize the responsiveness of an application in a specific scenario.
type LatencyGoal struct {
	// The latency to optimize. Can be one of the following values:
	// `minimum` (or `min`), `maximum` (or `max`), `mean` (or `average`, `avg`),
	// `percentile_50` (or `p50`, `median`, `med`), `percentile_95` (or `p95`),
	// `percentile_99` (or `p99`).
	LatencyType
}

// UnmarshalJSON allows a latency objective to be specified as a simple string.
func (in *LatencyGoal) UnmarshalJSON(data []byte) error {
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

// ErrorRateGoal is used to optimize the error rate of an application in a specific scenario.
type ErrorRateGoal struct {
	// The error rate to optimize. Can be one of the following values: `requests`.
	ErrorRateType
}

// UnmarshalJSON allows an error rate objective to be specified as a simple string.
func (in *ErrorRateGoal) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &in.ErrorRateType)
}

// ErrorRateType describes something which can fail.
type ErrorRateType string

const (
	ErrorRateRequests ErrorRateType = "requests"
)

// DurationGoal is used to optimize the amount of time elapsed in a specific scenario.
type DurationGoal struct {
	// The duration to optimize. Can be one of the following values: `trial`.
	DurationType
}

// UnmarshalJSON allows a timing objective to be specified as a simple string.
func (in *DurationGoal) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &in.DurationType)
}

// DurationType describes something which occurs over an arbitrary time interval.
type DurationType string

const (
	DurationTrial DurationType = "trial"
)

// PrometheusGoal is used to define an external optimization metric from Prometheus.
type PrometheusGoal struct {
	// The PromQL query to execute; the result of this query MUST be a scalar value.
	Query string `json:"query"`
	// The URL of the Prometheus deployment, leave blank to leverage a Prometheus instance
	// whose lifecycle is tied to the trial.
	URL string `json:"url,omitempty"`
	// Flag indicating the goal of optimization should be to maximize a metric.
	Maximize bool `json:"maximize,omitempty"`
}

// DatadogGoal is used to define an external optimization metric from DataDog.
type DatadogGoal struct {
	// The [Datadog](https://docs.datadoghq.com/tracing/trace_search_and_analytics/query_syntax/) query to execute.
	Query string `json:"query"`
	// The aggregator to use on the query results (one of: avg, last, max, min, sum).
	Aggregator string `json:"aggregator,omitempty"`
	// Flag indicating the goal of optimization should be to maximize a metric.
	Maximize bool `json:"maximize,omitempty"`
}

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
