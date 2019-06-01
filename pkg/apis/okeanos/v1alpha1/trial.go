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

// Manually write the deep copy method because of the empty interface usage

func (in *Assignments) DeepCopy() *Assignments {
	if in == nil {
		return nil
	}
	out := Assignments(make(map[string]interface{}, len(*in)))
	for key, val := range *in {
		out[key] = val
	}
	return &out
}

// Returns a fall back label for when the user has not specified anything
func (in *Trial) GetDefaultLabels() map[string]string {
	return map[string]string{"trial": in.Name}
}
