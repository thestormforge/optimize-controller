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
	"context"
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskytrial "github.com/redskyops/k8s-experiment/pkg/controller/trial"
	"github.com/redskyops/k8s-experiment/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ExperimentFinalizer is used by the experiment controller to ensure synchronization with the remote server
	ExperimentFinalizer = "experimentFinalizer.redskyops.dev"
)

// PopulateTrialFromTemplate creates a new trial for an experiment
func PopulateTrialFromTemplate(experiment *redskyv1alpha1.Experiment, trial *redskyv1alpha1.Trial, namespace string) {
	// Start with the trial template
	experiment.Spec.Template.ObjectMeta.DeepCopyInto(&trial.ObjectMeta)
	experiment.Spec.Template.Spec.DeepCopyInto(&trial.Spec)

	// The creation timestamp is NOT a pointer so it needs an explicit value that serializes to something
	// TODO This should not be necessary
	if trial.Spec.Template != nil {
		trial.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Now()
		trial.Spec.Template.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Now()
	}

	// Overwrite the target namespace unless we are only running a single trial on the cluster
	if experiment.GetReplicas() > 1 || experiment.Spec.NamespaceSelector != nil || experiment.Spec.Template.Namespace != "" {
		trial.Spec.TargetNamespace = namespace
	}

	if trial.Namespace == "" {
		trial.Namespace = namespace
	}

	if trial.Name == "" {
		if trial.Namespace != experiment.Namespace {
			trial.Name = experiment.Name
		} else if trial.GenerateName == "" {
			trial.GenerateName = experiment.Name + "-"
		}
	}

	if len(trial.Labels) == 0 {
		trial.Labels = experiment.GetDefaultLabels()
	}

	if trial.Annotations == nil {
		trial.Annotations = make(map[string]string)
	}

	if trial.Spec.ExperimentRef == nil {
		trial.Spec.ExperimentRef = experiment.GetSelfReference()
	}
}

// FindAvailableNamespace searches for a namespace to run a new trial in, returning an empty string if no such namespace can be found
func FindAvailableNamespace(r client.Reader, experiment *redskyv1alpha1.Experiment, trials []redskyv1alpha1.Trial) (string, error) {
	// Determine which namespaces are already in use
	var activeTrials int
	inuse := make(map[string]bool, len(trials))
	for i := range trials {
		if redskytrial.IsTrialActive(&trials[i]) {
			activeTrials++
			inuse[trials[i].Namespace] = true
		}
	}

	// Do not return a namespace if the number of desired replicas has been reached
	if activeTrials >= experiment.GetReplicas() {
		return "", nil
	}

	// Find eligible namespaces
	if experiment.Spec.NamespaceSelector != nil {
		list := &corev1.NamespaceList{}
		matchingSelector, err := util.MatchingSelector(experiment.Spec.NamespaceSelector)
		if err != nil {
			return "", err
		}
		if err := r.List(context.TODO(), list, matchingSelector); err != nil {
			return "", err
		}

		// Find the first available namespace
		for _, item := range list.Items {
			if !inuse[item.Name] {
				return item.Name, nil
			}
		}
		return "", nil
	}

	// No selector was specified, pretend like we only matched the experiment namespace
	if !inuse[experiment.Namespace] {
		return experiment.Namespace, nil
	}
	return "", nil
}

// NeedsCleanup checks whether a trial's TTL has expired
func NeedsCleanup(t *redskyv1alpha1.Trial) bool {
	// Trials that are deleted or do not have a TTL are never ready for clean up
	if !t.DeletionTimestamp.IsZero() || t.Spec.TTLSecondsAfterFinished == nil {
		return false
	}

	// Determine the finish time
	finishTime := trialFinishTime(t)
	if finishTime.IsZero() {
		return false
	}

	// Check to see if we are still in the TTL window
	ttl := time.Duration(*t.Spec.TTLSecondsAfterFinished) * time.Second
	return finishTime.UTC().Add(ttl).Before(time.Now().UTC())
}

// trialFinishTime returns the time that a trial finished; this time can skip ahead of the normal definition of "finished" if there are setup delete tasks.
func trialFinishTime(t *redskyv1alpha1.Trial) metav1.Time {
	for _, c := range t.Status.Conditions {
		if c.Status == corev1.ConditionTrue && !c.LastTransitionTime.IsZero() && (c.Type == redskyv1alpha1.TrialSetupDeleted || c.Type == redskyv1alpha1.TrialComplete || c.Type == redskyv1alpha1.TrialFailed) {
			return c.LastTransitionTime
		}
	}
	return metav1.Time{}
}
