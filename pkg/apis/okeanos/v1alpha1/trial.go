package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
)

// MergeFromJob merges the job status into the trial status
func (in *TrialStatus) MergeFromJob(j *batchv1.JobStatus) bool {
	var dirty bool

	if in.Start == nil {
		// Establish a start time if available
		in.Start = j.StartTime // TODO DeepCopy?
		dirty = j.StartTime != nil
	} else if j.StartTime != nil && j.StartTime.Before(in.Start) {
		// Move the start time up
		in.Start = j.StartTime
		dirty = true
	}

	if in.End == nil {
		// Establish an end time if available
		in.End = j.CompletionTime
		dirty = j.CompletionTime != nil
	} else if j.CompletionTime != nil && in.End.Before(j.CompletionTime) {
		// Move the start time back
		in.End = j.CompletionTime
		dirty = true
	}

	return dirty
}
