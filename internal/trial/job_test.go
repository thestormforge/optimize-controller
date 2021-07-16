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

package trial

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewJob(t *testing.T) {
	testCases := []struct {
		desc               string
		trial              *optimizev1beta2.Trial
		expectedContainers int
	}{
		{
			desc: "no containers",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					JobTemplate: &batchv1beta1.JobTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "trial-job",
							Namespace: "default",
						},
						Spec: batchv1.JobSpec{},
					},
				},
			},
			expectedContainers: 1,
		},
		{
			desc: "two containers",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					JobTemplate: &batchv1beta1.JobTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "trial-job",
							Namespace: "default",
						},
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:    "trial-run-1",
											Image:   "busybox",
											Command: []string{"/bin/sh"},
										},
										{
											Name:    "trial-run-2",
											Image:   "busybox",
											Command: []string{"/bin/sh"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedContainers: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			job := NewJob(tc.trial)
			assert.NotNil(t, job)
			assert.Equal(t, len(job.Spec.Template.Spec.Containers), tc.expectedContainers)
		})
	}
}
