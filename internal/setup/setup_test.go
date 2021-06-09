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

package setup_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/setup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestGetTrialConditionType(t *testing.T) {
	testCases := []struct {
		desc          string
		job           *batchv1.Job
		expectedError bool
		conditionType optimizev1beta2.TrialConditionType
	}{
		{
			desc: "no mode",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{},
								},
							},
						},
					},
				},
			},
			expectedError: true,
		},
		{
			desc: "create",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "MODE",
											Value: setup.ModeCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			conditionType: optimizev1beta2.TrialSetupCreated,
			expectedError: false,
		},
		{
			desc: "delete",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "MODE",
											Value: setup.ModeDelete,
										},
									},
								},
							},
						},
					},
				},
			},
			conditionType: optimizev1beta2.TrialSetupDeleted,
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			condType, err := setup.GetTrialConditionType(tc.job)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.conditionType, condType)
		})
	}
}
