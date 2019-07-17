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
package trial

import (
	"context"
	"fmt"
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StabilityError indicates that the cluster has not reached a sufficiently stable state
type StabilityError struct {
	// The reason for the stability error
	Reason string
	// The object on which the stability problem was detected
	TargetRef corev1.ObjectReference
	// The minimum amount of time until the object is expected to stabilize, if left unspecified there is no expectation of stability
	RetryAfter time.Duration
}

func (e *StabilityError) Error() string {
	if e.RetryAfter > 0 {
		// This is more of an informational message then an error since the problem may resolve itself after the wait
		return fmt.Sprintf("%s/%s is not ready: %s", e.TargetRef.Kind, e.TargetRef.Name, e.Reason)
	} else {
		// This is an error, the trial will record this message in the failure
		return fmt.Sprintf("%s stability error for '%s': %s", e.TargetRef.Kind, e.TargetRef.Name, e.Reason)
	}
}

// Check a stateful set to see if it has reached a stable state
func checkStatefulSet(sts *appsv1.StatefulSet) error {
	// Same tests used by `kubectl rollout status`
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/rollout_status.go
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		log.Info("StatefulSet stability check skipped due to legacy update strategy", "name", sts.Name, "updateStrategyType", sts.Spec.UpdateStrategy.Type)
		return nil
	}
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &StabilityError{Reason: "ObservedGeneration", RetryAfter: 5 * time.Second}
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &StabilityError{Reason: "ReadyReplicas", RetryAfter: 5 * time.Second}
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &StabilityError{Reason: "UpdatedReplicas", RetryAfter: 5 * time.Second}
			}
		}
		return nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &StabilityError{Reason: "CurrentRevision", RetryAfter: 5 * time.Second}
	}
	return nil
}

func checkDeployment(d *appsv1.Deployment) error {
	// Same tests used by `kubectl rollout status`
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/rollout_status.go
	if d.Generation > d.Status.ObservedGeneration {
		return &StabilityError{Reason: "ObservedGeneration", RetryAfter: 5 * time.Second}
	}
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			return &StabilityError{Reason: "ProgressDeadlineExceeded"}
		}
	}
	if d.Spec.Replicas != nil && d.Status.UpdatedReplicas < *d.Spec.Replicas {
		return &StabilityError{Reason: "UpdatedReplicas", RetryAfter: 5 * time.Second}
	}
	if d.Status.Replicas > d.Status.UpdatedReplicas {
		return &StabilityError{Reason: "Replicas", RetryAfter: 5 * time.Second}
	}
	if d.Status.AvailableReplicas < d.Status.UpdatedReplicas {
		return &StabilityError{Reason: "AvailableReplicas", RetryAfter: 5 * time.Second}
	}
	return nil
}

// Iterates over all of the supplied patches and ensures that the targets are in a "stable" state (where "stable"
// is determined by the object kind).
func waitForStableState(r client.Reader, ctx context.Context, p *redskyv1alpha1.PatchOperation) error {
	switch p.TargetRef.Kind {
	case "StatefulSet":
		ss := &appsv1.StatefulSet{}
		if err, ok := get(r, ctx, p.TargetRef, ss); err != nil {
			if ok {
				// TODO This should be IgnoreNotFound or something like that
				return nil
			}
			return err
		}
		if err := checkStatefulSet(ss); err != nil {
			return applyTarget(checkPods(err, r, ss.Spec.Selector), &p.TargetRef)
		}

	case "Deployment":
		d := &appsv1.Deployment{}
		if err, ok := get(r, ctx, p.TargetRef, d); err != nil {
			if ok {
				// TODO This should be IgnoreNotFound or something like that
				return nil
			}
			return err
		}
		if err := checkDeployment(d); err != nil {
			return applyTarget(checkPods(err, r, d.Spec.Selector), &p.TargetRef)
		}

		// TODO Should we also get DaemonSet like rollout?
	}
	return nil
}

// Helper that executes a Get and checks for ignorable errors
func get(r client.Reader, ctx context.Context, ref corev1.ObjectReference, obj runtime.Object) (error, bool) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
		if errors.IsNotFound(err) {
			return err, true
		}
		return err, false
	}
	return nil, true
}

func applyTarget(e error, r *corev1.ObjectReference) error {
	if serr, ok := e.(*StabilityError); ok {
		r.DeepCopyInto(&serr.TargetRef)
		return serr
	}
	return e
}

func checkPods(e error, r client.Reader, selector *metav1.LabelSelector) error {
	// BE SURE TO RETURN THE ORIGINAL ERROR IN PREFERENCE TO A NEWLY CREATED ERROR

	// We are checking if we should "upgrade" from delay to a hard fail
	if serr, ok := e.(*StabilityError); !ok || serr.RetryAfter == 0 {
		return e
	}

	// We were already going to initiate a delay, so the overhead of checking pods shouldn't hurt
	var err error
	list := &corev1.PodList{}
	opts := &client.ListOptions{}
	if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(selector); err != nil {
		return e
	}
	if err = r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
		return e
	}
	for _, p := range list.Items {
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
				// TODO Is it possible this is a transient condition or something else precludes it from being fatal?
				return &StabilityError{Reason: "Unschedulable"}
			}
		}
	}
	return e
}
