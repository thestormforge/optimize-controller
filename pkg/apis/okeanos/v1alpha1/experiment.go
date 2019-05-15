package v1alpha1

import (
	"github.com/gramLabs/okeanos/pkg/apis/okeanos/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CopyToRemote overwrites the state of the supplied (presumably empty) remote experiment representation with the data
// stored in the Kubernetes API. This is primarily intended to be used when creating remote resources.
func (in *Experiment) CopyToRemote(e *client.Experiment) {
	e.Configuration = in.Spec.Configuration

	e.Parameters = nil
	for _, p := range in.Spec.Parameters {
		e.Parameters = append(e.Parameters, client.Parameter{
			Name:   p.Name,
			Values: p.Values,
			// TODO Min/Max config?
		})
	}

	e.Metrics = nil
	for _, m := range in.Spec.Metrics {
		e.Metrics = append(e.Metrics, client.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
		})
	}

	return
}

// GetReplicas returns the effective replica (trial) count for the experiment. The number of replicas is bound by
// the optimization's parallelism configuration and may be zero to indicate the experiment is paused or complete.
func (in *Experiment) GetReplicas() int {
	if in.Spec.Replicas != nil && (*in.Spec.Replicas == 1 || *in.Spec.Replicas <= in.Spec.Configuration.Parallelism) {
		return int(*in.Spec.Replicas)
	}
	if in.Spec.Configuration.Parallelism > 0 {
		return int(in.Spec.Configuration.Parallelism)
	}
	return 1
}

// SetReplicas establishes a new replica (trial) count for the experiment. The value is adjusted to ensure it remains
// between 0 and the configured parallelism (inclusive).
func (in *Experiment) SetReplicas(r int) {
	replicas := int32(r)
	if replicas < 0 {
		replicas = 0
	}
	if in.Spec.Configuration.Parallelism > 0 && replicas > in.Spec.Configuration.Parallelism {
		replicas = in.Spec.Configuration.Parallelism
	}
	in.Spec.Replicas = &replicas
}

func (in *Experiment) ensureLabels() bool {
	if l, ok := in.Spec.Template.Labels["experiment"]; ok && l != "" {
		return false
	}

	in.Spec.Template.Labels["experiment"] = in.Name

	return true
}

func (in *Experiment) ensureSelector() bool {
	if in.Spec.Selector != nil {
		return false
	}

	in.Spec.Selector = metav1.SetAsLabelSelector(in.Spec.Template.Labels)

	return true
}
