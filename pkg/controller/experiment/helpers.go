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

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskytrial "github.com/redskyops/k8s-experiment/pkg/controller/trial"
	"github.com/redskyops/k8s-experiment/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Finalizer = "finalizer.redskyops.dev"
)

// HasFinalizer checks an object (should be an experiment or trial) for the experiment finalizer
func HasFinalizer(obj metav1.Object) bool {
	for _, f := range obj.GetFinalizers() {
		if f == Finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer adds the experiment finalizer to an object (should be an experiment or trial); returns true only if the finalizer is changed
func AddFinalizer(obj metav1.Object) bool {
	// Do not add the finalizer if the object is already deleted
	if obj.GetDeletionTimestamp() != nil {
		return false
	}

	// Do not add the finalizer more then once
	if HasFinalizer(obj) {
		return false
	}

	// Actually add the finalizer
	obj.SetFinalizers(append(obj.GetFinalizers(), Finalizer))
	return true
}

// RemoveFinalizer deletes the experiment finalizer from an object (should be an experiment or trial); return true only if the finalizer is changed
func RemoveFinalizer(obj metav1.Object) bool {
	finalizers := obj.GetFinalizers()
	for i := range finalizers {
		if finalizers[i] == Finalizer {
			finalizers[i] = finalizers[len(finalizers)-1]
			obj.SetFinalizers(finalizers[:len(finalizers)-1])
			return true
		}
	}
	return false
}

// Creates a new trial for an experiment
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

// Searches for a namespace to run a new trial in, returning an empty string if no such namespace can be found
func FindAvailableNamespace(r client.Reader, experiment *redskyv1alpha1.Experiment, trials []redskyv1alpha1.Trial) (string, error) {
	// Determine which namespaces are already in use
	var activeTrials int
	inuse := make(map[string]bool, len(trials))
	for i := range trials {
		if redskytrial.IsTrialFinished(&trials[i]) {
			for _, c := range trials[i].Status.Conditions {
				if c.Type == redskyv1alpha1.TrialSetupDeleted && c.Status != corev1.ConditionTrue {
					// Do not count this against the active trials, i.e. allow setup tasks to overlap
					inuse[trials[i].Namespace] = true
				}
			}
		} else {
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
