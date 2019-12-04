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

func (r *WaitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// TODO Update RBAC
// get,list,watch,update trials
// list pods
// ...like the patch controller we rely on customer supplied "get" permissions here

func (r *WaitReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	// Fetch the Trial instance
	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// If the trial is already finished or deleted there is nothing for us to do
	if trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Wait for a stable (ish) state
	if result, err := r.wait(ctx, t, &now); result != nil {
		return *result, err
	}

	// Update the trial status
	if result, err := r.finish(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *WaitReconciler) wait(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	var requeueAfter time.Duration
	for i := range t.Spec.PatchOperations {
		p := &t.Spec.PatchOperations[i]
		if !p.Wait {
			// We have already reached stability for this patch (eventually the whole list should be in this state)
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

		// Either overwrite the "waiting" reason from an earlier iteration or change the status from "unknown" to "false"
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "", "", probeTime)

		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// Remaining patches require a delay; update the trial and adjust the response
	if requeueAfter > 0 {
		return &ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	return nil, nil
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

func (r *WaitReconciler) finish(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// TODO Remove this once we know why it is actually needed
	for _, c := range t.Status.Conditions {
		if c.Type == redskyv1alpha1.TrialStable && c.LastTransitionTime.Add(1*time.Second).After(probeTime.Time) {
			return &ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	// If there is a stable condition that is not yet true, update the status
	if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue); ok && !cc {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func name(ref corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}
}
