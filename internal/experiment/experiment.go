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
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
)

// TODO Make the constant names better reflect the code, not the text
// TODO Use a prefix, like "summary"?
const (
	created   string = "Created"
	paused           = "Paused"
	empty            = "Never run" // TODO This is misleading, it could be that we already deleted the trials that ran
	idle             = "Idle"
	running          = "Running"
	completed        = "Completed"
)

// UpdateStatus will ensure the experiment's status matches what is in the supplied trial list; returns true only if
// changes were necessary
func UpdateStatus(exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) bool {
	summary := created
	activeTrials := int32(0)

	// Count up the trials by state
	for i := range trialList.Items {
		t := &trialList.Items[i]
		if trial.IsActive(t) {
			activeTrials++
		}
	}

	// The order if this if/else block is very specific
	if exp.Replicas() == 0 {
		if exp.Annotations[redskyv1alpha1.AnnotationExperimentURL] != "" && exp.Annotations[redskyv1alpha1.AnnotationNextTrialURL] == "" {
			// Either we got paused using manual suggestions (which doesn't make sense because you don't need to pause)
			// ...or we hit the end of the experiment and the server told us to stop
			summary = completed
		} else {
			summary = paused
		}
	} else if len(trialList.Items) == 0 {
		summary = empty
	} else if activeTrials == 0 {
		summary = idle
	} else {
		summary = running
	}

	// Update the status object
	var dirty bool
	if exp.Status.Summary != summary {
		exp.Status.Summary = summary
		dirty = true
	}
	if exp.Status.ActiveTrials != activeTrials {
		exp.Status.ActiveTrials = activeTrials
		dirty = true
	}
	return dirty
}
