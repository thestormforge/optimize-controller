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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Optimization is a configuration setting for the optimizer
type Optimization struct {
	// Name is the name of the optimization configuration to set
	Name string `json:"name"`
	// Value is string representation of the optimization configuration
	Value string `json:"value"`
}

// Parameter represents the domain of a single component of the experiment search space
type Parameter struct {
	// The name of the parameter
	Name string `json:"name"`
	// The baseline value for this parameter.
	Baseline *intstr.IntOrString `json:"baseline,omitempty"`
	// The inclusive minimum value of the parameter
	Min int32 `json:"min,omitempty"`
	// The inclusive maximum value of the parameter
	Max int32 `json:"max,omitempty"`
	// The discrete allowed values of the parameter
	Values []string `json:"values,omitempty"`
}

// Constraint represents a constraint to the domain of the parameters
type Constraint struct {
	// The optional name of the constraint
	Name string `json:"name,omitempty"`
	// The ordering constraint to impose
	Order *OrderConstraint `json:"order,omitempty"`
	// The sum constraint to impose
	Sum *SumConstraint `json:"sum,omitempty"`
}

// OrderConstraint defines a constraint between the ordering of two parameters in the experiment
type OrderConstraint struct {
	// LowerParameter is the name of the parameter that must be the smaller of two parameters
	LowerParameter string `json:"lowerParameter"`
	// UpperParameter is the name of the parameter that must be the larger of two parameters
	UpperParameter string `json:"upperParameter"`
}

// SumConstraintParameter is a weighted parameter specification in a sum constraint
type SumConstraintParameter struct {
	// Name of the parameter
	Name string `json:"name"`
	// Weight of the parameter
	Weight resource.Quantity `json:"weight"`
}

// SumConstraint defines a constraint between the sum of a collection of parameters
type SumConstraint struct {
	// Bound for the sum of the listed parameters
	Bound resource.Quantity `json:"bound"`
	// IsUpperBound determines if the bound values is an upper or lower bound on the sum
	IsUpperBound bool `json:"isUpperBound,omitempty"`
	// Parameters that should be summed
	Parameters []SumConstraintParameter `json:"parameters"`
}

// MetricType represents the allowable types of metrics
type MetricType string

const (
	// MetricKubernetes metrics issue Kubernetes API requests using the target reference and selector (if no
	// reference is supplied, the trial itself is assumed). Queries are Go Templates evaluated against the
	// the result of the API call.
	MetricKubernetes MetricType = "kubernetes"
	// MetricPrometheus metrics issue PromQL queries to a matched service. Queries MUST evaluate to a scalar value.
	MetricPrometheus MetricType = "prometheus"
	// MetricDatadog metrics issue queries to the Datadog service. Requires API and application key configuration.
	MetricDatadog MetricType = "datadog"
	// MetricJSONPath metrics fetch a JSON resource from the matched service. Queries are JSON path expression evaluated against the resource.
	MetricJSONPath MetricType = "jsonpath"
)

// Metric represents an observable outcome from a trial run
type Metric struct {
	// The name of the metric
	Name string `json:"name"`
	// Indicator that the goal of the experiment is to minimize the value of this metric
	Minimize bool `json:"minimize,omitempty"`
	// The inclusive minimum allowed value for the metric
	Min *resource.Quantity `json:"min,omitempty"`
	// The inclusive maximum allowed value for the metric
	Max *resource.Quantity `json:"max,omitempty"`
	// Indicator that this metric should be optimized (default: true)
	Optimize *bool `json:"optimize,omitempty"`

	// The metric collection type, one of: local|pods|prometheus|datadog|jsonpath, default: local
	Type MetricType `json:"type,omitempty"`
	// Collection type specific query, e.g. Go template for "local", PromQL for "prometheus" or a JSON pointer expression (with curly braces) for "jsonpath"
	Query string `json:"query"`
	// Collection type specific query for the error associated with collected metric value
	ErrorQuery string `json:"errorQuery,omitempty"`

	// URL to use when querying remote metric sources.
	URL string `json:"url,omitempty"`

	// Target reference of the Kubernetes object to query for metric information. Can be used
	// in conjunction with the selector when the name is empty. Mutually exclusive with the URL.
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
	// Selector matching to apply in conjunction with the target reference.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// PatchReadinessGate contains a reference to a condition
type PatchReadinessGate struct {
	// ConditionType refers to a condition in the patched target's condition list
	ConditionType string `json:"conditionType"`
}

// PatchType represents the allowable types of patches
type PatchType string

const (
	// PatchStrategic is the patch type for a strategic merge patch
	PatchStrategic PatchType = "strategic"
	// PatchMerge is the patch type for a merge patch
	PatchMerge PatchType = "merge"
	// PatchJSON is the patch type for aJSON patch (RFC 6902)
	PatchJSON PatchType = "json"
)

// PatchTemplate defines a target resource and a patch template to apply
type PatchTemplate struct {
	// The patch type, one of: strategic|merge|json, default: strategic
	Type PatchType `json:"type,omitempty"`
	// A Go Template that evaluates to valid patch
	Patch string `json:"patch"`
	// Direct reference to the object the patch should be applied to
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
	// ReadinessGates will be evaluated for patch target readiness. A patch target is ready if all conditions specified
	// in the readiness gates have a status equal to "True". If no readiness gates are specified, some target types may
	// have default gates assigned to them. Some condition checks may result in errors, e.g. a condition type of "Ready"
	// is not allowed for a ConfigMap. Condition types starting with "redskyops.dev/" may not appear in the patched
	// target's condition list, but are still evaluated against the resource's state.
	ReadinessGates []PatchReadinessGate `json:"readinessGates,omitempty"`
}

// NamespaceTemplateSpec is used as a template for creating new namespaces
type NamespaceTemplateSpec struct {
	// Standard object metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the namespace
	Spec corev1.NamespaceSpec `json:"spec,omitempty"`
}

// TrialTemplateSpec is used as a template for creating new trials
type TrialTemplateSpec struct {
	// Standard object metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior for the trial
	Spec TrialSpec `json:"spec,omitempty"`
}

// ExperimentConditionType represents the possible observable conditions for an experiment
type ExperimentConditionType string

const (
	// ExperimentFailed is a condition that indicates an experiment failed
	ExperimentFailed ExperimentConditionType = "redskyops.dev/experiment-failed"
)

// ExperimentCondition represents an observed condition of an experiment
type ExperimentCondition struct {
	// The condition type
	Type ExperimentConditionType `json:"type"`
	// The status of the condition, one of "True", "False", or "Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last known time the condition was checked
	LastProbeTime metav1.Time `json:"lastProbeTime"`
	// The time at which the condition last changed status
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// A reason code describing the why the condition occurred
	Reason string `json:"reason,omitempty"`
	// A human readable message describing the transition
	Message string `json:"message,omitempty"`
}

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// Replicas is the number of trials to execute concurrently, defaults to 1
	Replicas *int32 `json:"replicas,omitempty"`
	// Optimization defines additional configuration for the optimization
	Optimization []Optimization `json:"optimization,omitempty"`
	// Parameters defines the search space for the experiment
	Parameters []Parameter `json:"parameters"`
	// Constraints defines restrictions on the parameter domain for the experiment
	Constraints []Constraint `json:"constraints,omitempty"`
	// Metrics defines the outcomes for the experiment
	Metrics []Metric `json:"metrics"`
	// Patches is a sequence of templates written against the experiment parameters that will be used to put the
	// cluster into the desired state
	Patches []PatchTemplate `json:"patches,omitempty"`
	// NamespaceSelector is used to locate existing namespaces for trials
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// NamespaceTemplate can be specified to create new namespaces for trials; if specified created namespaces must be
	// matched by the namespace selector
	NamespaceTemplate *NamespaceTemplateSpec `json:"namespaceTemplate,omitempty"`
	// Selector locates trial resources that are part of this experiment
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// TrialTemplate for creating a new trial. The resulting trial must be matched by Selector. The template can provide an
	// initial namespace, however other namespaces (matched by NamespaceSelector) will be used if the effective
	// replica count is more then one
	TrialTemplate TrialTemplateSpec `json:"trialTemplate,omitempty"`
}

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	// Phase is a brief human readable description of the experiment status
	Phase string `json:"phase"`
	// ActiveTrials is the observed number of running trials
	ActiveTrials int32 `json:"activeTrials"`
	// Conditions is the current state of the experiment
	Conditions []ExperimentCondition `json:"conditions,omitempty"`
	// TODO Number of trials: Succeeded, Failed int32 (this would need to be fetch remotely, falling back to the in cluster count)
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// Experiment is the Schema for the experiments API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Experiment status"
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
