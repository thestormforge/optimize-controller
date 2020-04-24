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
	"fmt"
	"strings"
	"time"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
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

// IsAbandoned checks to see if the specified trial is abandoned
func IsAbandoned(t *redskyv1alpha1.Trial) bool {
	return !IsFinished(t) && !t.GetDeletionTimestamp().IsZero()
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

	return false
}

// IsTrialJobReference checks to see if the supplied reference likely points to the job of a trial. This is
// used primarily to give special handling to patch operations so they can refer to trial job before it exists.
func IsTrialJobReference(t *redskyv1alpha1.Trial, ref *corev1.ObjectReference) bool {
	// Kind _must_ be job
	if ref.Kind != "Job" {
		return false
	}

	// Allow version to be omitted for compatibility with old job definitions
	if ref.APIVersion != "" && ref.APIVersion != "batch/v1" {
		return false
	}

	// Allow namespace to be omitted for trials that run in multiple namespaces
	if ref.Namespace != "" && ref.Namespace != t.Namespace {
		return false
	}

	// If the trial job template has name, it must match...
	if t.Spec.Template != nil && t.Spec.Template.Name != "" {
		return t.Spec.Template.Name != ref.Name
	}

	// ...otherwise the trial name must match by prefix
	if !strings.HasPrefix(t.Name, ref.Name) {
		return false
	}

	return true
}

// AppendAssignmentEnv appends an environment variable for each trial assignment
func AppendAssignmentEnv(t *redskyv1alpha1.Trial, env []corev1.EnvVar) []corev1.EnvVar {
	for _, a := range t.Spec.Assignments {
		name := strings.ReplaceAll(strings.ToUpper(a.Name), ".", "_")
		env = append(env, corev1.EnvVar{Name: name, Value: fmt.Sprintf("%d", a.Value)})
	}
	return env
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
