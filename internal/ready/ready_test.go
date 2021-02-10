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

package ready

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadinessChecker_CheckConditions(t *testing.T) {
	cases := []struct {
		desc           string
		objs           []runtime.Object
		conditionTypes []string
		msg            string
		ready          bool
		err            error
	}{
		{
			desc:           "always-true",
			conditionTypes: []string{ConditionTypeAlwaysTrue},
			ready:          true,
		},
		{
			desc:           "pod-ready",
			conditionTypes: []string{ConditionTypePodReady},
			ready:          true,

			objs: []runtime.Object{
				&appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"test": "test"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
		},
		{
			desc:           "unschedulable",
			conditionTypes: []string{ConditionTypePodReady},
			err: &ReadinessError{
				Reason: corev1.PodReasonUnschedulable,
				error:  "pod unschedulable",
			},

			objs: []runtime.Object{
				&appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"test": "test"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionFalse,
							Reason: corev1.PodReasonUnschedulable,
						}},
					},
				},
			},
		},
		{
			desc:           "status-phase-running",
			conditionTypes: []string{ConditionTypeStatus + "phase-running"},
			ready:          true,

			objs: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
		},
		{
			desc:           "status-phase-pending",
			conditionTypes: []string{ConditionTypeStatus + "phase-running"},
			ready:          false,
			msg:            "Testing",

			objs: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Status: corev1.PodStatus{
						Phase:   corev1.PodPending,
						Message: "Testing",
					},
				},
			},
		},
		{
			desc:           "pod-status-not-ready",
			conditionTypes: []string{ConditionTypePodReady},
			ready:          false,
			err: &ReadinessError{
				Reason: "Error",
				error:  "container error",
			},
			objs: []runtime.Object{
				&appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"test": "test"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										Reason: "Error",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc:           "container-oom",
			conditionTypes: []string{ConditionTypePodReady},
			ready:          false,
			err: &ReadinessError{
				Reason: "OOMKilled",
				error:  "container error",
			},
			objs: []runtime.Object{
				&appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"test": "test"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								LastTerminationState: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										Reason: "OOMKilled",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ctx := context.TODO()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			// Setup a readiness checker for the test case using the supplied objects
			u := &unstructured.Unstructured{}
			if len(c.objs) > 0 {
				if err := scheme.Convert(c.objs[0], u, nil); err != nil {
					t.Fatalf("Could not convert to unstructured: %v", err)
				}
				c.objs = c.objs[1:]
			}
			rc := &ReadinessChecker{Reader: fake.NewFakeClientWithScheme(scheme, c.objs...)}

			// Verify the results
			msg, ready, err := rc.CheckConditions(ctx, u, c.conditionTypes)
			assert.Equal(t, c.ready, ready)
			assert.Equal(t, c.msg, msg)
			if c.err != nil {
				assert.EqualError(t, err, c.err.Error())
				if rerr, ok := c.err.(*ReadinessError); ok {
					if assert.IsType(t, err, &ReadinessError{}) {
						assert.Equal(t, rerr.Reason, err.(*ReadinessError).Reason)
						assert.Equal(t, rerr.Message, err.(*ReadinessError).Message)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
