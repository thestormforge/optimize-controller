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
	"testing"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func checkPhase(t *testing.T, status *redskyv1alpha1.TrialStatus, expected string) {
	tt := &redskyv1alpha1.Trial{Status: *status}
	UpdateStatus(tt)
	actual := tt.Status.Phase
	if actual != expected {
		t.Errorf("incorrect phase: %s (expecting %s)", actual, expected)
	}
}

func TestUpdateStatusSummary(t *testing.T) {
	status := &redskyv1alpha1.TrialStatus{}

	// Initial state, no status
	status.Conditions = nil
	checkPhase(t, status, created)

	// First setup probe with at least one setup task
	status.Conditions = nil
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupCreated,
		Status: corev1.ConditionUnknown,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupDeleted,
		Status: corev1.ConditionUnknown,
	})
	checkPhase(t, status, settingUp)

	// Setup create task is running
	status.Conditions = nil
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupCreated,
		Status: corev1.ConditionFalse,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupDeleted,
		Status: corev1.ConditionUnknown,
	})
	checkPhase(t, status, settingUp)

	// Setup create task is running
	status.Conditions = nil
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupCreated,
		Status: corev1.ConditionTrue,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupDeleted,
		Status: corev1.ConditionUnknown,
	})
	checkPhase(t, status, setupCreated)

	// Setup create task never started
	status.Conditions = nil
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupCreated,
		Status: corev1.ConditionFalse,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupDeleted,
		Status: corev1.ConditionUnknown,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialFailed,
		Status: corev1.ConditionTrue,
	})
	checkPhase(t, status, failed)

	// Setup create task just failed
	status.Conditions = nil
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupCreated,
		Status: corev1.ConditionTrue,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialSetupDeleted,
		Status: corev1.ConditionUnknown,
	}, redskyv1alpha1.TrialCondition{
		Type:   redskyv1alpha1.TrialFailed,
		Status: corev1.ConditionTrue,
	})
	checkPhase(t, status, failed)
}
