package trial

import (
	"context"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StabilityError indicates that the cluster has not reached a sufficiently stable state
type StabilityError struct {
	// The minimum amount of time until the object is expected to stabilize, if left unspecified there is no expectation of stability
	RetryAfter time.Duration
}

func (e *StabilityError) Error() string {
	// TODO Make something nice
	return "not stable"
}

// Check a stateful set to see if it has reached a stable state
func checkStatefulSet(sts *appsv1.StatefulSet) error {
	// Same tests used by `kubectl rollout status`
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/rollout_status.go
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		// TODO Log this?
		return nil // Nothing we can do
	}
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &StabilityError{RetryAfter: 5 * time.Second}
			}
		}
		return nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	return nil
}

func checkDeployment(d *appsv1.Deployment) error {
	// Same tests used by `kubectl rollout status`
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/rollout_status.go
	if d.Generation > d.Status.ObservedGeneration {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			return &StabilityError{}
		}
	}
	if d.Spec.Replicas != nil && d.Status.UpdatedReplicas < *d.Spec.Replicas {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	if d.Status.Replicas > d.Status.UpdatedReplicas {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	if d.Status.AvailableReplicas < d.Status.UpdatedReplicas {
		return &StabilityError{RetryAfter: 5 * time.Second}
	}
	return nil
}

// Iterates over all of the supplied patches and ensures that the targets are in a "stable" state (where "stable"
// is determined by the object kind).
func waitForStableState(r client.Reader, ctx context.Context, patches []okeanosv1alpha1.PatchOperation) error {
	for _, p := range patches {
		switch p.TargetRef.Kind {
		case "StatefulSet":
			ss := &appsv1.StatefulSet{}
			if err, ok := get(r, ctx, p.TargetRef, ss); err != nil {
				if ok {
					continue
				}
				return err
			}
			if err := checkStatefulSet(ss); err != nil {
				return err
			}

		case "Deployment":
			d := &appsv1.Deployment{}
			if err, ok := get(r, ctx, p.TargetRef, d); err != nil {
				if ok {
					continue
				}
				return err
			}
			if err := checkDeployment(d); err != nil {
				return err
			}

			// TODO Should we also get DaemonSet like rollout?
		}
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
