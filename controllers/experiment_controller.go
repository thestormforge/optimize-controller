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

	"github.com/go-logr/logr"
	"github.com/redskyops/redskyops-controller/internal/controller"
	"github.com/redskyops/redskyops-controller/internal/experiment"
	"github.com/redskyops/redskyops-controller/internal/meta"
	"github.com/redskyops/redskyops-controller/internal/trial"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ExperimentReconciler reconciles an Experiment object
type ExperimentReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=list;watch;update;delete

func (r *ExperimentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, exp); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	trialList := &redskyv1alpha1.TrialList{}
	if err := r.listTrials(ctx, trialList, exp.TrialSelector()); err != nil {
		return ctrl.Result{}, err
	}

	if result, err := r.updateStatus(ctx, exp, trialList); result != nil {
		return *result, err
	}

	if result, err := r.updateTrialStatus(ctx, trialList); result != nil {
		return *result, err
	}

	if result, err := r.cleanupTrials(ctx, exp, trialList); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *ExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("experiment").
		For(&redskyv1alpha1.Experiment{}).
		Watches(&source.Kind{Type: &redskyv1alpha1.Trial{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(trialToExperimentRequest)}).
		Complete(r)
}

// trialToExperimentRequest extracts the reconcile request for an experiment of a trial
func trialToExperimentRequest(o handler.MapObject) []reconcile.Request {
	if t, ok := o.Object.(*redskyv1alpha1.Trial); ok {
		return []reconcile.Request{{NamespacedName: t.ExperimentNamespacedName()}}
	}
	return nil
}

// updateStatus will ensure the experiment and trial status matches the current state
func (r *ExperimentReconciler) updateStatus(ctx context.Context, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	var dirty bool

	// Update the HasTrialFinalizer
	if len(trialList.Items) > 0 {
		dirty = meta.AddFinalizer(exp, experiment.HasTrialFinalizer) || dirty
	} else {
		dirty = meta.RemoveFinalizer(exp, experiment.HasTrialFinalizer) || dirty
	}

	// Update the experiment status
	dirty = experiment.UpdateStatus(exp, trialList) || dirty

	// Only send an update if something actually changed
	if dirty {
		if err := r.Update(ctx, exp); err != nil {
			return controller.RequeueConflict(err)
		}
	}
	return nil, nil
}

// updateTrialStatus will update the status of all the experiment trials
func (r *ExperimentReconciler) updateTrialStatus(ctx context.Context, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	for i := range trialList.Items {
		t := &trialList.Items[i]

		var dirty bool

		// If the trial is not finished, but it has been observed, mark it as complete
		if !trial.IsFinished(t) && trial.CheckCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue) {
			now := metav1.Now()
			trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialComplete, corev1.ConditionTrue, "", "", &now)
			dirty = true
		}

		// Update the trial status
		dirty = trial.UpdateStatus(t) || dirty

		// Only send an update if something actually changed
		if dirty {
			if err := r.Update(ctx, t); err != nil {
				return controller.RequeueConflict(err)
			}
		}
	}
	return nil, nil
}

// cleanupTrials will delete any trials whose TTL has expired or are active past
func (r *ExperimentReconciler) cleanupTrials(ctx context.Context, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	for i := range trialList.Items {
		t := &trialList.Items[i]

		// Trial is already deleted, no clean up possible
		if !t.GetDeletionTimestamp().IsZero() {
			continue
		}

		// Delete trials if they have expired or if the experiment has been deleted
		if trial.NeedsCleanup(t) || !exp.GetDeletionTimestamp().IsZero() {
			// TODO client.PropagationPolicy(metav1.DeletePropagationBackground) ?
			if err := r.Delete(ctx, t); err != nil {
				return &ctrl.Result{}, err
			}
		}
	}
	return nil, nil
}

// listTrials retrieves the list of trial objects matching the specified selector
func (r *ExperimentReconciler) listTrials(ctx context.Context, trialList *redskyv1alpha1.TrialList, selector *metav1.LabelSelector) error {
	matchingSelector, err := meta.MatchingSelector(selector)
	if err != nil {
		return err
	}
	return r.List(ctx, trialList, matchingSelector)
}
