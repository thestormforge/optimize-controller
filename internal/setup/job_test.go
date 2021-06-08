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

		})
	}
}
