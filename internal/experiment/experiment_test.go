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

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
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
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			summary := summarize(tc.experiment, tc.activeTrials, tc.totalTrials)
			assert.Equal(t, tc.expectedPhase, summary)
		})
	}
}

// Explicitly sets the state of the fields consider when computing the phase
func setupExperiment(exp *redsky.Experiment, replicas *int32, experimentURL, nextTrialURL string, deletionTimestamp *metav1.Time) {
	exp.Spec.Replicas = replicas

	if experimentURL != "" {
		exp.Annotations[redskyv1beta1.AnnotationExperimentURL] = experimentURL
	} else {
		delete(exp.Annotations, redskyv1beta1.AnnotationExperimentURL)
	}

	if nextTrialURL != "" {
		exp.Annotations[redskyv1beta1.AnnotationNextTrialURL] = nextTrialURL
	} else {
		delete(exp.Annotations, redskyv1beta1.AnnotationNextTrialURL)
	}

	exp.DeletionTimestamp = deletionTimestamp
}
