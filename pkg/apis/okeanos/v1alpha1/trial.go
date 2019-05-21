package v1alpha1

import (
	"k8s.io/apimachinery/pkg/types"
)

// ExperimentNamespacedName returns the namespaced name of the experiment for this trial
func (in *Trial) ExperimentNamespacedName() types.NamespacedName {
	name := in.Name
	namespace := in.Namespace

	if in.Spec.ExperimentRef != nil && in.Spec.ExperimentRef.Name != "" {
		name = in.Spec.ExperimentRef.Name
	}
	if in.Spec.ExperimentRef != nil && in.Spec.ExperimentRef.Namespace != "" {
		namespace = in.Spec.ExperimentRef.Namespace
	}

	return types.NamespacedName{Name: name, Namespace: namespace}
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
