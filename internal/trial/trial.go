/*
Copyright 2019 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trial

import (
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsFinished checks to see if the specified trial is finished
func IsFinished(t *redskyv1alpha1.Trial) bool {
	for _, c := range t.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			if c.Type == redskyv1alpha1.TrialComplete || c.Type == redskyv1alpha1.TrialFailed {
				return true
			}
		}
	}
	return false
}

// IsActive checks to see if the specified trial and any setup delete tasks are NOT finished
func IsActive(t *redskyv1alpha1.Trial) bool {
	// Not finished, definitely active
	if !IsFinished(t) {
		return true
	}

	// Check if a setup delete task exists and has not yet completed (remember the TrialSetupDeleted status is optional!)
	for _, c := range t.Status.Conditions {
		if c.Type == redskyv1alpha1.TrialSetupDeleted && c.Status != corev1.ConditionTrue {
			return true
		}
	}

	// We do not have a condition indicating that the trial has been reported so we need to check
	// for the presence of a reporting URL (which will be removed once the trial has been reported)
	if t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL] != "" {
		return true
	}

	return false
}

// NeedsCleanup checks to see if a trial's TTL has expired
func NeedsCleanup(t *redskyv1alpha1.Trial) bool {
	// Already deleted or still active, no cleanup necessary
	if !t.GetDeletionTimestamp().IsZero() || IsActive(t) {
		return false
	}

	// Try to determine effective finish time and TTL
	finishTime := metav1.Time{}
	ttlSeconds := t.Spec.TTLSecondsAfterFinished
	for _, c := range t.Status.Conditions {
		if isFinishTimeCondition(&c) {
			// Adjust the TTL if specified separately for failures
			if c.Type == redskyv1alpha1.TrialFailed && t.Spec.TTLSecondsAfterFailure != nil {
				ttlSeconds = t.Spec.TTLSecondsAfterFailure
			}

			// Take the latest time possible
			if finishTime.Before(&c.LastTransitionTime) {
				finishTime = c.LastTransitionTime
			}
		}
	}

	// No finish time or TTL, no cleanup necessary
	if finishTime.IsZero() || ttlSeconds == nil || *ttlSeconds < 0 {
		return false
	}

	// Check to see if we are still in the TTL window
	ttl := time.Duration(*ttlSeconds) * time.Second
	return finishTime.UTC().Add(ttl).Before(time.Now().UTC())
}

// isFinishTimeCondition returns true if the condition is relevant to the "finish time"
func isFinishTimeCondition(c *redskyv1alpha1.TrialCondition) bool {
	switch c.Type {
	case redskyv1alpha1.TrialComplete, redskyv1alpha1.TrialFailed, redskyv1alpha1.TrialSetupDeleted:
		return c.Status == corev1.ConditionTrue
	default:
		return false
	}
}

// CheckAssignments ensures the trial assignments match the definitions on the experiment
func CheckAssignments(t *redskyv1alpha1.Trial, exp *redskyv1alpha1.Experiment) error {
	err := &AssignmentError{}

	// Index the assignments, checking for duplicates
	assignments := make(map[string]int64, len(t.Spec.Assignments))
	for _, a := range t.Spec.Assignments {
		if _, ok := assignments[a.Name]; !ok {
			assignments[a.Name] = a.Value
		} else {
			err.Duplicated = append(err.Duplicated, a.Name)
		}
	}

	// Verify against the parameter specifications
	for _, p := range exp.Spec.Parameters {
		if a, ok := assignments[p.Name]; ok {
			if a < p.Min || a > p.Max {
				err.OutOfBounds = append(err.OutOfBounds, p.Name)
			}
			delete(assignments, p.Name)
		} else {
			err.Unassigned = append(err.Unassigned, p.Name)
		}
	}
	for n := range assignments {
		err.Undefined = append(err.Undefined, n)
	}

	// If there were no problems found, return nil
	if len(err.Unassigned) == 0 && len(err.Undefined) == 0 && len(err.OutOfBounds) == 0 && len(err.Duplicated) == 0 {
		return nil
	}
	return err
}

// AssignmentError is raised when trial assignments do not match the experiment parameter definitions
type AssignmentError struct {
	// Parameter names for which the assignment is missing
	Unassigned []string
	// Parameter names for which there is no definition
	Undefined []string
	// Parameter names for which the assignment is out of bounds
	OutOfBounds []string
	// Parameter names for which multiple assignments exist
	Duplicated []string
}

// Error returns a message describing the nature of the problems with the assignments
func (e *AssignmentError) Error() string {
	// TODO Improve this error message
	return "invalid assignments"
}
