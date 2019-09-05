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
package status

import (
	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ExperimentSummary is text description summarizing the entire status of an experiment
type ExperimentSummary string

// TrialSummary is text description summarizing the entire status of a trial
type TrialSummary string

// TODO Make the constant names better reflect the code, not the text
const (
	ExperimentCreated ExperimentSummary = "Created"
	ExperimentPaused                    = "Paused"
	ExperimentEmpty                     = "Never run" // TODO This is misleading, it could be that we already deleted the trials that ran
	ExperimentIdle                      = "Idle"
	ExperimentRunning                   = "Running"

	TrialCreated      TrialSummary = "Created"
	TrialSetupCreated              = "Setup Created"
	TrialSettingUp                 = "Setting up"
	TrialSetupDeleted              = "Setup Deleted"
	TrialTearingDown               = "Tearing Down"
	TrialPatched                   = "Patched"
	TrialPatching                  = "Patching"
	TrialRunning                   = "Running"
	TrialStabilized                = "Stabilized"
	TrailWaiting                   = "Waiting"
	TrialCaptured                  = "Captured"
	TrialCapturing                 = "Capturing"
	TrialCompleted                 = "Completed"
	TrialFailed                    = "Failed"
)

// ExperimentStatusSummary is a summary of the resource status (which doesn't make sense since the resource status is an empty struct)
type ExperimentStatusSummary struct {
	Status         ExperimentSummary `json:"status"`
	CompletedCount int               `json:"completed"`
	FailedCount    int               `json:"failed"`
	ActiveCount    int               `json:"active"`
}

func (s *ExperimentStatusSummary) String() string {
	return string(s.Status)
}

func NewExperimentStatusSummary(experiment *v1alpha1.Experiment, trialList *v1alpha1.TrialList) (*ExperimentStatusSummary, error) {
	s := &ExperimentStatusSummary{Status: ExperimentCreated}
	for i := range trialList.Items {
		ts, err := NewTrialStatusSummary(&trialList.Items[i])
		if err != nil {
			return nil, err
		}
		switch ts.Status {
		case TrialCompleted:
			s.CompletedCount++
		case TrialFailed:
			s.FailedCount++
		default:
			s.ActiveCount++
		}
	}

	// The order if this if/else block is very specific
	if experiment.GetReplicas() == 0 {
		s.Status = ExperimentPaused
	} else if len(trialList.Items) == 0 {
		s.Status = ExperimentEmpty
	} else if s.ActiveCount == 0 {
		s.Status = ExperimentIdle
	} else {
		s.Status = ExperimentRunning
	}

	return s, nil
}

// TrialStatusSummary is just a summary of the resource status
type TrialStatusSummary struct {
	Status TrialSummary `json:"status"`
}

func (s *TrialStatusSummary) String() string {
	return string(s.Status)
}

// Returns a string to summarize the trial status
func NewTrialStatusSummary(trial *v1alpha1.Trial) (*TrialStatusSummary, error) {
	s := &TrialStatusSummary{Status: TrialCreated}

	for i := range trial.Status.Conditions {
		c := trial.Status.Conditions[i]
		switch c.Type {

		case v1alpha1.TrialSetupCreated:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialSetupCreated
			case corev1.ConditionFalse:
				s.Status = TrialSettingUp
			case corev1.ConditionUnknown:
				s.Status = TrialSettingUp
			}

		case v1alpha1.TrialSetupDeleted:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialSetupDeleted
			case corev1.ConditionFalse:
				s.Status = TrialTearingDown
			}

		case v1alpha1.TrialPatched:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialPatched
			case corev1.ConditionFalse:
				s.Status = TrialPatching
			case corev1.ConditionUnknown:
				s.Status = TrialPatching
			}

		case v1alpha1.TrialStable:
			switch c.Status {
			case corev1.ConditionTrue:
				if trial.Status.StartTime != nil {
					s.Status = TrialRunning
				} else {
					s.Status = TrialStabilized
				}
			case corev1.ConditionFalse:
				s.Status = TrailWaiting
			case corev1.ConditionUnknown:
				s.Status = TrailWaiting
			}

		case v1alpha1.TrialObserved:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialCaptured
			case corev1.ConditionFalse:
				s.Status = TrialCapturing
			case corev1.ConditionUnknown:
				s.Status = TrialCapturing
			}

		case v1alpha1.TrialComplete:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialCompleted
				return s, nil
			}

		case v1alpha1.TrialFailed:
			switch c.Status {
			case corev1.ConditionTrue:
				s.Status = TrialFailed
				return s, nil
			}
		}
	}

	return s, nil
}
