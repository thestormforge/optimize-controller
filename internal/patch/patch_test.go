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

package patch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/template"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPatch(t *testing.T) {
	te := template.New()

	trial := &optimizev1beta2.Trial{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mytrial",
			Namespace: "default",
		},
	}

	patchMeta := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: default`

	patchSpec := `spec:
        template:
          spec:
            containers:
            - name: postgres
              imagePullPolicy: Always`

	fullPatch := patchMeta + "\n" + patchSpec

	jsonPatch := `[
    {
     "op": "replace",
     "path": "/spec/template/spec/containers/0/imagePullPolicy",
		 "value": "Always"
    },
  ]`

	testCases := []struct {
		desc                string
		trial               *optimizev1beta2.Trial
		patchTemplate       *optimizev1beta2.PatchTemplate
		attemptsRemaining   int
		expectedError       bool
		expectedRenderError bool
		expectedPOError     bool
	}{
		{
			desc:  "default",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				// Note: defining an empty string ("") results
				// in a `null` being returned. Think this is valid
				// but makes testing a little more complicated.
				Patch: fullPatch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
			attemptsRemaining: defaultAttemptsRemaining,
		},
		{
			desc:  "strategic w/o targetref",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:      optimizev1beta2.PatchStrategic,
				Patch:     fullPatch,
				TargetRef: nil,
			},
			attemptsRemaining: defaultAttemptsRemaining,
		},
		{
			desc:  "strategic w/o targetref w/o full",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:      optimizev1beta2.PatchStrategic,
				Patch:     patchSpec,
				TargetRef: nil,
			},
			expectedRenderError: true,
		},
		{
			desc:  "patchmerge",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:  optimizev1beta2.PatchMerge,
				Patch: fullPatch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
			attemptsRemaining: defaultAttemptsRemaining,
		},
		{
			desc:  "patchjson",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:  optimizev1beta2.PatchJSON,
				Patch: jsonPatch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
			attemptsRemaining: defaultAttemptsRemaining,
		},
		{
			desc:  "patchTrial - json",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:  optimizev1beta2.PatchJSON,
				Patch: patchSpec,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Job",
					APIVersion: "batch/v1",
					Name:       trial.Name,
					Namespace:  trial.Namespace,
				},
			},
			expectedPOError: true,
		},
		{
			desc:  "patchTrial - strategic merge",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:  optimizev1beta2.PatchStrategic,
				Patch: patchSpec,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Job",
					APIVersion: "batch/v1",
					Name:       trial.Name,
					Namespace:  trial.Namespace,
				},
			},
			attemptsRemaining: 0,
		},
		{
			desc:  "patchTrial - optional name",
			trial: trial,
			patchTemplate: &optimizev1beta2.PatchTemplate{
				Type:  optimizev1beta2.PatchStrategic,
				Patch: patchSpec,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Job",
					APIVersion: "batch/v1",
					Name:       "",
					Namespace:  trial.Namespace,
				},
			},
			attemptsRemaining: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			// Test RenderTemplate
			ref, data, err := RenderTemplate(te, tc.trial, tc.patchTemplate)
			if tc.expectedRenderError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, ref)
			assert.NotEmpty(t, data)

			if tc.patchTemplate.TargetRef != nil {
				assert.Equal(t, tc.patchTemplate.TargetRef, ref)
			}

			switch tc.patchTemplate.Type {
			case optimizev1beta2.PatchStrategic, optimizev1beta2.PatchMerge:
				if !strings.Contains(tc.desc, "patchTrial") {
					assert.Equal(t, tc.patchTemplate.Patch, fullPatch)
				}
			case optimizev1beta2.PatchJSON:
				if !strings.Contains(tc.desc, "patchTrial") {
					assert.Equal(t, tc.patchTemplate.Patch, jsonPatch)
				}
			}

			// Test CreatePatchOperation
			po, err := CreatePatchOperation(tc.trial, tc.patchTemplate, ref, data)
			if tc.expectedPOError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, po)

			assert.Equal(t, tc.attemptsRemaining, po.AttemptsRemaining)
			if tc.patchTemplate.TargetRef != nil {
				assert.Equal(t, tc.patchTemplate.TargetRef, &po.TargetRef)
			}
			assert.Equal(t, data, po.Data)
		})
	}
}
