package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Values map[string]interface{}

type Outcomes map[string]interface{}

type PatchOperation struct {
	TargetRef corev1.ObjectReference `json:"targetRef"`
	PatchType types.PatchType        `json:"patchType"`
	Data      []byte                 `json:"data"`
	Pending   bool                   `json:"pending,omitempty"`
}

// Pending is a stretch to be in the status, technically one can determine it's value by looking at cluster state
// and confirming re-application of the patch results in a different state

type MetricQuery struct {
	Name       string `json:"name"`
	MetricType string `json:"metricType,omitempty"`
	Query      string `json:"query"`
	URL        string `json:"url,omitempty"`
}

// TrialSpec defines the desired state of Trial
type TrialSpec struct {
	ExperimentRef *corev1.ObjectReference `json:"experimentRef,omitempty"` // Defaults to experiment with same name
	Suggestions   Values                  `json:"suggestions"`
	Metrics       Outcomes                `json:"metrics"`
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
