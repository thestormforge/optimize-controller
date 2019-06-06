package v1alpha1

import (
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type PatchOperation struct {
	TargetRef corev1.ObjectReference `json:"targetRef"`
	PatchType types.PatchType        `json:"patchType"`
	Data      []byte                 `json:"data"`
	Pending   bool                   `json:"pending,omitempty"`
}

type TrialConditionType string

const (
	TrialComplete TrialConditionType = "Complete"
	TrialFailed                      = "Failed"
	// TODO TrialPatched?
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
	// TargetNamespace defines the default namespace of the objects to apply patches to
	TargetNamespace string `json:"targetNamespace"`
	// Assignments are used to patch the cluster state prior to the trial run
	Assignments map[string]string `json:"assignments"`
	// Values are the collected metrics at the end of the trial run
	Values map[string]string `json:"values"`
	// Selector matches the job representing the trial run
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Template is the job template used to create trial run jobs
	Template *batchv1beta1.JobTemplateSpec `json:"jobTemplate"`
	// The offset used to adjust the start time to account for spin up of the trial run
	StartTimeOffset *metav1.Duration `json:"startTimeOffset,omitempty"`
	// The approximate amount of time the trial run should execute (not inclusive of the start time offset)
	ApproximateRuntime *metav1.Duration `json:"approximateRuntime,omitempty"`
}

// TODO What should TargetNamespace default to? The trial namespace or the default namespace?
// TODO Should `Assignments` be `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/Unstructured`? CT doesn't have that in known_types.go

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	PatchOperations []PatchOperation `json:"patchOperations,omitempty"`
	StartTime       *metav1.Time     `json:"startTime,omitempty"`
	CompletionTime  *metav1.Time     `json:"completionTime,omitempty"`
	Conditions      []TrialCondition `json:"conditions,omitempty"`
}

// TODO Server side formatting, can display the suggestions and metrics in the default output? We need to format a string field in the status
// TODO How do we get rid of PatchOperations? Or does it just go on the Spec instead?

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trial is the Schema for the trials API
// +k8s:openapi-gen=true
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
