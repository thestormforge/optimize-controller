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
	"testing"
	"time"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestEngine_RenderPatch(t *testing.T) {
	eng := New()

	cases := []struct {
		desc          string
		patchTemplate redskyv1beta1.PatchTemplate
		trial         redskyv1beta1.Trial
		expected      []byte
	}{
		{
			desc: "default patch",
			patchTemplate: redskyv1beta1.PatchTemplate{
				Patch: "metadata:\n  labels:\n    app: testApp\n",
				TargetRef: &corev1.ObjectReference{
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "testPod",
					APIVersion: "v1",
				},
			},
			expected: []byte(`{"metadata":{"labels":{"app":"testApp"}}}`),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := eng.RenderPatch(&c.patchTemplate, &c.trial)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestEngine_RenderHelmValue(t *testing.T) {
	eng := New()

	cases := []struct {
		desc      string
		helmValue redskyv1beta1.HelmValue
		trial     redskyv1beta1.Trial
		expected  string
	}{
		{
			desc: "default helm",
			helmValue: redskyv1beta1.HelmValue{
				Name:  "name",
				Value: intstr.FromString("testName"),
			},
			expected: "testName",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := eng.RenderHelmValue(&c.helmValue, &c.trial)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestEngine_RenderMetricQueries(t *testing.T) {
	eng := New()
	now := metav1.Now()

	cases := []struct {
		desc               string
		metric             redskyv1beta1.Metric
		trial              redskyv1beta1.Trial
		target             runtime.Object
		expectedQuery      string
		expectedErrorQuery string
	}{
		{
			desc: "default metric (duration)",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  redskyv1beta1.MetricLocal,
			},
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					StartTime:      &metav1.Time{Time: now.Add(-5 * time.Second)},
					CompletionTime: &now,
				},
			},
			target:        &corev1.Pod{},
			expectedQuery: "5",
		},
		{
			desc: "default metric (percent)",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{percent 100 5}}",
				Type:  redskyv1beta1.MetricLocal,
			},
			target:        &corev1.Pod{},
			expectedQuery: "5",
		},
		{
			desc: "default metric (weighted)",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: `{{resourceRequests .Pods "cpu=0.05,memory=0.005"}}`,
				Type:  redskyv1beta1.MetricLocal,
			},
			target: &corev1.PodList{
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
			expectedQuery: "25010",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actualQuery, actualErrorQuery, err := eng.RenderMetricQueries(&c.metric, &c.trial, c.target)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expectedQuery, actualQuery)
				assert.Equal(t, c.expectedErrorQuery, actualErrorQuery)
			}
		})
	}
}
