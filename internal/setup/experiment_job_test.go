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

package setup

import (
	"fmt"
	"testing"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExperimentJob(t *testing.T) {
	testCases := []struct {
		desc string
		exp  redskyv1beta1.Experiment
		mode string
	}{
		{
			desc: "default create",
			exp: redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exp",
					Namespace: "default",
				},
			},
			mode: ModeCreate,
		},
		{
			desc: "default delete",
			exp: redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exp",
					Namespace: "default",
				},
			},
			mode: ModeDelete,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			job, err := NewExperimentJob(&tc.exp, tc.mode)
			assert.NoError(t, err)

			assert.NotNil(t, job)
			assert.Equal(t, tc.exp.Name, job.Labels[redskyv1beta1.LabelExperiment])
			assert.Len(t, job.Spec.Template.Spec.Containers, 1)
			assert.Equal(t, tc.exp.Namespace, job.Spec.Template.Spec.Containers[0].Env[0].Value)
			assert.Contains(t, job.Spec.Template.Spec.Containers[0].Args, tc.mode)
		})
	}
}
