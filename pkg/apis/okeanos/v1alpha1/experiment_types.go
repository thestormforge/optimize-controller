package v1alpha1

import (
	okeanosclient "github.com/gramLabs/okeanos/pkg/apis/okeanos/client"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Parameter
type Parameter struct {
	Name   string   `json:"name"`
	Values []string `json:"values,omitempty"`
	Min    int      `json:"min,omitempty"` // TODO Not json.Number, but what else can be either float or int?
	Max    int      `json:"max,omitempty"`
}

// Metric
type Metric struct {
	Name     string                `json:"name"`
	Minimize bool                  `json:"minimize,omitempty"`
	Type     string                `json:"type,omitempty"` // "prometheus" or "jsonpath"
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

// TODO This whole templating thing is a mess

// Trial template is the user configurable part of the trial
type TrialTemplate struct {
	Selector *metav1.LabelSelector         `json:"selector,omitempty"`
	Template *batchv1beta1.JobTemplateSpec `json:"template"`
}

// TrialTemplateSpec is used as a template for creating new trials
type TrialTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TrialTemplate `json:"spec"`
}

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// Parameters defines the search space for the experiment
	Parameters []Parameter `json:"parameters,omitempty"`
	// Metrics defines the outcomes for the experiment
	Metrics []Metric `json:"metrics,omitempty"`
	// Configuration defines the optimization specific configuration options
	Configuration okeanosclient.Configuration `json:"configuration,omitempty"`
	// RemoteURL is a reference to the experiment on the remote optimization service. If left blank an attempt will be
	// made to generate a value and the corresponding remote resource.
	RemoteURL string `json:"remoteURL,omitempty"`
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
}

// TODO The remote URL should be replaced with a service so we can reference a cluster IP, and that should proxy in the actual remote

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	SuggestionURL string `json:"suggestionURL,omitempty"`
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
