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

	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PopulateTrialFromTemplate creates a new trial for an experiment
func PopulateTrialFromTemplate(experiment *redskyv1alpha1.Experiment, t *redskyv1alpha1.Trial, namespace string) {
	// Start with the trial template
	experiment.Spec.Template.ObjectMeta.DeepCopyInto(&t.ObjectMeta)
	experiment.Spec.Template.Spec.DeepCopyInto(&t.Spec)

	// The creation timestamp is NOT a pointer so it needs an explicit value that serializes to something
	// TODO This should not be necessary
	if t.Spec.Template != nil {
		t.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Now()
		t.Spec.Template.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Now()
	}

	// Overwrite the target namespace unless we are only running a single trial on the cluster
	if experiment.GetReplicas() > 1 || experiment.Spec.NamespaceSelector != nil || experiment.Spec.Template.Namespace != "" {
		t.Spec.TargetNamespace = namespace
	}

	if t.Namespace == "" {
		t.Namespace = namespace
	}

	if t.Name == "" {
		if t.Namespace != experiment.Namespace {
			t.Name = experiment.Name
		} else if t.GenerateName == "" {
			t.GenerateName = experiment.Name + "-"
		}
	}

	if len(t.Labels) == 0 {
		t.Labels = experiment.GetDefaultLabels()
	}

	if t.Annotations == nil {
		t.Annotations = make(map[string]string)
	}

	if t.Spec.ExperimentRef == nil {
		t.Spec.ExperimentRef = experiment.GetSelfReference()
	}
}

// FindAvailableNamespace searches for a namespace to run a new trial in, returning an empty string if no such namespace can be found
func FindAvailableNamespace(r client.Reader, experiment *redskyv1alpha1.Experiment, trials []redskyv1alpha1.Trial) (string, error) {
	// Do not return a namespace if the number of desired replicas has been reached
	// IMPORTANT: This is a safe guard for callers who don't make this check prior to calling
	if experiment.Status.ActiveTrials >= experiment.GetReplicas() {
		return "", nil
	}

	// Determine which namespaces are already in use
	inuse := make(map[string]bool, len(trials))
	for i := range trials {
		if trial.IsActive(&trials[i]) {
			if trials[i].Spec.TargetNamespace != "" {
				inuse[trials[i].Spec.TargetNamespace] = true
			} else {
				inuse[trials[i].Namespace] = true
			}
		}
	}

	// Find eligible namespaces
	if experiment.Spec.NamespaceSelector != nil {
		list := &corev1.NamespaceList{}
		matchingSelector, err := meta.MatchingSelector(experiment.Spec.NamespaceSelector)
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
