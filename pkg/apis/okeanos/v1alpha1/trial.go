package v1alpha1

import (
	"k8s.io/apimachinery/pkg/types"
)

// ExperimentNamespacedName returns the namespaced name of the experiment for this trial
func (in *Trial) ExperimentNamespacedName() types.NamespacedName {
	nn := types.NamespacedName{Namespace: in.Namespace, Name: in.Name}
	if in.Spec.ExperimentRef != nil {
		if in.Spec.ExperimentRef.Namespace != "" {
			nn.Namespace = in.Spec.ExperimentRef.Namespace
		}
		if in.Spec.ExperimentRef.Name != "" {
			nn.Name = in.Spec.ExperimentRef.Name
		}
	}
	return nn
}

// Returns a fall back label for when the user has not specified anything
func (in *Trial) GetDefaultLabels() map[string]string {
	return map[string]string{"trial": in.Name}
}
