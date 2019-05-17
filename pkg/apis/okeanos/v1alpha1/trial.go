package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
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

// MergeFromJob merges the job status into the trial status
func (in *TrialStatus) MergeFromJob(j *batchv1.JobStatus) bool {
	var dirty bool

	if in.StartTime == nil {
		// Establish a start time if available
		in.StartTime = j.StartTime // TODO DeepCopy?
		dirty = dirty || j.StartTime != nil
	} else if j.StartTime != nil && j.StartTime.Before(in.StartTime) {
		// Move the start time up
		in.StartTime = j.StartTime
		dirty = true
	}

	if in.CompletionTime == nil {
		// Establish an end time if available
		in.CompletionTime = j.CompletionTime
		dirty = dirty || j.CompletionTime != nil
	} else if j.CompletionTime != nil && in.CompletionTime.Before(j.CompletionTime) {
		// Move the start time back
		in.CompletionTime = j.CompletionTime
		dirty = true
	}

	return dirty
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
