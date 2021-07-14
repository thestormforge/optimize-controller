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
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
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
		experiment    *optimizev1beta2.Experiment
		expectedPhase string
		activeTrials  int32
		totalTrials   int
	}{
		{
			desc:          "empty",
			experiment:    &optimizev1beta2.Experiment{},
			expectedPhase: PhaseEmpty,
		},
		{
			desc: "created",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						optimizev1beta2.AnnotationExperimentURL: experimentURL,
					},
				},
			},
			expectedPhase: PhaseCreated,
		},
		{
			desc: "deleted",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			expectedPhase: PhaseDeleted,
		},
		{
			desc: "deleted ignore active trials",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			expectedPhase: PhaseDeleted,
			activeTrials:  1,
		},
		{
			desc: "deleted ignore replicas",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
				Spec: optimizev1beta2.ExperimentSpec{
					Replicas: &oneReplica,
				},
			},
			expectedPhase: PhaseDeleted,
		},
		{
			desc: "paused no active trials",
			experiment: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
			},
			expectedPhase: PhasePaused,
		},
		{
			desc: "paused active trials",
			experiment: &optimizev1beta2.Experiment{
				Spec: optimizev1beta2.ExperimentSpec{
					Replicas: &oneReplica,
				},
			},
			expectedPhase: PhaseRunning,
			activeTrials:  1,
		},
		{
			desc: "paused budget done",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						optimizev1beta2.AnnotationExperimentURL: experimentURL,
					},
				},
				Spec: optimizev1beta2.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
				Status: optimizev1beta2.ExperimentStatus{
					Conditions: []optimizev1beta2.ExperimentCondition{
						{
							Type:   optimizev1beta2.ExperimentComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedPhase: PhaseCompleted,
		},
		{
			desc: "paused budget",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						optimizev1beta2.AnnotationExperimentURL: experimentURL,
						optimizev1beta2.AnnotationNextTrialURL:  nextExperimentURL,
					},
				},
				Spec: optimizev1beta2.ExperimentSpec{
					Replicas: &zeroReplicas,
				},
			},
			expectedPhase: PhasePaused,
		},
		{
			desc:          "idle not synced",
			experiment:    &optimizev1beta2.Experiment{},
			expectedPhase: PhaseIdle,
			totalTrials:   1,
		},
		{
			desc: "idle synced",
			experiment: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						optimizev1beta2.AnnotationExperimentURL: experimentURL,
					},
				},
			},
			totalTrials:   1,
			expectedPhase: PhaseIdle,
		},
		{
			desc: "failed",
			experiment: &optimizev1beta2.Experiment{
				Status: optimizev1beta2.ExperimentStatus{
					Conditions: []optimizev1beta2.ExperimentCondition{
						{
							Type:   optimizev1beta2.ExperimentFailed,
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
		conditionType      optimizev1beta2.ExperimentConditionType
		conditionStatus    corev1.ConditionStatus
		reason             string
		message            string
		time               *metav1.Time
		initialConditions  []optimizev1beta2.ExperimentCondition
		expectedConditions []optimizev1beta2.ExperimentCondition
	}{
		{
			desc:            "add to empty",
			conditionType:   optimizev1beta2.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			expectedConditions: []optimizev1beta2.ExperimentCondition{
				{
					Type:               optimizev1beta2.ExperimentFailed,
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
			conditionType:   optimizev1beta2.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			initialConditions: []optimizev1beta2.ExperimentCondition{
				{
					Type:               optimizev1beta2.ExperimentFailed,
					Status:             corev1.ConditionFalse,
					LastProbeTime:      then,
					LastTransitionTime: then,
					Reason:             "Foo",
					Message:            "Bar",
				},
			},
			expectedConditions: []optimizev1beta2.ExperimentCondition{
				{
					Type:               optimizev1beta2.ExperimentFailed,
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
			conditionType:   optimizev1beta2.ExperimentFailed,
			conditionStatus: corev1.ConditionTrue,
			reason:          "Testing",
			message:         "Test Test",
			time:            &now,
			initialConditions: []optimizev1beta2.ExperimentCondition{
				{
					Type:               optimizev1beta2.ExperimentFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      then,
					LastTransitionTime: then,
					Reason:             "Foo",
					Message:            "Bar",
				},
			},
			expectedConditions: []optimizev1beta2.ExperimentCondition{
				{
					Type:               optimizev1beta2.ExperimentFailed,
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
			actual := optimizev1beta2.ExperimentStatus{Conditions: c.initialConditions}
			ApplyCondition(&actual, c.conditionType, c.conditionStatus, c.reason, c.message, c.time)
			assert.Equal(t, c.expectedConditions, actual.Conditions)
		})
	}
}

func TestStopExperiment(t *testing.T) {
	cases := []struct {
		desc        string
		exp         *optimizev1beta2.Experiment
		err         error
		expectedOut bool
		expectedExp *optimizev1beta2.Experiment
	}{
		{
			desc: "no error",
			exp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err:         nil,
			expectedOut: false,
			expectedExp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error wrong type",
			exp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err: &api.Error{
				Type: experimentsv1alpha1.ErrExperimentNameInvalid,
			},
			expectedOut: false,
			expectedExp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error",
			exp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						optimizev1beta2.AnnotationNextTrialURL: "111",
					},
				},
			},
			err: &api.Error{
				Type: experimentsv1alpha1.ErrExperimentStopped,
			},
			expectedOut: true,
			expectedExp: &optimizev1beta2.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			out := StopExperiment(c.exp, c.err)
			assert.Equal(t, c.expectedOut, out)
			assert.Equal(t, c.expectedExp.GetAnnotations(), c.exp.GetAnnotations())
		})
	}
}
