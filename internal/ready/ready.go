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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConditionTypeAlwaysTrue is a special condition type whose status is always "True"
	ConditionTypeAlwaysTrue = "redskyops.dev/always-true"
	// ConditionTypePodReady is a special condition type whose status is determined by fetching the pods associated
	// with the target object and checking that they all have a condition type of "Ready" with a status of "True".
	ConditionTypePodReady = "redskyops.dev/pod-ready"
	// ConditionTypeRolloutStatus is a special condition type whose status is determined using the equivalent of a
	// `kubectl rollout status` call on the target object. This condition will return "True" when evaluated against
	// an object whose "update strategy" is not "RollingUpdate"; use the "app ready" check to perform a rollout
	// status that falls back to a pod readiness check in cases where the rollout status cannot be determined.
	ConditionTypeRolloutStatus = "redskyops.dev/rollout-status"
	// ConditionTypeAppReady is a special condition type that combines the efficiency of the rollout status check,
	// the compatibility of the pod ready check.
	ConditionTypeAppReady = "redskyops.dev/app-ready"
)

// ReadinessChecker is used to check the conditions of runtime objects
type ReadinessChecker struct {
	// Reader is used to fetch information about objects related to the object whose conditions are being checked
	Reader client.Reader
}

// ReadinessError is an error that occurs while testing for readiness, it indicates a "hard failure" and is not just
// an indicator that something is not ready (i.e. it is an unrecoverable state and will never be "ready").
type ReadinessError struct {
	// Reason is a code indicating the reason why a readiness check failed
	Reason string
	// Message is a more detailed message indicating the nature of the failure
	Message string

	error string
}

// Error returns the message
func (e *ReadinessError) Error() string {
	if e.error != "" {
		return e.error
	}
	return "readiness check failed"
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
			msg, s, err = r.alwaysTrue(obj)
		case ConditionTypePodReady:
			msg, s, err = r.podReady(ctx, obj)
		case ConditionTypeRolloutStatus:
			msg, s, err = r.rolloutStatus(obj)
		case ConditionTypeAppReady:
			msg, s, err = r.appReady(ctx, obj)
		default:
			msg, s, err = r.unstructuredConditionStatus(obj, c)
		}

		// Hard stop
		if err != nil {
			return msg, false, err
		}

		// Continue checking conditions
		if s == corev1.ConditionTrue {
			continue
		}

		// Make sure it's not a hard fail
		if err := r.podFailed(ctx, obj); err != nil {
			return "", false, err
		}

		// Stop checking as soon as a condition is not "True"
		return msg, false, nil
	}
	return "", true, nil
}

// alwaysTrue does not actually check any status and just returns true
func (r *ReadinessChecker) alwaysTrue(obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	_ = obj.GroupVersionKind() // Just to be consistent with everyone else
	return "", corev1.ConditionTrue, nil
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

// appReady performs a rollout status check and falls back to a pod ready check
func (r *ReadinessChecker) appReady(ctx context.Context, obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	// Get the kubectl status viewer for the object, if no status viewer is available, fall back to pod ready
	sv, err := polymorphichelpers.StatusViewerFor(obj.GetObjectKind().GroupVersionKind().GroupKind())
	if err != nil {
		return r.podReady(ctx, obj)
	}

	// Evaluate the status
	msg, ok, err := sv.Status(obj, 0)
	msg = strings.TrimSpace(msg)
	if ok {
		// If the object isn't supported (i.e. we are OK, but still have an error), fall back to pod ready
		if err != nil {
			return r.podReady(ctx, obj)
		}
		return msg, corev1.ConditionTrue, nil
	}
	return msg, corev1.ConditionFalse, err

}

// rolloutStatus uses the kubectl implementation of rollout status to get the status of an object
func (r *ReadinessChecker) rolloutStatus(obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	// Get the kubectl status viewer for the object
	sv, err := polymorphichelpers.StatusViewerFor(obj.GetObjectKind().GroupVersionKind().GroupKind())
	if err != nil {
		return "", corev1.ConditionFalse, err
	}

	// Evaluate the status
	msg, ok, err := sv.Status(obj, 0)
	msg = strings.TrimSpace(msg)
	if ok {
		return msg, corev1.ConditionTrue, nil
	}
	return msg, corev1.ConditionFalse, err
}

// podReady attempts to locate the pods associated with the specified object and
func (r *ReadinessChecker) podReady(ctx context.Context, obj *unstructured.Unstructured) (string, corev1.ConditionStatus, error) {
	// Get the list of pods for the object
	list, err := r.listPods(ctx, obj)
	if err != nil {
		return "", corev1.ConditionFalse, err
	}

	// Iterate over the pods looking for their ready state
	for i := range list.Items {
		rc := &corev1.PodCondition{Status: corev1.ConditionUnknown}
		for _, c := range list.Items[i].Status.Conditions {
			if c.Type == corev1.PodReady {
				rc = &c
				break
			}
		}
		if rc.Status != corev1.ConditionTrue {
			return rc.Message, rc.Status, nil
		}
	}

	// All the ready conditions were true (or there were no pods)
	return "", corev1.ConditionTrue, nil
}

// podFailed looks for pods that are obviously in a failed state and are unlikely to recover
func (r *ReadinessChecker) podFailed(ctx context.Context, obj *unstructured.Unstructured) error {
	// Get the list of pods for the object
	list, err := r.listPods(ctx, obj)
	if err != nil {
		return err
	}

	// Iterate over the pods looking for failures
	for i := range list.Items {
		p := &list.Items[i]
		for _, c := range p.Status.Conditions {
			// Check for unschedulable pods
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
				return &ReadinessError{error: "pod unschedulable", Reason: c.Reason, Message: c.Message}
			}

			// Check the container status
			var cs []corev1.ContainerStatus
			cs = append(cs, p.Status.InitContainerStatuses...)
			cs = append(cs, p.Status.ContainerStatuses...)
			for _, cc := range cs {
				if !cc.Ready {
					switch {
					case cc.State.Waiting != nil:
						if cc.RestartCount > 0 && cc.State.Waiting.Reason == "CrashLoopBackOff" {
							return &ReadinessError{error: "container crash loop back off", Reason: cc.State.Waiting.Reason, Message: cc.State.Waiting.Message}
						}

					case cc.State.Terminated != nil:
						if p.Spec.RestartPolicy == corev1.RestartPolicyNever && cc.RestartCount == 0 && cc.State.Terminated.Reason == "Error" {
							return &ReadinessError{error: "container error", Reason: cc.State.Terminated.Reason, Message: cc.State.Terminated.Message}
						}
					}
				}
			}
		}
	}

	// There are no recognizably failed pods
	return nil
}

// listPods returns the pods "owned" by the supplied unstructured object
func (r *ReadinessChecker) listPods(ctx context.Context, obj *unstructured.Unstructured) (*corev1.PodList, error) {
	// Get the pod selector
	sel, err := podSelector(obj)
	if err != nil {
		return nil, err
	}

	// Get the list of pods
	list := &corev1.PodList{}
	if sel != nil {
		err = r.Reader.List(ctx, list, client.InNamespace(obj.GetNamespace()), client.MatchingLabelsSelector{Selector: sel})
	}
	return list, err
}

// podSelector returns the label selector for pods "owned" by the specified object; returns nil if the selector could
// not be determined for the supplied object.
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
		// Return a nil selector (which is not the same as leaving `ls == nil`)
		return nil, nil
	}

	return metav1.LabelSelectorAsSelector(ls)
}
