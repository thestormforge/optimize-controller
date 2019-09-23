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

	"github.com/go-logr/logr"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		return fmt.Sprintf("%s stability error for %s: %s", e.TargetRef.Kind, e.TargetRef.Name, e.Reason)
	}
}

// WaitForStableState checks to see if the object referenced by the supplied patch operation has stabilized.
// If stabilization has not occurred, an error is returned: errors with a delay indicate that the resource is
// not ready, errors without a delay indicate the resource is never expected to become ready.
func WaitForStableState(r client.Reader, ctx context.Context, log logr.Logger, p *redskyv1alpha1.PatchOperation) error {
	var selector *metav1.LabelSelector
	var err error

	// Same tests used by `kubectl rollout status`
	// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/polymorphichelpers/rollout_status.go
	switch p.TargetRef.Kind {

	case "Deployment":
		d := &appsv1.Deployment{}
		if err := r.Get(ctx, name(p.TargetRef), d); err != nil {
			return ignoreNotFound(err)
		}
		selector = d.Spec.Selector
		err = checkDeployment(d)

	case "DaemonSet":
		ds := &appsv1.DaemonSet{}
		if err := r.Get(ctx, name(p.TargetRef), ds); err != nil {
			return ignoreNotFound(err)
		}
		selector = ds.Spec.Selector
		err = checkDaemonSet(ds, log)

	case "StatefulSet":
		ss := &appsv1.StatefulSet{}
		if err := r.Get(ctx, name(p.TargetRef), ss); err != nil {
			return ignoreNotFound(err)
		}
		selector = ss.Spec.Selector
		err = checkStatefulSet(ss, log)

	}

	if serr, ok := err.(*StabilityError); ok {
		if serr.RetryAfter != 0 {
			// We were already going to initiate a delay, so the overhead of checking pods shouldn't hurt
			list := &corev1.PodList{}
			if matchingSelector, err := util.MatchingSelector(selector); err == nil {
				_ = r.List(ctx, list, matchingSelector)
			}

			// Continue to ignore anything that isn't a StabilityError so we retain the original error
			if err, ok := (checkPods(list)).(*StabilityError); ok {
				serr = err
			}
		}

		// Add some additional context to the error
		p.TargetRef.DeepCopyInto(&serr.TargetRef)
		return serr
	}

	return err
}

func name(ref corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}
}

func ignoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func checkPods(list *corev1.PodList) error {
	for i := range list.Items {
		p := &list.Items[i]
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
				return &StabilityError{Reason: c.Reason}
			}
		}
		for _, c := range p.Status.ContainerStatuses {
			if !c.Ready && c.RestartCount > 0 && c.State.Waiting != nil && c.State.Waiting.Reason == "CrashLoopBackOff" {
				return &StabilityError{Reason: c.State.Waiting.Reason}
			}
		}
	}
	return nil
}

func checkDeployment(deployment *appsv1.Deployment) error {
	if deployment.Generation > deployment.Status.ObservedGeneration {
		return &StabilityError{Reason: "ObservedGeneration", RetryAfter: 5 * time.Second}
	}
	for _, c := range deployment.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			return &StabilityError{Reason: "ProgressDeadlineExceeded"}
		}
	}
	if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
		return &StabilityError{Reason: "UpdatedReplicas", RetryAfter: 5 * time.Second}
	}
	if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
		return &StabilityError{Reason: "Replicas", RetryAfter: 5 * time.Second}
	}
	if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
		return &StabilityError{Reason: "AvailableReplicas", RetryAfter: 5 * time.Second}
	}
	return nil
}

func checkDaemonSet(daemon *appsv1.DaemonSet, log logr.Logger) error {
	if daemon.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		// TODO Can we still do something to test this?
		log.Info("DaemonSet stability check skipped due to legacy update strategy", "name", daemon.Name, "updateStrategyType", daemon.Spec.UpdateStrategy.Type)
		return nil
	}
	if daemon.Generation > daemon.Status.ObservedGeneration {
		return &StabilityError{Reason: "ObservedGeneration", RetryAfter: 5 * time.Second}
	}
	if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
		return &StabilityError{Reason: "NumberScheduled", RetryAfter: 5 * time.Second}
	}
	if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
		return &StabilityError{Reason: "NumberAvailable", RetryAfter: 5 * time.Second}
	}
	return nil
}

// Check a stateful set to see if it has reached a stable state
func checkStatefulSet(sts *appsv1.StatefulSet, log logr.Logger) error {
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		// TODO Can we still do something to test this?
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
