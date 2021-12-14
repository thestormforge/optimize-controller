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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewJob(t *testing.T) {
	testCases := []struct {
		desc    string
		trial   *optimizev1beta2.Trial
		args    []string
		command []string
	}{
		{
			desc: "default",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					SetupTasks: []optimizev1beta2.SetupTask{},
				},
			},
		},
		{
			desc: "default with args",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					SetupTasks: []optimizev1beta2.SetupTask{
						{
							Args: []string{"fun", "setup"},
						},
					},
				},
			},
		},
		{
			desc: "default with command and image",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					SetupTasks: []optimizev1beta2.SetupTask{
						{
							Image:   "whyis6afraidof7:because789",
							Command: []string{"fun", "setup"},
						},
					},
				},
			},
		},
		{
			desc: "default with env and labels",
			trial: &optimizev1beta2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: optimizev1beta2.TrialSpec{
					SetupTasks: []optimizev1beta2.SetupTask{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "imalittleteapot",
									Value: "shortandstout",
								},
							},
							Labels: map[string]string{
								"thisismyhandle": "andthisismyspout",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			j, err := setup.NewJob(tc.trial, "create")
			assert.NoError(t, err)

			if len(tc.trial.Spec.SetupTasks) == 0 {
				return
			}

			if len(tc.trial.Spec.SetupTasks[0].Command) > 0 {
				assert.Len(t, j.Spec.Template.Spec.Containers[0].Command, len(tc.trial.Spec.SetupTasks[0].Command))
			}

			if len(tc.trial.Spec.SetupTasks[0].Args) > 0 {
				assert.Len(t, j.Spec.Template.Spec.Containers[0].Args, len(tc.trial.Spec.SetupTasks[0].Args))
			}

			if len(tc.trial.Spec.SetupTasks[0].Env) > 0 {
				// We have 4 specific optimize env variables we inject
				assert.Len(t, j.Spec.Template.Spec.Containers[0].Env, len(tc.trial.Spec.SetupTasks[0].Env)+4)
			}

			if len(tc.trial.Spec.SetupTasks[0].Labels) > 0 {
				// We have 3 specific optimize labels we inject
				assert.Len(t, j.Labels, len(tc.trial.Spec.SetupTasks[0].Labels)+3)
				assert.Len(t, j.Spec.Template.Labels, len(tc.trial.Spec.SetupTasks[0].Labels)+3)
			}
		})
	}
}
