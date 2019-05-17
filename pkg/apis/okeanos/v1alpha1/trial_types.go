package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Assignments is a type alias for a generic string to value map
type Assignments map[string]interface{}

type PatchOperation struct {
	TargetRef corev1.ObjectReference `json:"targetRef"`
	PatchType types.PatchType        `json:"patchType"`
	Data      []byte                 `json:"data"`
	Pending   bool                   `json:"pending,omitempty"`
}

type MetricQuery struct {
	Name       string `json:"name"`
	MetricType string `json:"metricType,omitempty"`
	Query      string `json:"query"`
	URL        string `json:"url,omitempty"`
}

// TrialSpec defines the desired state of Trial
type TrialSpec struct {
	ExperimentRef *corev1.ObjectReference `json:"experimentRef,omitempty"` // Defaults to experiment with same name
	Assignments   Assignments             `json:"assignments"`
	Values        map[string]float64      `json:"values"`
	Selector      *metav1.LabelSelector   `json:"selector,omitempty"`
}

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	PatchOperations []PatchOperation `json:"patchOperations,omitempty"`
	MetricQueries   []MetricQuery    `json:"metricQueries,omitempty"`
	StartTime       *metav1.Time     `json:"startTime,omitempty"`
	CompletionTime  *metav1.Time     `json:"completionTime,omitempty"`
	Failed          bool             `json:"failed,omitempty"` // Can be true without a job if patches or waits fail
}

// TODO Server side formatting, can display the suggestions and metrics in the default output?
// TODO Trial conditions: Patched, Wait?, Complete, Failed

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
