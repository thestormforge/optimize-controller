package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// GetSelfReference returns an object reference to this experiment
func (in *Experiment) GetSelfReference() *corev1.ObjectReference {
	if in == nil {
		return nil
	}
	// TODO Is there a standard helper somewhere that does this?
	return &corev1.ObjectReference{
		Kind:       in.TypeMeta.Kind,
		Name:       in.GetName(),
		Namespace:  in.GetNamespace(),
		APIVersion: in.TypeMeta.APIVersion,
	}
}

// GetReplicas returns the effective replica (trial) count for the experiment
func (in *Experiment) GetReplicas() int {
	if in == nil || in.DeletionTimestamp != nil {
		return 0
	}
	if in.Spec.Replicas != nil {
		return int(*in.Spec.Replicas)
	}
	return 1
}

// SetReplicas establishes a new replica (trial) count for the experiment
func (in *Experiment) SetReplicas(r int) {
	if in != nil {
		replicas := int32(r)
		if replicas < 0 {
			replicas = 0
		}
		in.Spec.Replicas = &replicas
	}
}

// Returns a fall back label for when the user has not specified anything
func (in *Experiment) GetDefaultLabels() map[string]string {
	return map[string]string{"experiment": in.Name}
}
