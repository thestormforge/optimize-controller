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

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadinessChecker_CheckConditions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	t.Run("always-true", withReadinessChecker(scheme,
		func(g *WithT, rc *ReadinessChecker, u *unstructured.Unstructured) {
			msg, ok, err := rc.CheckConditions(context.TODO(), u, []string{ConditionTypeAlwaysTrue})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(ok).To(BeTrue())
			g.Expect(msg).To(BeEmpty())
		},
	))

	t.Run("pod-ready", withReadinessChecker(scheme,
		func(g *WithT, rc *ReadinessChecker, u *unstructured.Unstructured) {
			msg, ok, err := rc.CheckConditions(context.TODO(), u, []string{ConditionTypePodReady})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(ok).To(BeFalse())
			g.Expect(msg).To(BeEmpty())
		},
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
	))

	t.Run("unschedulable", withReadinessChecker(scheme,
		func(g *WithT, rc *ReadinessChecker, u *unstructured.Unstructured) {
			_, _, err := rc.CheckConditions(context.TODO(), u, []string{ConditionTypePodReady})
			g.Expect(err).Should(MatchError(corev1.PodReasonUnschedulable))
		},
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
	))
}

// withReadinessChecker wraps a ReadinessChecker test function; the first object will be converted to an unstructured
// object for use as the target object, the remaining objects will be available through the checker's reader
func withReadinessChecker(scheme *runtime.Scheme, f func(*WithT, *ReadinessChecker, *unstructured.Unstructured), objs ...runtime.Object) func(*testing.T) {
	return func(t *testing.T) {
		u := &unstructured.Unstructured{}
		if len(objs) > 0 {
			if err := scheme.Convert(objs[0], u, nil); err != nil {
				t.Fatalf("Could not convert to unstructured: %v", err)
			}
			objs = objs[1:]
		}
		reader := fake.NewFakeClientWithScheme(scheme, objs...)
		f(NewWithT(t), &ReadinessChecker{Reader: reader}, u)
	}
}
