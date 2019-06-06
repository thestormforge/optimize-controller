package v1alpha1

import (
	okeanosclient "github.com/gramLabs/okeanos/pkg/api/okeanos/v1alpha1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Parameter
type Parameter struct {
	Name string `json:"name"`
	Min  int64  `json:"min,omitempty"`
	Max  int64  `json:"max,omitempty"`
}

type MetricType string

const (
	MetricLocal      MetricType = "local"
	MetricPrometheus            = "prometheus"
	MetricJSONPath              = "jsonpath"
	// TODO "regex"?
)

// Metric
type Metric struct {
	Name     string                `json:"name"`
	Minimize bool                  `json:"minimize,omitempty"`
	Type     MetricType            `json:"type,omitempty"`
	Query    string                `json:"query"`          // Type specific query, e.g. PromQL or a JSON pointer expression
	Path     string                `json:"path,omitempty"` // Path appended to the endpoint (used as a prefix for prometheus)
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	Port     intstr.IntOrString    `json:"endpoint,omitempty"`
}

// PatchTemplate defines a target resource and a patch template to apply
type PatchTemplate struct {
	Type      string                 `json:"type"`
	Patch     string                 `json:"patch"`
	TargetRef corev1.ObjectReference `json:"targetRef"`
	Selector  *metav1.LabelSelector  `json:"selector,omitempty"`
}

// TrialTemplateSpec is used as a template for creating new trials
type TrialTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TrialSpec `json:"spec"`
}

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// Parameters defines the search space for the experiment
	Parameters []Parameter `json:"parameters,omitempty"`
	// Metrics defines the outcomes for the experiment
	Metrics []Metric `json:"metrics,omitempty"`
	// Optimization defines the optimization specific configuration options
	Optimization okeanosclient.Optimization `json:"optimization,omitempty"`
	// Replicas is the number of trials to execute at once. It must be no greater then the parallelism defined in the
	// optimization configuration (which will be used as a default if replicas is left unspecified). When running an
	// experiment in multiple clusters, the sum of all the replica counts should be used as the parallelism.
	Replicas *int32 `json:"replicas,omitempty"`
	// Selector locates trial resources that are part of this experiment
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// NamespaceSelector is used to determine which namespaces on a cluster can be used to create trials. Only a single
	// trial can be created in each namespace so if there are fewer matching namespaces then replicas, no trials will
	// be created.
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// Template for create a new trial. The resulting trial must be matched by Selector. If the template can provide an
	// initial namespace, however other namespaces (matched by NamespaceSelector) will be used if the effective
	// replica count is more then one
	Template TrialTemplateSpec `json:"template"`
	// Patches is a sequence of templates written against the experiment parameters that will be used to put the
	// cluster into the desired state
	Patches []PatchTemplate `json:"patches,omitempty"`
	// JobTemplate is the template used to create trial run jobs
	JobTemplate *batchv1beta1.JobTemplateSpec `json:"jobTemplate"`
	// The offset used to adjust the start time to account ignore spin up of the trial run
	StartTimeOffset *metav1.Duration `json:"startTimeOffset,omitempty"`
	// The approximate amount of time the trial run should execute (not inclusive of the start time offset)
	ApproximateRuntime *metav1.Duration `json:"approximateRuntime,omitempty"`
}

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Experiment is the Schema for the experiments API
// +k8s:openapi-gen=true
type Experiment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExperimentSpec   `json:"spec,omitempty"`
	Status ExperimentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExperimentList contains a list of Experiment
type ExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Experiment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Experiment{}, &ExperimentList{})
}
