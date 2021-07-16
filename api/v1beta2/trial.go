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

package v1beta2

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ExperimentNamespacedName returns the namespaced name of the experiment for this trial
func (in *Trial) ExperimentNamespacedName() types.NamespacedName {
	nn := types.NamespacedName{Namespace: in.Namespace, Name: in.Name}
	if in.Labels[LabelExperiment] != "" {
		nn.Name = in.Labels[LabelExperiment]
	}
	if in.Spec.ExperimentRef != nil {
		if in.Spec.ExperimentRef.Namespace != "" {
			nn.Namespace = in.Spec.ExperimentRef.Namespace
		}
		if in.Spec.ExperimentRef.Name != "" {
			nn.Name = in.Spec.ExperimentRef.Name
		}
	}
	return nn
}

// HasInitializer checks to see if the trial has an initializer
func (in *Trial) HasInitializer() bool {
	return strings.TrimSpace(in.GetAnnotations()[AnnotationInitializer]) != ""
}

// GetAssignment returns an assignment value by name
func (in *Trial) GetAssignment(name string) (intstr.IntOrString, bool) {
	for i := range in.Spec.Assignments {
		if in.Spec.Assignments[i].Name == name {
			return in.Spec.Assignments[i].Value, true
		}
	}
	return intstr.FromInt(0), false
}

// GetJobSelector returns the job selector
func (in *Trial) GetJobSelector() *metav1.LabelSelector {
	if in.Spec.Selector != nil {
		return in.Spec.Selector
	}

	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelTrial:     in.Name,
			LabelTrialRole: "trialRun",
		},
	}
}
