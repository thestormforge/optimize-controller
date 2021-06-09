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

package v1beta2

import (
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Assignment represents an individual name/value pair. Assignment names must correspond to parameter
// names on the associated experiment.
type Assignment struct {
	// Name of the parameter being assigned
	Name string `json:"name"`
	// Value of the assignment
	Value intstr.IntOrString `json:"value"`
}

// TrialReadinessGate represents a readiness check on one or more objects that must pass after patches
// have been applied, but before the trial run job can start
type TrialReadinessGate struct {
	// Kind of the readiness target
	Kind string `json:"kind,omitempty"`
	// Name of the readiness target, mutually exclusive with "Selector"
	Name string `json:"name,omitempty"`
	// APIVersion of the readiness target
	APIVersion string `json:"apiVersion,omitempty"`
	// Selector matches the resources whose condition must be checked, mutually exclusive with "Name"
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// ConditionTypes are the status conditions that must be "True"
	ConditionTypes []string `json:"conditionTypes,omitempty"`
	// InitialDelaySeconds is the approximate number of seconds after all of the patches have been applied to start
	// evaluating this check
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// PeriodSeconds is the approximate amount of time in between evaluation attempts of this check;
	// defaults to 10 seconds, minimum value is 1 second
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// FailureThreshold is number of times that any of the specified ready conditions may be "False";
	// defaults to 3, minimum value is 1
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

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

// HelmValuesFromSource represents a source of a values mapping
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
	// Override the default command for the container
	Command []string `json:"command,omitempty"`
	// Override the default args for the container
	Args []string `json:"args,omitempty"`
	// Flag to indicate the creation part of the task can be skipped
	SkipCreate bool `json:"skipCreate,omitempty"`
	// Flag to indicate the deletion part of the task can be skipped
	SkipDelete bool `json:"skipDelete,omitempty"`
	// Volume mounts for the setup task
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// The Helm chart reference to release as part of this task
	HelmChart string `json:"helmChart,omitempty"`
	// The Helm chart version, empty means use the latest
	HelmChartVersion string `json:"helmChartVersion,omitempty"`
	// The Helm values to set, ignored unless helmChart is also set
	HelmValues []HelmValue `json:"helmValues,omitempty"`
	// The Helm values, ignored unless helmChart is also set
	HelmValuesFrom []HelmValuesFromSource `json:"helmValuesFrom,omitempty"`
	// The Helm repository to fetch the chart from
	HelmRepository string `json:"helmRepository,omitempty"`
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
}

// ReadinessCheck represents a check to determine when the patched application is "ready" and it is
// safe to start the trial run job
type ReadinessCheck struct {
	// TargetRef is the reference to the object to test the readiness of
	TargetRef corev1.ObjectReference `json:"targetRef"`
	// Selector may be used to trigger a search for multiple related objects to search; this may have RBAC implications,
	// in particular "list" permissions are required
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// ConditionTypes are the status conditions that must be "True"; in addition to conditions that appear in the
	// status of the target object, additional special conditions starting with "stormforge.io/" can be tested
	ConditionTypes []string `json:"conditionTypes,omitempty"`
	// InitialDelaySeconds is the approximate number of seconds after all of the patches have been applied to start
	// evaluating this check
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// PeriodSeconds is the approximate amount of time in between evaluation attempts of this check
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// AttemptsRemaining is the number of failed attempts to allow before marking the entire trial as failed, will be
	// automatically set to zero if the check has been successfully evaluated
	AttemptsRemaining int32 `json:"attemptsRemaining,omitempty"`
	// LastCheckTime is the timestamp of the last evaluation attempt
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
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
}

// TrialConditionType represents the possible observable conditions for a trial
type TrialConditionType string

const (
	// TrialComplete is a condition that indicates a successful trial run
	TrialComplete TrialConditionType = "stormforge.io/trial-complete"
	// TrialFailed is a condition that indicates a failed trial run
	TrialFailed TrialConditionType = "stormforge.io/trial-failed"
	// TrialSetupCreated is a condition that indicates all "create" setup tasks have finished
	TrialSetupCreated TrialConditionType = "stormforge.io/trial-setup-created"
	// TrialSetupDeleted is a condition that indicates all "delete" setup tasks have finished
	TrialSetupDeleted TrialConditionType = "stormforge.io/trial-setup-deleted"
	// TrialPatched is a condition that indicates patches have been applied for a trial
	TrialPatched TrialConditionType = "stormforge.io/trial-patched"
	// TrialReady is a condition that indicates the application is ready after patches were applied
	TrialReady TrialConditionType = "stormforge.io/trial-ready"
	// TrialObserved is a condition that indicates a trial has had metrics collected
	TrialObserved TrialConditionType = "stormforge.io/trial-observed"
)

// TrialCondition represents an observed condition of a trial
type TrialCondition struct {
	// The condition type, e.g. "stormforge.io/trial-complete"
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
	// Assignments are used to patch the cluster state prior to the trial run
	Assignments []Assignment `json:"assignments,omitempty"`
	// Selector matches the job representing the trial run
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// JobTemplate is the job template used to create trial run jobs
	JobTemplate *batchv1beta1.JobTemplateSpec `json:"jobTemplate,omitempty"`
	// InitialDelaySeconds is number of seconds to wait after a trial becomes ready before starting the trial run job
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// The offset used to adjust the start time to account for spin up of the trial run
	StartTimeOffset *metav1.Duration `json:"startTimeOffset,omitempty"`
	// The approximate amount of time the trial run should execute (not inclusive of the start time offset)
	ApproximateRuntime *metav1.Duration `json:"approximateRuntime,omitempty"`
	// The minimum number of seconds before an attempt should be made to clean up the trial, if unset or negative no attempt is made to clean up the trial
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
	// The minimum number of seconds before an attempt should be made to clean up a failed trial, defaults to TTLSecondsAfterFinished
	TTLSecondsAfterFailure *int32 `json:"ttlSecondsAfterFailure,omitempty"`
	// The readiness gates to check before running the trial job
	ReadinessGates []TrialReadinessGate `json:"readinessGates,omitempty"`

	// Values are the collected metrics at the end of the trial run
	Values []Value `json:"values,omitempty"`

	// Setup tasks that must run before the trial starts (and possibly after it ends)
	SetupTasks []SetupTask `json:"setupTasks,omitempty"`
	// Volumes to make available to setup tasks, typically ConfigMap backed volumes
	SetupVolumes []corev1.Volume `json:"setupVolumes,omitempty"`
	// Service account name for running setup tasks, needs enough permissions to add and remove software
	SetupServiceAccountName string `json:"setupServiceAccountName,omitempty"`
	// Cluster role name to be assigned to the setup service account when creating namespaces
	SetupDefaultClusterRole string `json:"setupDefaultClusterRole,omitempty"`
	// Policy rules to be assigned to the setup service account when creating namespaces
	SetupDefaultRules []rbacv1.PolicyRule `json:"setupDefaultRules,omitempty"`
}

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	// Phase is a brief human readable description of the trial status
	Phase string `json:"phase"`
	// Assignments is a string representation of the trial assignments for reporting purposes
	Assignments string `json:"assignments"`
	// Values is a string representation of the trial values for reporting purposes
	Values string `json:"values"`
	// StartTime is the effective (possibly adjusted) time the trial run job started
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// CompletionTime is the effective (possibly adjusted) time the trial run job completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// Conditions is the current state of the trial
	Conditions []TrialCondition `json:"conditions,omitempty"`
	// PatchOperations are the patches from the experiment evaluated in the context of this trial
	PatchOperations []PatchOperation `json:"patchOperations,omitempty"`
	// ReadinessChecks are the all of the objects whose conditions need to be inspected for this trial
	ReadinessChecks []ReadinessCheck `json:"readinessChecks,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// Trial is the Schema for the trials API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Trial status"
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
