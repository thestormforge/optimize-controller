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

package experiment

import (
	"testing"

	. "github.com/onsi/gomega"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSummarize(t *testing.T) {
	g := NewGomegaWithT(t)
	now := metav1.Now()
	paused := int32(0)
	exp := &redskyv1alpha1.Experiment{}
	exp.Annotations = map[string]string{}

	// Initial phase is "empty" or created
	setupExperiment(exp, nil, "", "", nil)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhaseEmpty))
	setupExperiment(exp, nil, "http://example.com/experiment", "", nil)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhaseCreated))

	// Once deleted, phase should ignore replica/active trials
	setupExperiment(exp, nil, "", "", &now)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhaseDeleted))
	g.Expect(summarize(exp, 1, 0)).To(Equal(PhaseDeleted))
	setupExperiment(exp, &paused, "", "", &now)
	g.Expect(summarize(exp, 1, 0)).To(Equal(PhaseDeleted))

	// Even when paused, phase should reflect active trials
	setupExperiment(exp, &paused, "", "", nil)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhasePaused))
	g.Expect(summarize(exp, 1, 0)).To(Equal(PhaseRunning))

	// When the experiment is synchronized remotely the "paused" state accounts for the experiment exceeding the budget
	setupExperiment(exp, &paused, "http://example.com/experiment", "", nil)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhaseCompleted))
	setupExperiment(exp, &paused, "http://example.com/experiment", "http://example.com/experiment/next", nil)
	g.Expect(summarize(exp, 0, 0)).To(Equal(PhasePaused))

	// Idle occurs when there are trials
	setupExperiment(exp, nil, "", "", nil)
	g.Expect(summarize(exp, 0, 1)).To(Equal(PhaseIdle))
	setupExperiment(exp, nil, "http://example.com/experiment", "", nil)
	g.Expect(summarize(exp, 0, 1)).To(Equal(PhaseIdle))
}

// Explicitly sets the state of the fields consider when computing the phase
func setupExperiment(exp *redskyv1alpha1.Experiment, replicas *int32, experimentURL, nextTrialURL string, deletionTimestamp *metav1.Time) {
	exp.Spec.Replicas = replicas

	if experimentURL != "" {
		exp.Annotations[redskyv1alpha1.AnnotationExperimentURL] = experimentURL
	} else {
		delete(exp.Annotations, redskyv1alpha1.AnnotationExperimentURL)
	}

	if nextTrialURL != "" {
		exp.Annotations[redskyv1alpha1.AnnotationNextTrialURL] = nextTrialURL
	} else {
		delete(exp.Annotations, redskyv1alpha1.AnnotationNextTrialURL)
	}

	exp.DeletionTimestamp = deletionTimestamp
}
