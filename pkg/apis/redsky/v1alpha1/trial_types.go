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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// Annotation that contains a comma-delimited list of initializing processes. Similar to a "finalizer", the trial
	// will not start executing until the initializer is empty.
	AnnotationInitializer = "redskyops.dev/initializer"

	// Label that contains the name of the trial associated with an object
	LabelTrial = "redskyops.dev/trial"
	// Label that contains the role in trial execution
	LabelTrialRole = "redskyops.dev/trial-role"
)

// HelmValue represents a value in a Helm template
type HelmValue struct {
	// The name of Helm value as passed to one of the set options
	Name string `json:"name"`
	// Force the value to be treated as a string
	ForceString bool `json:"forceString,omitempty"`
	// Set a Helm value using the evaluated template. Templates are evaluated using the same rules as patches
	Value intstr.IntOrString `json:"value,omitempty"`
	// Source for a Helm value
	ValueFrom *HelmValueSource `json:"valueFrom,omitempty"`
}

// HelmValueSource represents a source for a Helm value
type HelmValueSource struct {
	// Selects a trial parameter assignment as a Helm value
	ParameterRef *ParameterSelector `json:"parameterRef,omitempty"`
	// TODO Also support the corev1.EnvVarSource selectors?
}

// ParameterSelector selects a trial parameter assignment. Note that parameters values are used as is (i.e. in
// numeric form), for more control over the formatting of a parameter assignment use the template option on HelmValue.
type ParameterSelector struct {
	// The name of the trial parameter to use
	Name string `json:"name"`
	// TODO Offer simple manipulations via "percent", "delta", "scale"? Allow string references to other parameters
}

// HelmValueFromSource represents a source of a values mapping
type HelmValuesFromSource struct {
	// The ConfigMap to select from
	ConfigMap *ConfigMapHelmValuesFromSource `json:"configMap,omitempty"`
	// TODO Secret support?
}

// ConfigMapHelmValuesFromSource is a reference to a ConfigMap that contains "*values.yaml" keys
// TODO How do document the side effect of things like patches in the ConfigMap also being applied?
type ConfigMapHelmValuesFromSource struct {
	corev1.LocalObjectReference `json:",inline"`
}

// SetupTask represents the configuration necessary to apply application state to the cluster
// prior to each trial run and remove that state after the run concludes
type SetupTask struct {
	// The name that uniquely identifies the setup task
	Name string `json:"name"`
	// Override the default image used for performing setup tasks
	Image string `json:"image,omitempty"`
	// Flag to indicate the creation part of the task can be skipped
	SkipCreate bool `json:"skipCreate,omitempty"`
	// Flag to indicate the deletion part of the task can be skipped
	SkipDelete bool `json:"skipDelete,omitempty"`
	// Volume mounts for the setup task
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// The Helm chart reference to release as part of this task
	HelmChart string `json:"helmChart,omitempty"`
	// The Helm values to set, ignored unless helmChart is also set
	HelmValues []HelmValue `json:"helmValues,omitempty"`
	// The Helm values, ignored unless helmChart is also set
	HelmValuesFrom []HelmValuesFromSource `json:"helmValuesFrom,omitempty"`
}

// PatchOperation represents a patch used to prepare the cluster for a trial run, includes the evaluated
// parameter assignments as necessary
type PatchOperation struct {
	// The reference to the object that the patched should be applied to
	TargetRef corev1.ObjectReference `json:"targetRef"`
	// The patch content type, must be a type supported by the Kubernetes API server
	PatchType types.PatchType `json:"patchType"`
	// The raw data representing the patch to be applied
	Data []byte `json:"data"`
	// The number of remaining attempts to apply the patch, will be automatically set
	// to zero if the patch is successfully applied
	AttemptsRemaining int `json:"attemptsRemaining,omitempty"`
	// Wait for the patched object to stabilize
	Wait bool `json:"wait,omitempty"`
}

// Assignment represents an individual name/value pair. Assignment names must correspond to parameter
// names on the associated experiment.
type Assignment struct {
	// Parameter name being assigned
	Name string `json:"name"`
	// The value of the assignment
	Value int64 `json:"value"`
}

// Value represents an observed metric value after a trial run has completed successfully. Value names
// must correspond to metric names on the associated experiment.
type Value struct {
	// The metric name the value corresponds to
	Name string `json:"name"`
	// The observed float64 value, formatted as a string
	Value string `json:"value"`
	// The observed float64 error (standard deviation), formatted as a string
	Error string `json:"error,omitempty"`
	// The number of remaining attempts to observer the value, will be automatically set
	// to zero if the metric is successfully collected
	AttemptsRemaining int `json:"attemptsRemaining,omitempty"`
	// TODO Initial value captured prior to job execution for local metrics?
}

// TrialConditionType represents the possible observable conditions for a trial
type TrialConditionType string

const (
	// Condition that indicates a successful trial run
	TrialComplete TrialConditionType = "Complete"
	// Condition that indicates a failed trial run
	TrialFailed TrialConditionType = "Failed"
	// Condition that indicates all "create" setup tasks have executed successfully
	TrialSetupCreated TrialConditionType = "SetupCreated"
	// Condition that indicates all "delete" setup tasks have executed successfully
	TrialSetupDeleted TrialConditionType = "SetupDeleted"
	// Condition that indicates patches have been applied for a trial
	TrialPatched TrialConditionType = "Patched"
	// Condition that indicates a trail has stabilized after patches
	TrialStable TrialConditionType = "Stable"
	// Condition that indicates a trial has had metrics collected
	TrialObserved TrialConditionType = "Observed"
)

// TrialCondition represents an observed condition of a trial
type TrialCondition struct {
	// The condition type, e.g. "Complete"
	Type TrialConditionType `json:"type"`
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

// TrialSpec defines the desired state of Trial
type TrialSpec struct {
	// ExperimentRef is the reference to the experiment that contains the definitions to use for this trial,
	// defaults to an experiment in the same namespace with the same name
	ExperimentRef *corev1.ObjectReference `json:"experimentRef,omitempty"`
	// TargetNamespace defines the default namespace of the objects to apply patches to, defaults to the namespace of the trial
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// Assignments are used to patch the cluster state prior to the trial run
	Assignments []Assignment `json:"assignments,omitempty"`
	// Selector matches the job representing the trial run
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Template is the job template used to create trial run jobs
	Template *batchv1beta1.JobTemplateSpec `json:"template,omitempty"`
	// The offset used to adjust the start time to account for spin up of the trial run
	StartTimeOffset *metav1.Duration `json:"startTimeOffset,omitempty"`
	// The approximate amount of time the trial run should execute (not inclusive of the start time offset)
	ApproximateRuntime *metav1.Duration `json:"approximateRuntime,omitempty"`
	// The minimum number of seconds before an attempt should be made to clean up the trial
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// Values are the collected metrics at the end of the trial run
	Values []Value `json:"values,omitempty"`
	// PatchOperations are the patches from the experiment evaluated in the context of this trial
	PatchOperations []PatchOperation `json:"patchOperations,omitempty"`

	// Setup tasks that must run before the trial starts (and possibly after it ends)
	SetupTasks []SetupTask `json:"setupTasks,omitempty"`
	// Volumes to make available to setup tasks, typically ConfigMap backed volumes
	SetupVolumes []corev1.Volume `json:"setupVolumes,omitempty"`
	// Service account name for running setup tasks, needs enough permissions to add and remove software
	SetupServiceAccountName string `json:"setupServiceAccountName,omitempty"`
}

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	// Assignments is a string representation of the trial assignments for reporting purposes
	Assignments string `json:"assignments"`
	// Values is a string representation of the trial values for reporting purposes
	Values string `json:"values"`
	// StartTime is the effective (possibly adjusted) time the trial run job started
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// CompletionTime is the effective (possibly adjusted) time the trial run job completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// Condition is the current state of the trial
	Conditions []TrialCondition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true

// Trial is the Schema for the trials API
// +kubebuilder:printcolumn:name="Assignments",type="string",JSONPath=".status.assignments",description="Current assignments"
// +kubebuilder:printcolumn:name="Values",type="string",JSONPath=".status.values",description="Current values"
type Trial struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior for a trial
	Spec TrialSpec `json:"spec,omitempty"`
	// Current status of a trial
	Status TrialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TrialList contains a list of Trial
type TrialList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// The list of trials
	Items []Trial `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Trial{}, &TrialList{})
}
