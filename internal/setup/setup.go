/*
Copyright 2020 GramLabs, Inc.

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

package setup

import (
	"fmt"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ModeCreate is the primary argument to the setup tools container when the task is creating objects
	ModeCreate = "create"
	// ModeDelete is the primary argument to the setup tools container when the task is deleting objects
	ModeDelete = "delete"

	// Initializer is used to paused the trial initialization for setup tasks
	Initializer = "setupInitializer.redskyops.dev"
	// Finalizer is used to prevent the trial deletion for setup tasks
	Finalizer = "setupFinalizer.redskyops.dev"
)

// UpdateStatus returns true if there are setup tasks
func UpdateStatus(t *redskyv1beta1.Trial, probeTime *metav1.Time) bool {
	var needsCreate, needsDelete bool
	for _, task := range t.Spec.SetupTasks {
		needsCreate = needsCreate || !task.SkipCreate
		needsDelete = needsDelete || !task.SkipDelete
	}

	// Short circuit, there are no setup tasks
	if !needsCreate && !needsDelete {
		return false
	}

	// TODO Can we return false from this here as an optimization if both status are True?
	for i := range t.Status.Conditions {
		switch t.Status.Conditions[i].Type {
		case redskyv1beta1.TrialSetupCreated:
			t.Status.Conditions[i].LastProbeTime = *probeTime
			needsCreate = false
		case redskyv1beta1.TrialSetupDeleted:
			t.Status.Conditions[i].LastProbeTime = *probeTime
			needsDelete = false
		}
	}

	if needsCreate {
		t.Status.Conditions = append(t.Status.Conditions, redskyv1beta1.TrialCondition{
			Type:               redskyv1beta1.TrialSetupCreated,
			Status:             corev1.ConditionUnknown,
			LastProbeTime:      *probeTime,
			LastTransitionTime: *probeTime,
		})
	}

	if needsDelete {
		t.Status.Conditions = append(t.Status.Conditions, redskyv1beta1.TrialCondition{
			Type:               redskyv1beta1.TrialSetupDeleted,
			Status:             corev1.ConditionUnknown,
			LastProbeTime:      *probeTime,
			LastTransitionTime: *probeTime,
		})
	}

	// There is at least one setup task
	return true
}

// GetTrialConditionType returns the trial condition type used to report status for the specified job
func GetTrialConditionType(j *batchv1.Job) (redskyv1beta1.TrialConditionType, error) {
	for _, c := range j.Spec.Template.Spec.Containers {
		for _, env := range c.Env {
			if env.Name != "MODE" {
				continue
			}

			switch env.Value {
			case ModeCreate:
				return redskyv1beta1.TrialSetupCreated, nil
			case ModeDelete:
				return redskyv1beta1.TrialSetupDeleted, nil
			default:
				return "", fmt.Errorf("unknown setup job container argument: %s", c.Args[0])
			}
		}
	}
	return "", fmt.Errorf("unable to determine setup job type")
}

// GetConditionStatus returns condition True for a finished job or condition False for an job in progress
func GetConditionStatus(j *batchv1.Job) (corev1.ConditionStatus, string) {
	// Never return "ConditionUnknown", that is reserved to mean "a setup task exists"

	// Check the job conditions first
	for _, c := range j.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			switch c.Type {
			case batchv1.JobComplete:
				return corev1.ConditionTrue, ""
			case batchv1.JobFailed:
				switch c.Reason {
				case "BackoffLimitExceeded":
					// If we hit the backoff limit it means that at least one container is exiting with 1
					return corev1.ConditionTrue, "Setup job did not complete successfully"
				default:
					// Use the condition to construct a message
					m := c.Message
					if m == "" && c.Reason != "" {
						m = fmt.Sprintf("Setup job failed with reason '%s'", c.Reason)
					}
					if m == "" {
						m = "Setup job failed without reporting a reason"
					}
					return corev1.ConditionTrue, m
				}
			}
		}
	}

	// For versions of Kube that do not report failures as conditions, just look for failed pods
	if j.Status.Failed > 0 {
		return corev1.ConditionTrue, fmt.Sprintf("Setup job has %d failed pod(s)", j.Status.Failed)
	}

	return corev1.ConditionFalse, ""
}
