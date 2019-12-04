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
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/experiment"
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExperimentReconciler reconciles an Experiment object
type ExperimentReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=list;watch;delete

func (r *ExperimentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// Fetch the Experiment instance
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, exp); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Find trials labeled for this experiment
	trialList := &redskyv1alpha1.TrialList{}
	if err := r.listTrials(ctx, trialList, exp.TrialSelector()); err != nil {
		return ctrl.Result{}, err
	}

	// Update experiment status
	if result, err := r.updateStatus(ctx, exp, trialList); result != nil {
		return *result, err
	}

	// Clean up trials
	if result, err := r.cleanupTrials(ctx, exp, trialList); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *ExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Experiment{}).
		Owns(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// updateStatus will ensure the experiment status matches the current state
func (r *ExperimentReconciler) updateStatus(ctx context.Context, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	if experiment.UpdateStatus(exp, trialList) {
		err := r.Update(ctx, exp)
		return controller.RequeueConflict(err)
	}
	return nil, nil
}

// cleanupTrials will delete any trials whose TTL has expired or are active past
func (r *ExperimentReconciler) cleanupTrials(ctx context.Context, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	for i := range trialList.Items {
		t := &trialList.Items[i]

		// Cleanup finished trials
		if trial.NeedsCleanup(t) {
			err := r.Delete(ctx, t)
			return &ctrl.Result{}, err
		}

		// If the experiment was deleted, delete the trial instead of waiting for it to finish
		if !trial.IsFinished(t) && !exp.GetDeletionTimestamp().IsZero() && t.GetDeletionTimestamp().IsZero() {
			err := r.Delete(ctx, t)
			return &ctrl.Result{}, err
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
