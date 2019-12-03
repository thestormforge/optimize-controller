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

package trial

import (
	"fmt"
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewJob returns a new trial run job from the template on the trial
func NewJob(t *redskyv1alpha1.Trial) *batchv1.Job {
	job := &batchv1.Job{}

	// Start with the job template
	if t.Spec.Template != nil {
		t.Spec.Template.ObjectMeta.DeepCopyInto(&job.ObjectMeta)
		t.Spec.Template.Spec.DeepCopyInto(&job.Spec)
	}

	// Provide default metadata
	if job.Name == "" {
		job.Name = t.Name
	}
	if job.Namespace == "" {
		job.Namespace = t.Namespace
	}

	// Provide default labels
	if len(job.Labels) == 0 {
		job.Labels = t.GetDefaultLabels()
	}
	if len(job.Spec.Template.Labels) == 0 {
		job.Spec.Template.Labels = t.GetDefaultLabels()
	}

	// Always provide experiment labels
	job.Labels[redskyv1alpha1.LabelExperiment] = t.ExperimentNamespacedName().Name
	job.Spec.Template.Labels[redskyv1alpha1.LabelExperiment] = t.ExperimentNamespacedName().Name

	// The default restart policy for a pod is not acceptable in the context of a job
	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	// The default backoff limit will restart the trial job which is unlikely to produce desirable results
	if job.Spec.BackoffLimit == nil {
		job.Spec.BackoffLimit = new(int32)
	}

	// Containers cannot be empty, inject a sleep by default
	if len(job.Spec.Template.Spec.Containers) == 0 {
		s := t.Spec.ApproximateRuntime
		if s == nil || s.Duration == 0 {
			s = &metav1.Duration{Duration: 2 * time.Minute}
		}
		if t.Spec.StartTimeOffset != nil {
			s = &metav1.Duration{Duration: s.Duration + t.Spec.StartTimeOffset.Duration}
		}
		job.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:    "default-trial-run",
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", fmt.Sprintf("echo 'Sleeping for %s...' && sleep %.0f && echo 'Done.'", s.Duration.String(), s.Seconds())},
			},
		}
	}

	return job
}
