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
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PopulateTrialFromTemplate creates a new trial for an experiment
func PopulateTrialFromTemplate(exp *optimizev1beta2.Experiment, t *optimizev1beta2.Trial) {
	// Start with the trial template
	exp.Spec.TrialTemplate.ObjectMeta.DeepCopyInto(&t.ObjectMeta)
	exp.Spec.TrialTemplate.Spec.DeepCopyInto(&t.Spec)

	// The creation timestamp is NOT a pointer so it needs an explicit value that serializes to something
	// TODO This should not be necessary
	if t.Spec.JobTemplate != nil {
		t.Spec.JobTemplate.ObjectMeta.CreationTimestamp = metav1.Now()
		t.Spec.JobTemplate.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Now()
	}

	// Make sure labels and annotation maps are not nil
	if t.Labels == nil {
		t.Labels = map[string]string{}
	}
	if t.Annotations == nil {
		t.Annotations = map[string]string{}
	}

	// Record the experiment
	t.Labels[optimizev1beta2.LabelExperiment] = exp.Name
	t.Spec.ExperimentRef = &corev1.ObjectReference{
		Name:      exp.Name,
		Namespace: exp.Namespace,
	}

	// Default trial name is the experiment name with a random suffix
	if t.Name == "" && t.GenerateName == "" {
		t.GenerateName = exp.Name + "-"
	}

	// Default trial namespace only if the experiment is not configured to find or create a namespace to run in
	if t.Namespace == "" && exp.Spec.NamespaceSelector == nil && exp.Spec.NamespaceTemplate == nil {
		t.Namespace = exp.Namespace
	}
}
