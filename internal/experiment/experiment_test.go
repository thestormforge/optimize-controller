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

package experiment

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	redsky "github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSummarize(t *testing.T) {
	var (
		experimentURL           = "http://example.com/experiment"
		nextExperimentURL       = path.Join(experimentURL, "next")
		now                     = metav1.Now()
		oneReplica        int32 = 1
		zeroReplicas      int32 = 0
	)

	testCases := []struct {
		desc          string
		experiment    *redsky.Experiment
		expectedPhase string
		activeTrials  int32
		totalTrials   int
	}{
		{
			desc:          "empty",
			experiment:    &redsky.Experiment{},
			expectedPhase: PhaseEmpty,
		},
		{
			desc: "created",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redsky.AnnotationExperimentURL: experimentURL,
					},
				},
			},
			expectedPhase: PhaseCreated,
		},
		{
			desc: "deleted",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			expectedPhase: PhaseDeleted,
		},
		{
			desc: "deleted ignore active trials",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			expectedPhase: PhaseDeleted,
			activeTrials:  1,
		},
		{
			desc: "deleted ignore replicas",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
				Spec: redsky.ExperimentSpec{
					Replicas: &oneReplica,
				},
			},
			expectedPhase: PhaseDeleted,
		},
		{
			desc: "paused no active trials",
			experiment: &redsky.Experiment{
				Spec: redsky.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
			},
			expectedPhase: PhasePaused,
		},
		{
			desc: "paused active trials",
			experiment: &redsky.Experiment{
				Spec: redsky.ExperimentSpec{
					Replicas: &oneReplica,
				},
			},
			expectedPhase: PhaseRunning,
			activeTrials:  1,
		},
		{
			desc: "paused budget done",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redsky.AnnotationExperimentURL: experimentURL,
					},
				},
				Spec: redsky.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
				Status: redsky.ExperimentStatus{
					Conditions: []redsky.ExperimentCondition{
						{
							Type:   redsky.ExperimentComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedPhase: PhaseCompleted,
		},
		{
			desc: "paused budget",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redsky.AnnotationExperimentURL: experimentURL,
						redsky.AnnotationNextTrialURL:  nextExperimentURL,
					},
				},
				Spec: redsky.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
			},
			expectedPhase: PhasePaused,
		},
		{
			desc:          "idle not synced",
			experiment:    &redsky.Experiment{},
			expectedPhase: PhaseIdle,
			totalTrials:   1,
		},
		{
			desc: "idle synced",
			experiment: &redsky.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redsky.AnnotationExperimentURL: experimentURL,
					},
				},
			},
			totalTrials:   1,
			expectedPhase: PhaseIdle,
		},
		{
			desc: "failed",
			experiment: &redsky.Experiment{
				Status: redsky.ExperimentStatus{
					Conditions: []redsky.ExperimentCondition{
						{
							Type:   redsky.ExperimentFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedPhase: PhaseFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			summary := summarize(tc.experiment, tc.activeTrials, tc.totalTrials)
			assert.Equal(t, tc.expectedPhase, summary)
		})
	}
}

func TestApplyCondition(t *testing.T) {
	now := metav1.Now()
	then := metav1.NewTime(now.Add(-5 * time.Second))

	cases := []struct {
		desc               string
		conditionType      redsky.ExperimentConditionType
		conditionStatus    corev1.ConditionStatus
		reason             string
		message            string
		time               *metav1.Time
		initialConditions  []redsky.ExperimentCondition
		expectedConditions []redsky.ExperimentCondition
	}{
		{
			desc:            "add to empty",
			conditionType:   redsky.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			expectedConditions: []redsky.ExperimentCondition{
				{
					Type:               redsky.ExperimentFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      now,
					LastTransitionTime: now,
					Reason:             "Testing",
					Message:            "Test Test",
				},
			},
		},
		{
			desc:            "update status",
			conditionType:   redsky.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			initialConditions: []redsky.ExperimentCondition{
				{
					Type:               redsky.ExperimentFailed,
					Status:             corev1.ConditionFalse,
					LastProbeTime:      then,
					LastTransitionTime: then,
					Reason:             "Foo",
					Message:            "Bar",
				},
			},
			expectedConditions: []redsky.ExperimentCondition{
				{
					Type:               redsky.ExperimentFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      now,
					LastTransitionTime: now,
					Reason:             "Testing",
					Message:            "Test Test",
				},
			},
		},
		{
			desc:            "update no change",
			conditionType:   redsky.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			initialConditions: []redsky.ExperimentCondition{
				{
					Type:               redsky.ExperimentFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      then,
					LastTransitionTime: then,
					Reason:             "Foo",
					Message:            "Bar",
				},
			},
			expectedConditions: []redsky.ExperimentCondition{
				{
					Type:               redsky.ExperimentFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      now,
					LastTransitionTime: then,
					Reason:             "Foo",
					Message:            "Bar",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual := redsky.ExperimentStatus{Conditions: c.initialConditions}
			ApplyCondition(&actual, c.conditionType, c.conditionStatus, c.reason, c.message, c.time)
			assert.Equal(t, c.expectedConditions, actual.Conditions)
		})
	}
}
