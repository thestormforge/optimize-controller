package v1alpha1

import (
	"github.com/gramLabs/okeanos/pkg/apis/okeanos/client"
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

// EnsureReplicas makes sure the replicas value is explicitly set to a valid value. If omitted, replicas will be set
// to the current parallelism configuration, if both values are omitted, a default of 1 is assumed.
func (in *Experiment) EnsureReplicas() bool {
	// If there is an explicit replica count that does not exceed the minimal parallelism we are done
	if in.Spec.Replicas != nil && (*in.Spec.Replicas == 1 || *in.Spec.Replicas <= in.Spec.Configuration.Parallelism) {
		return false
	}

	replicas := int32(1)
	if in.Spec.Configuration.Parallelism > 0 {
		replicas = in.Spec.Configuration.Parallelism
	}
	in.Spec.Replicas = &replicas
	return true
}
