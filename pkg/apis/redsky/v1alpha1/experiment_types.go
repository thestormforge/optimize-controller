/*
Copyright 2019 GramLabs, Inc.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// Annotation that contains the URL of the experiment on the remote server
	AnnotationExperimentURL = "redskyops.dev/experiment-url"
	// Annotation that contains the URL used to obtain the next trial suggestion
	AnnotationNextTrialURL = "redskyops.dev/next-trial-url"
	// Annotation that contains the URL used to report trial observations
	AnnotationReportTrialURL = "redskyops.dev/report-trial-url"

	// Label that contains the name of the experiment associated with an object
	LabelExperiment = "redskyops.dev/experiment"
)

// Parameter represents the domain of a single component of the experiment search space
type Parameter struct {
	// The name of the parameter
	Name string `json:"name"`
	// The inclusive minimum value of the parameter
	Min int64 `json:"min,omitempty"`
	// The inclusive maximum value of the parameter
	Max int64 `json:"max,omitempty"`
}

// PatchType represents the allowable types of patches
type PatchType string

// MetricType represents the allowable types of metrics
type MetricType string

const (
	// Strategic merge patch
	PatchStrategic PatchType = "strategic"
	// Merge patch
	PatchMerge = "merge"
	// JSON patch (RFC 6902)
	PatchJSON = "json"

	// Local metrics are Go Templates evaluated against the trial itself. No external service is consulted, primarily
	// useful for extracting start and completion times.
	MetricLocal MetricType = "local"
	// Prometheus metrics issue PromQL queries to a matched service. Queries MUST evaluate to a scalar value.
	MetricPrometheus = "prometheus"
	// JSON path metrics fetch a JSON resource from the matched service. Queries are JSON path expression evaluated against the resource.
	MetricJSONPath = "jsonpath"
	// TODO "regex"?
)

// Metric represents an observable outcome from a trial run
type Metric struct {
	// The name of the metric
	Name string `json:"name"`
	// Indicator that the goal of the experiment is to minimize the value of this metric
	Minimize bool `json:"minimize,omitempty"`

	// The metric collection type, one of: local|prometheus|jsonpath, default: local
	Type MetricType `json:"type,omitempty"`
	// Collection type specific query, e.g. Go template for "local", PromQL for "prometheus" or a JSON pointer expression (with curly braces) for "jsonpath"
	Query string `json:"query"`
	// Collection type specific query for the error associated with collected metric value
	ErrorQuery string `json:"errorQuery,omitempty"`

	// The scheme to use when collecting metrics
	Scheme string `json:"scheme,omitempty"`
	// Selector matching services to collect this metric from, only the first matched service to provide a value is used
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// The port number or name on the matched service to collect the metric value from
	Port intstr.IntOrString `json:"port,omitempty"`
	// URL path component used to collect the metric value from an endpoint (used as a prefix for the Prometheus API)
	Path string `json:"path,omitempty"`
}

// PatchTemplate defines a target resource and a patch template to apply
type PatchTemplate struct {
	// The patch type, one of: json|merge|strategic, default: strategic
	Type PatchType `json:"type,omitempty"`
	// A Go Template that evaluates to valid patch.
	Patch string `json:"patch"`
	// Direct reference to the object the patch should be applied to.
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
}

// TrialTemplateSpec is used as a template for creating new trials
type TrialTemplateSpec struct {
	// Standard object metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior for the trial
	Spec TrialSpec `json:"spec"`
}

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// Replicas is the number of trials to execute concurrently, defaults to 1
	Replicas *int32 `json:"replicas,omitempty"`
	// Parallelism is the total number of expected replicas across all clusters, defaults to the replica count
	Parallelism *int32 `json:"parallelism,omitempty"`
	// Burn-in is the number of trials using random suggestions at the start of an experiment
	BurnIn *int32 `json:"burnIn,omitempty"`
	// Budget is the maximum number of trials to run for an experiment across all clusters
	Budget *int32 `json:"budget,omitempty"`
	// Parameters defines the search space for the experiment
	Parameters []Parameter `json:"parameters,omitempty"`
	// Metrics defines the outcomes for the experiment
	Metrics []Metric `json:"metrics,omitempty"`
	// Patches is a sequence of templates written against the experiment parameters that will be used to put the
	// cluster into the desired state
	Patches []PatchTemplate `json:"patches,omitempty"`
	// NamespaceSelector is used to determine which namespaces on a cluster can be used to create trials. Only a single
	// trial can be created in each namespace so if there are fewer matching namespaces then replicas, no trials will
	// be created
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// Selector locates trial resources that are part of this experiment
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Template for creating a new trial. The resulting trial must be matched by Selector. The template can provide an
	// initial namespace, however other namespaces (matched by NamespaceSelector) will be used if the effective
	// replica count is more then one
	Template TrialTemplateSpec `json:"template"`
}

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	// ActiveTrials is the observed number of running trials
	ActiveTrials int32 `json:"activeTrials"`
	// TODO Number of trials: Succeeded, Failed int32 (this is difficult, if not impossible, because we delete trials)
}

// +genclient
// +kubebuilder:object:root=true

// Experiment is the Schema for the experiments API
type Experiment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior for an experiment
	Spec ExperimentSpec `json:"spec,omitempty"`
	// Current status of an experiment
	Status ExperimentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExperimentList contains a list of Experiment
type ExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// The list of experiments
	Items []Experiment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Experiment{}, &ExperimentList{})
}
