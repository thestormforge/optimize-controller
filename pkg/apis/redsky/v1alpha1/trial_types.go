package v1alpha1

import (
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// HelmValue represents a value in a Helm template
type HelmValue struct {
	// The name of Helm value as passed to one of the set options
	Name string `json:"name"`
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
	// TODO String bool to force usage of --set-string?
}

// HelmValueFromSource represents a source of a values mapping
type HelmValuesFromSource struct {
	ConfigMap *ConfigMapHelmValuesFromSource `json:"configMap"`
	// TODO Secret support?
}

// ConfigMapHelmValuesFromSource is a reference to a ConfigMap that contains "*values.yaml" keys
// TODO How do document the side effect of things like patches in the ConfigMap also being applied?
type ConfigMapHelmValuesFromSource struct {
	corev1.LocalObjectReference `json:",inline"`
}

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

type PatchOperation struct {
	TargetRef         corev1.ObjectReference `json:"targetRef"`
	PatchType         types.PatchType        `json:"patchType"`
	Data              []byte                 `json:"data"`
	AttemptsRemaining int                    `json:"attemptsRemaining,omitempty"`
}

type Assignment struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type Value struct {
	Name              string `json:"name"`
	Value             string `json:"value"`           // float64
	Error             string `json:"error,omitempty"` // float64
	AttemptsRemaining int    `json:"attemptsRemaining,omitempty"`
	// TODO Initial value captured prior to job execution for local metrics?
}

type TrialConditionType string

const (
	TrialComplete TrialConditionType = "Complete"
	TrialFailed                      = "Failed"
	// TODO TrialPatched? TrialStable?
)

type TrialCondition struct {
	Type               TrialConditionType     `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trial is the Schema for the trials API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Assignments",type="string",JSONPath=".status.assignments",description="Current assignments"
// +kubebuilder:printcolumn:name="Values",type="string",JSONPath=".status.values",description="Current values"
type Trial struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrialSpec   `json:"spec,omitempty"`
	Status TrialStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TrialList contains a list of Trial
type TrialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trial `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Trial{}, &TrialList{})
}
