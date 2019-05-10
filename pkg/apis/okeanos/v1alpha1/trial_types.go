package v1alpha1

import (
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Patch represents a required change to the cluster state that has not yet been made
type Patch struct {
	PatchType types.PatchType        `json:"patchType"`
	Data      []byte                 `json:"data"`
	Reference corev1.ObjectReference `json:"keys"`
}

// MetricQuery represents the retrieval of a metric value from a specific service within the cluster
type MetricQuery struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	Query string `json:"query"`
}

// TrialSpec defines the desired state of Trial
type TrialSpec struct {
	RemoteURL string                        `json:"remoteURL"`
	Patches   []Patch                       `json:"patches,omitempty"`
	Queries   []MetricQuery                 `json:"queries,omitempty"`
	Selector  *metav1.LabelSelector         `json:"selector,omitempty"`
	Template  *batchv1beta1.JobTemplateSpec `json:"template"`
	Metrics   map[string]string             `json:"metrics,omitempty"` // TODO This should be generic, not strings...
	Failed    bool                          `json:"failed,omitempty"`
}

// TrialStatus defines the observed state of Trial
type TrialStatus struct {
	Suggestions map[string]string        `json:"suggestions,omitempty"` // TODO This should be generic not strings...
	Patched     []corev1.ObjectReference `json:"patched,omitempty"`
	Start       *metav1.Time             `json:"start,omitempty"`
	End         *metav1.Time             `json:"end,omitempty"`
}

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
