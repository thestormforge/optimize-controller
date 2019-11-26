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

	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TODO Make the constant names better reflect the code, not the text
// TODO Use a prefix, like "summary"?
const (
	created      string = "Created"
	setupCreated        = "Setup Created"
	settingUp           = "Setting up"
	setupDeleted        = "Setup Deleted"
	tearingDown         = "Tearing Down"
	patched             = "Patched"
	patching            = "Patching"
	running             = "Running"
	stabilized          = "Stabilized"
	waiting             = "Waiting"
	captured            = "Captured"
	capturing           = "Capturing"
	completed           = "Completed"
	failed              = "Failed"
)

// UpdateStatus will make sure the trial status matches the current state of the trial; returns true only if changes were necessary
func UpdateStatus(t *v1alpha1.Trial) bool {
	summary := summarize(t)
	assignments := assignments(t)
	values := values(t)

	var dirty bool
	if t.Status.Summary != summary {
		t.Status.Summary = summary
		dirty = true
	}
	if t.Status.Assignments != assignments {
		t.Status.Assignments = assignments
		dirty = true
	}
	if t.Status.Values != values {
		t.Status.Values = values
		dirty = true
	}
	return dirty
}

func summarize(t *v1alpha1.Trial) string {
	// If there is an initializer we are in the "setting up" phase
	if t.HasInitializer() {
		return settingUp
	}

	summary := created
	for i := range t.Status.Conditions {
		c := t.Status.Conditions[i]
		switch c.Type {

		case v1alpha1.TrialSetupCreated:
			switch c.Status {
			case corev1.ConditionTrue:
				summary = setupCreated
			case corev1.ConditionFalse:
				summary = settingUp
			case corev1.ConditionUnknown:
				summary = settingUp
			}

		case v1alpha1.TrialSetupDeleted:
			switch c.Status {
			case corev1.ConditionTrue:
				summary = setupDeleted
			case corev1.ConditionFalse:
				summary = tearingDown
			}

		case v1alpha1.TrialPatched:
			switch c.Status {
			case corev1.ConditionTrue:
				summary = patched
			case corev1.ConditionFalse:
				summary = patching
			case corev1.ConditionUnknown:
				summary = patching
			}

		case v1alpha1.TrialStable:
			switch c.Status {
			case corev1.ConditionTrue:
				if t.Status.StartTime != nil {
					summary = running
				} else {
					summary = stabilized
				}
			case corev1.ConditionFalse:
				summary = waiting
			case corev1.ConditionUnknown:
				summary = waiting
			}

		case v1alpha1.TrialObserved:
			switch c.Status {
			case corev1.ConditionTrue:
				summary = captured
			case corev1.ConditionFalse:
				summary = capturing
			case corev1.ConditionUnknown:
				summary = capturing
			}

		case v1alpha1.TrialComplete:
			switch c.Status {
			case corev1.ConditionTrue:
				return completed
			}

		case v1alpha1.TrialFailed:
			switch c.Status {
			case corev1.ConditionTrue:
				return failed
			}
		}
	}
	return summary
}

func assignments(t *v1alpha1.Trial) string {
	assignments := make([]string, len(t.Spec.Assignments))
	for i := range t.Spec.Assignments {
		assignments[i] = fmt.Sprintf("%s=%d", t.Spec.Assignments[i].Name, t.Spec.Assignments[i].Value)
	}
	return strings.Join(assignments, ", ")
}

func values(t *v1alpha1.Trial) string {
	values := make([]string, len(t.Spec.Values))
	for i := range t.Spec.Values {
		if t.Spec.Values[i].AttemptsRemaining == 0 {
			values[i] = fmt.Sprintf("%s=%s", t.Spec.Values[i].Name, t.Spec.Values[i].Value)
		}
	}
	return strings.Join(values, ", ")
}
