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

package wait

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"time"
)

// This mostly uses the same tests as `kubectl rollout status`
// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/polymorphichelpers/rollout_status.go

// StabilityError indicates that the cluster has not reached a sufficiently stable state
type StabilityError struct {
	// The reason for the stability error
	Reason string
	// The object on which the stability problem was detected
	TargetRef corev1.ObjectReference
	// The minimum amount of time until the object is expected to stabilize, if left unspecified there is no expectation of stability
	RetryAfter time.Duration
}

// Error returns the summary about why the stability check failed
func (e *StabilityError) Error() string {
	if e.RetryAfter > 0 {
		// This is more of an informational message then an error since the problem may resolve itself after the wait
		return fmt.Sprintf("%s/%s is not ready: %s", e.TargetRef.Kind, e.TargetRef.Name, e.Reason)
	}

	// This is an error, the trial will record this message in the failure
	return fmt.Sprintf("%s stability error for %s: %s", e.TargetRef.Kind, e.TargetRef.Name, e.Reason)
}

// CheckPods inspects a pod list for stability
func CheckPods(list *corev1.PodList) error {
	// NOTE: We loop through the pods twice, look for hard failures first so they are not masked by something that just needs a wait

	// First loop, look for hard failures
	for i := range list.Items {
		p := &list.Items[i]
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
				return &StabilityError{Reason: c.Reason}
			}
		}
		if err := checkContainerStatus(p.Status.InitContainerStatuses, p.Spec.RestartPolicy); err != nil {
			return err
		}
		if err := checkContainerStatus(p.Status.ContainerStatuses, p.Spec.RestartPolicy); err != nil {
			return err
		}
	}

	// Do a second loop, this time look for things that may just need more time
	for i := range list.Items {
		p := &list.Items[i]
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionFalse {
				return &StabilityError{Reason: c.Reason, RetryAfter: 5 * time.Second}
			}
		}
	}
	return nil
}

func checkContainerStatus(cs []corev1.ContainerStatus, restartPolicy corev1.RestartPolicy) error {
	for _, c := range cs {
		if c.Ready {
			continue
		}
		if c.RestartCount > 0 && c.State.Waiting != nil && c.State.Waiting.Reason == "CrashLoopBackOff" {
			return &StabilityError{Reason: c.State.Waiting.Reason}
		}
		if restartPolicy == corev1.RestartPolicyNever && c.RestartCount == 0 && c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
			return &StabilityError{Reason: c.State.Terminated.Reason}
		}
	}
	return nil
}

// CheckDeployment inspects a deployment for stability
func CheckDeployment(deployment *appsv1.Deployment) error {
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

// CheckDaemonSet inspects a daemon set for stability
func CheckDaemonSet(daemon *appsv1.DaemonSet) error {
	if daemon.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return &StabilityError{Reason: "UpdateStrategy"}
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

// CheckStatefulSet inspects a stateful set for stability
func CheckStatefulSet(sts *appsv1.StatefulSet) error {
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		return &StabilityError{Reason: "UpdateStrategy"}
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
