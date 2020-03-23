/*
Copyright 2019 GramLabs, Inc.

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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConditionTypeAlwaysTrue is a special condition type whose status is always "True"
	ConditionTypeAlwaysTrue = "redskyops.dev/always-true"
	// ConditionTypeRolloutStatus is a special condition type whose status is determined using the equivalent of a
	// `kubectl rollout status` call on the target object
	ConditionTypeRolloutStatus = "redskyops.dev/rollout-status"
	// ConditionTypePodReady is a special condition type whose status is determined by fetching the pods associated
	// with the target object and checking that they all have a condition type of "Ready" with a status of "True";
	// depending on your version of Kubernetes, you should prefer just checking "Ready" on the target object instead
	ConditionTypePodReady = "redskyops.dev/pod-ready"
)

// AllowRolloutStatus checks to see the rollout status condition is supported for the referenced type
func AllowRolloutStatus(k schema.ObjectKind) bool {
	// Make a dummy unstructured object just to test if it would work or not
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(k.GroupVersionKind())
	_, err := podSelector(u)
	return err == nil
}

// ReadinessChecker is used to check the conditions of runtime objects
type ReadinessChecker struct {
	// Reader is used to fetch information about objects related to the object whose conditions are being checked
	Reader client.Reader
}

// CheckConditions checks to see that all of the listed conditions have a status of true on the specified object. Note
// that in addition to generically checking in the `status.conditions` field, special conditions are also supported. The
// special conditions are prefixed with "redskyops.dev/".
func (r *ReadinessChecker) CheckConditions(ctx context.Context, obj *unstructured.Unstructured, conditionTypes []string) (string, bool, error) {
	for _, c := range conditionTypes {
		var msg string
		var s corev1.ConditionStatus
		var err error

		// Handle special condition types here
		switch c {
		case ConditionTypeAlwaysTrue:
			msg, s, err = "", corev1.ConditionTrue, nil
		case ConditionTypeRolloutStatus:
			msg, s, err = r.rolloutStatus(ctx, obj)
		case ConditionTypePodReady:
			msg, s, err = r.podReady(ctx, obj)
		default:
			msg, s, err = r.unstructuredConditionStatus(obj, c)
		}

		// Stop the loop if something isn't ready or encountered an error
		if s != corev1.ConditionTrue || err != nil {
			return msg, false, err
		}
	}

	// All the conditions must have been true
	return "", true, nil
}

// unstructuredConditionStatus inspects unstructured contents for the status of a condition
func (r *ReadinessChecker) unstructuredConditionStatus(obj *unstructured.Unstructured, conditionType string) (string, corev1.ConditionStatus, error) {
	s, ok := obj.UnstructuredContent()["status"].(map[string]interface{})
	if !ok {
		return "", corev1.ConditionFalse, fmt.Errorf("unable to locate status")
	}
	cl, ok := s["conditions"].([]interface{})
	if !ok {
		return "", corev1.ConditionFalse, fmt.Errorf("unable to locate conditions")
	}
	for i := range cl {
		cm, ok := cl[i].(map[string]interface{})
		if !ok {
			return "", corev1.ConditionFalse, fmt.Errorf("unable to locate condition")
		}
		if cm["type"] == conditionType {
			msg, _ := cm["message"].(string)
			switch cm["status"] {
			case string(corev1.ConditionTrue):
				return msg, corev1.ConditionTrue, nil
			case string(corev1.ConditionFalse):
				return msg, corev1.ConditionFalse, nil
			default:
				return msg, corev1.ConditionUnknown, nil
			}
		}
	}
	// This is a legitimate "unknown" case because we didn't see the condition
	return "", corev1.ConditionUnknown, nil
}

// rolloutStatus uses the kubectl implementation of rollout status to get the status of an object
func (r *ReadinessChecker) rolloutStatus(ctx context.Context, obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	// Get the kubectl status viewer for the object
	sv, err := polymorphichelpers.StatusViewerFor(obj.GetObjectKind().GroupVersionKind().GroupKind())
	if err != nil {
		return "", corev1.ConditionFalse, err
	}

	// Evaluate the status
	msg, ok, err := sv.Status(obj, 0)

	// Check OK first since it can be true even if the error is not nil
	if ok {
		// TODO This is a legacy behavior, we should stop doing it and just require that "pod-ready" also be added as a condition
		if err != nil {
			return r.podReady(ctx, obj)
		}
		return msg, corev1.ConditionTrue, nil
	}

	return msg, corev1.ConditionFalse, err
}

// podReady attempts to locate the pods associated with the specified object and
func (r *ReadinessChecker) podReady(ctx context.Context, obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	// Get the pod selector
	sel, err := podSelector(obj)
	if err != nil {
		return "", corev1.ConditionFalse, err
	}

	// Get the list of pods
	list := &corev1.PodList{}
	if err := r.Reader.List(ctx, list, client.InNamespace(obj.GetNamespace()), client.MatchingLabelsSelector{Selector: sel}); err != nil {
		return "", corev1.ConditionFalse, err
	}

	// Iterate over the pods looking for their ready state
	for i := range list.Items {
		for _, c := range list.Items[i].Status.Conditions {
			if c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue {
				return c.Message, c.Status, nil
			}
		}
	}

	// All the ready conditions were true (or there were no pods)
	return "", corev1.ConditionTrue, nil
}

// podSelector returns the label selector for pods "owned" by the specified object
func podSelector(obj *unstructured.Unstructured) (labels.Selector, error) {
	// TODO Instead of a label selector would we ever want to return a generic client.ListOption; e.g. a field selector?
	var ls *metav1.LabelSelector

	kind := obj.GetObjectKind().GroupVersionKind().GroupKind()
	switch kind {

	case extensionsv1beta1.SchemeGroupVersion.WithKind("Deployment").GroupKind(),
		appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():

		deployment := &appsv1.Deployment{}
		if err := scheme.Scheme.Convert(obj, deployment, nil); err != nil {
			return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, deployment, err)
		}
		ls = deployment.Spec.Selector

	case extensionsv1beta1.SchemeGroupVersion.WithKind("DaemonSet").GroupKind(),
		appsv1.SchemeGroupVersion.WithKind("DaemonSet").GroupKind():

		daemon := &appsv1.DaemonSet{}
		if err := scheme.Scheme.Convert(obj, daemon, nil); err != nil {
			return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, daemon, err)
		}
		ls = daemon.Spec.Selector

	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():

		sts := &appsv1.StatefulSet{}
		if err := scheme.Scheme.Convert(obj, sts, nil); err != nil {
			return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, sts, err)
		}
		ls = sts.Spec.Selector

	default:
		return nil, fmt.Errorf("no pod selection has been implemented for %v", kind)
	}

	return metav1.LabelSelectorAsSelector(ls)
}
