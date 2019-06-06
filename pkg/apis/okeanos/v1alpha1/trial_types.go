package v1alpha1

import (
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

type MetricQuery struct {
	Name       string     `json:"name"`
	MetricType MetricType `json:"metricType,omitempty"`
	Query      string     `json:"query"`
	URL        string     `json:"url,omitempty"`
	// TODO ErrorQuery?
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
	ExperimentRef   *corev1.ObjectReference `json:"experimentRef,omitempty"`
	TargetNamespace string                  `json:"targetNamespace"`
	Assignments     map[string]string       `json:"assignments"`
	Values          map[string]string       `json:"values"`
	Selector        *metav1.LabelSelector   `json:"selector,omitempty"`
}

// TODO Should `Assignments` be `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/Unstructured`? CT doesn't have that in known_types.go

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	PatchOperations []PatchOperation `json:"patchOperations,omitempty"`
	MetricQueries   []MetricQuery    `json:"metricQueries,omitempty"`
	StartTime       *metav1.Time     `json:"startTime,omitempty"`
	CompletionTime  *metav1.Time     `json:"completionTime,omitempty"`
	Conditions      []TrialCondition `json:"conditions,omitempty"`
}

// TODO Server side formatting, can display the suggestions and metrics in the default output? We need to format a string field in the status

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
