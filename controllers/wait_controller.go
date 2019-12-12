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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/trial"
	"github.com/redskyops/k8s-experiment/internal/wait"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitReconciler checks for stability of the patched objects
type WaitReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Keep the raw API reader for doing stabilization checks. In that case we only have patch/get permissions
	// on the object and if we were to use the standard caching reader we would hang because cache itself also
	// requires list/watch. If we ever get a way to disable the cache or the cache becomes smart enough to handle
	// permission errors without hanging we can go back to using standard reader.
	apiReader client.Reader
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list

func (r *WaitReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil || r.ignoreTrial(t) {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	if result, err := r.wait(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *WaitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return ctrl.NewControllerManagedBy(mgr).
		Named("wait").
		For(&redskyv1alpha1.Trial{}).
		Complete(r)
}

func (r *WaitReconciler) ignoreTrial(t *redskyv1alpha1.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue) {
		return true
	}

	// Ignore trials whose patches have not been evaluated yet
	// NOTE: If "trial patched" is not unknown, then `len(t.Spec.PatchOperations) == 0` means the experiment had no patches
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionUnknown) {
		return true
	}

	// Ignore trials that have patches which still need to be applied
	for i := range t.Spec.PatchOperations {
		if t.Spec.PatchOperations[i].AttemptsRemaining > 0 {
			return true
		}
	}

	// Do not ignore trials that have pending waits
	for i := range t.Spec.PatchOperations {
		if t.Spec.PatchOperations[i].Wait {
			return false
		}
	}

	// Do not ignore trials if we haven't finished processing them
	if !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue) {
		return false
	}

	// Ignore everything else
	return true
}

func (r *WaitReconciler) wait(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	var requeueAfter time.Duration
	for i := range t.Spec.PatchOperations {
		p := &t.Spec.PatchOperations[i]
		if !p.Wait {
			continue
		}

		if err := r.waitFor(ctx, p); err != nil {
			// Record the largest retry delay, but continue through the list looking for show stoppers
			if serr, ok := err.(*wait.StabilityError); ok && serr.RetryAfter > 0 {
				if serr.RetryAfter > requeueAfter {
					trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "Waiting", err.Error(), probeTime)
					requeueAfter = serr.RetryAfter
				}
				continue
			}

			// Fail the trial since we couldn't detect a stable state
			trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "WaitFailed", err.Error(), probeTime)
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}

		// Mark that we have successfully waited for this patch
		p.Wait = false

		// We have started waiting, transition into a false status (without overwriting previous message)
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// Remaining patches require a delay; update the trial and adjust the response
	if requeueAfter > 0 {
		return &ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// We made it through all of the waits without needing additional changes
	trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

func (r *WaitReconciler) waitFor(ctx context.Context, p *redskyv1alpha1.PatchOperation) error {
	// TODO Should we be checking apiVersions?
	log := r.Log.WithValues("kind", p.TargetRef.Kind, "name", p.TargetRef.Name, "namespace", p.TargetRef.Namespace)

	var selector *metav1.LabelSelector
	var err error
	switch p.TargetRef.Kind {

	case "Deployment":
		d := &appsv1.Deployment{}
		if err := r.apiReader.Get(ctx, name(p.TargetRef), d); err != nil {
			return controller.IgnoreNotFound(err)
		}
		selector = d.Spec.Selector
		err = wait.CheckDeployment(d)

	case "DaemonSet":
		daemon := &appsv1.DaemonSet{}
		if err := r.apiReader.Get(ctx, name(p.TargetRef), daemon); err != nil {
			return controller.IgnoreNotFound(err)
		}
		selector = daemon.Spec.Selector
		err = wait.CheckDaemonSet(daemon)

	case "StatefulSet":
		sts := &appsv1.StatefulSet{}
		if err := r.apiReader.Get(ctx, name(p.TargetRef), sts); err != nil {
			return controller.IgnoreNotFound(err)
		}
		selector = sts.Spec.Selector
		err = wait.CheckStatefulSet(sts)

	case "ConfigMap":
	// Nothing to check

	default:
		// TODO Can we have some kind of generic condition check? Or "readiness gates"?
		// Get unstructured, look for status conditions list

		log.Info("Stability check skipped due to unsupported object kind")
	}

	if serr, ok := err.(*wait.StabilityError); ok {
		// If we are going to initiate a delay, or if we failed to check due to a legacy update strategy, try checking the pods
		if serr.RetryAfter != 0 || serr.Reason == "UpdateStrategy" {
			list := &corev1.PodList{}
			if matchingSelector, err := meta.MatchingSelector(selector); err == nil {
				_ = r.apiReader.List(ctx, list, client.InNamespace(p.TargetRef.Namespace), matchingSelector)
			}

			// Continue to ignore anything that isn't a StabilityError so we retain the original error
			if err, ok := (wait.CheckPods(list)).(*wait.StabilityError); ok {
				serr = err
			}
		}

		// Ignore legacy update strategy errors
		if serr.Reason == "UpdateStrategy" {
			return nil
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
