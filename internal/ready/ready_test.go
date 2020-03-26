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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadinessChecker_CheckConditions(t *testing.T) {
	g := NewWithT(t)
	rc := &ReadinessChecker{}

	ctx := context.TODO()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	var u *unstructured.Unstructured
	var msg string
	var ok bool
	var err error

	// As long as the object is not nil, we should get true
	u = &unstructured.Unstructured{}
	msg, ok, err = rc.CheckConditions(ctx, u, []string{ConditionTypeAlwaysTrue})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ok).To(BeTrue())
	g.Expect(msg).To(BeEmpty())

	// Create a stateful set and pod for testing
	rc.Reader, u = testObjects(t, scheme,
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
	)
	msg, ok, err = rc.CheckConditions(ctx, u, []string{ConditionTypePodReady})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ok).To(BeTrue())
	g.Expect(msg).To(BeEmpty())

	// Create a deployment and pod for testing
	rc.Reader, u = testObjects(t, scheme,
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
	)
	msg, ok, err = rc.CheckConditions(ctx, u, []string{ConditionTypePodReady})
	g.Expect(err).Should(MatchError(corev1.PodReasonUnschedulable))
}

// testObjects returns the first object converted to an unstructured object and a client reader for the remaining objects
func testObjects(t *testing.T, scheme *runtime.Scheme, objs ...runtime.Object) (client.Reader, *unstructured.Unstructured) {
	var u *unstructured.Unstructured
	var initObjs []runtime.Object
	for i := range objs {
		if u != nil {
			initObjs = append(initObjs, objs[i])
		} else {
			u = &unstructured.Unstructured{}
			if err := scheme.Convert(objs[i], u, nil); err != nil {
				t.Fatalf("Could not convert to unstructured: %v", err)
				return nil, nil
			}
		}
	}
	return fake.NewFakeClientWithScheme(scheme, initObjs...), u
}
