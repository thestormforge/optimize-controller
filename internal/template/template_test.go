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

package template

import (
	"fmt"
	"testing"
	"time"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestEngine(t *testing.T) {
	now := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Minute))
	later := metav1.NewTime(now.Add(5 * time.Second))

	eng := New()

	testCases := []struct {
		desc     string
		trial    *redskyv1alpha1.Trial
		input    interface{}
		obj      runtime.Object
		expected string
	}{
		{
			desc: "default patch",
			trial: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			},
			input: &redskyv1alpha1.PatchTemplate{
				Patch: "metadata:\n  labels:\n    app: testApp\n",
				TargetRef: &corev1.ObjectReference{
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "testPod",
					APIVersion: "v1",
				},
			},
			expected: `{"metadata":{"labels":{"app":"testApp"}}}`,
		},
		{
			desc: "default helm",
			trial: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			},
			input: &redskyv1alpha1.HelmValue{
				Name:  "name",
				Value: intstr.FromString("testName"),
			},
			expected: "testName",
		},
		{
			desc: "default metric (duration)",
			trial: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			},
			input: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  redskyv1alpha1.MetricLocal,
			},
			obj:      &corev1.Pod{},
			expected: "5",
		},
		{
			desc: "default metric (percent)",
			trial: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			},
			input: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "{{percent 100 5}}",
				Type:  redskyv1alpha1.MetricLocal,
			},
			obj:      &corev1.Pod{},
			expected: "5",
		},
		{
			desc: "default metric (weighted)",
			trial: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			},
			input: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: `{{resourceRequests .Pods "cpu=0.05,memory=0.005"}}`,
				Type:  redskyv1alpha1.MetricLocal,
			},
			obj: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testpod1",
							Namespace: "default",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "testContainer1",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("200m"),
											corev1.ResourceMemory: resource.MustParse("5000"),
										},
									},
								},
							},
						},
					},
				},
			},
			expected: "25010",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			var (
				err     error
				boutput []byte
				got     string
			)

			switch tc.input.(type) {
			case *redskyv1alpha1.PatchTemplate:
				boutput, err = eng.RenderPatch(tc.input.(*redskyv1alpha1.PatchTemplate), tc.trial)
				got = string(boutput)
			case *redskyv1alpha1.HelmValue:
				got, err = eng.RenderHelmValue(tc.input.(*redskyv1alpha1.HelmValue), tc.trial)
			case *redskyv1alpha1.Metric:
				got, _, err = eng.RenderMetricQueries(tc.input.(*redskyv1alpha1.Metric), tc.trial, tc.obj)
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
