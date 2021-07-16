/*
Copyright 2020 GramLabs, Inc.

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
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/controller"
	"github.com/thestormforge/optimize-controller/v2/internal/meta"
	"github.com/thestormforge/optimize-controller/v2/internal/setup"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SetupReconciler reconciles a Trial object for setup tasks
type SetupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=trials;trials/finalizers,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=list;watch;create

func (r *SetupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &optimizev1beta2.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Update the status, return if there are no actionable setup tasks
	if !setup.UpdateStatus(t, &now) {
		return ctrl.Result{}, nil
	}

	// Update trial status based on existing setup job state
	if result, err := r.inspectSetupJobs(ctx, t, &now); result != nil {
		return *result, err
	}

	// If necessary, create the setup (create or delete) job
	if result, err := r.createSetupJob(ctx, t, &now); result != nil {
		return *result, err
	}

	// Finish
	if result, err := r.finish(ctx, t); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *SetupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO Have some type of setting to by-pass this
	return ctrl.NewControllerManagedBy(mgr).
		Named("setup").
		For(&optimizev1beta2.Trial{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// inspectSetupJobs will look for the setup jobs and update the trial status accordingly
func (r *SetupReconciler) inspectSetupJobs(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Find the setup jobs for this trial
	list := &batchv1.JobList{}
	setupJobLabels := map[string]string{optimizev1beta2.LabelTrial: t.Name, optimizev1beta2.LabelTrialRole: "trialSetup"}
	if err := r.List(ctx, list, client.InNamespace(t.Namespace), client.MatchingLabels(setupJobLabels)); err != nil {
		return &ctrl.Result{}, err
	}

	// This is purely for recovery
	if len(list.Items) == 0 {
		if t.DeletionTimestamp.IsZero() {
			// Normally if the trial hasn't been deleted and there are no jobs, the status will already be unknown
			// NOTE: Do not use ApplyCondition unless we are sure the condition is already there
			for i := range t.Status.Conditions {
				if ct := t.Status.Conditions[i].Type; ct == optimizev1beta2.TrialSetupCreated || ct == optimizev1beta2.TrialSetupDeleted {
					trial.ApplyCondition(&t.Status, ct, corev1.ConditionUnknown, "", "", probeTime)
				}
			}
		} else if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionFalse) {
			// We only need this for the delete job (the corresponding failure for the create job is handled when creating the jobs):
			// If "setup deleted" condition is "false" then we must have started job, but if the list is empty someone
			// deleted the job (e.g. the namespace was deleted while the setup delete job was running); mark the setup
			// deletion as "true" to ensure the finalizers are cleaned up.
			trial.ApplyCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionTrue, "MissingJob", "", probeTime)
		}
	}

	// Update the conditions based on existing jobs
	for i := range list.Items {
		job := &list.Items[i]

		// Inspect the job to determine which condition to update
		conditionType, err := setup.GetTrialConditionType(job)
		if err != nil {
			return &ctrl.Result{}, err
		}

		// Determine if the job is finished (i.e. completed or failed)
		conditionStatus, failureMessage := setup.GetConditionStatus(job)
		if conditionStatus == corev1.ConditionFalse {
			conditionStatus, failureMessage = r.inspectSetupJobPods(ctx, job)
		}
		trial.ApplyCondition(&t.Status, conditionType, conditionStatus, "", "", probeTime)

		// Only fail the trial itself if it isn't already finished; both to prevent overwriting an existing success
		// or failure status and to avoid updating the probe time (which would get us stuck in a busy loop)
		if failureMessage != "" && !trial.IsFinished(t) {
			trial.ApplyCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue, "SetupJobFailed", failureMessage, probeTime)
		}
	}

	// Check to see if we need to update the trial to record a condition change
	// TODO This check just looks for the probeTime in "last transition" times, is this causing unnecessary updates?
	// TODO Can we use pointer equivalence on probeTime to help mitigate that problem?
	for i := range t.Status.Conditions {
		if t.Status.Conditions[i].LastTransitionTime.Equal(probeTime) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}
	}
	return nil, nil
}

// inspectSetupJobPods will do further inspection on a job's pods to determine its current state
func (r *SetupReconciler) inspectSetupJobPods(ctx context.Context, j *batchv1.Job) (corev1.ConditionStatus, string) {
	list := &corev1.PodList{}
	if matchingSelector, err := meta.MatchingSelector(j.Spec.Selector); err == nil {
		_ = r.List(ctx, list, client.InNamespace(j.Namespace), matchingSelector)
	}

	for i := range list.Items {
		for _, cs := range list.Items[i].Status.ContainerStatuses {
			if !cs.Ready && cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				return corev1.ConditionTrue, "Setup job has a failed container"
			}
		}
	}

	return corev1.ConditionFalse, ""
}

// createSetupJob determines if a setup job is necessary and creates it
func (r *SetupReconciler) createSetupJob(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	mode := ""

	// If the created condition is unknown, we may need a create job
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupCreated, corev1.ConditionUnknown) {
		// Before we can create the job, we need an initializer/finalizer
		if trial.AddInitializer(t, setup.Initializer) || meta.AddFinalizer(t, setup.Finalizer) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}

		// Do not create setup tasks if the trial is deleted
		if t.DeletionTimestamp.IsZero() {
			mode = setup.ModeCreate
		}
	}

	// If the deleted condition is unknown, we may need a delete job
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionUnknown) {
		// We do not need the deleted job until the trial is finished or it gets deleted
		if trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
			mode = setup.ModeDelete
		}
	}

	// Create a setup job if necessary
	if mode != "" {
		job, err := setup.NewJob(t, mode)
		if err != nil {
			return &ctrl.Result{}, err
		}
		if err := controllerutil.SetControllerReference(t, job, r.Scheme); err != nil {
			return &ctrl.Result{}, err
		}
		err = r.Create(ctx, job)

		// Forbidden for a delete job indicates that namespace was probably deleted
		if apierrs.IsForbidden(err) && mode == setup.ModeDelete {
			trial.ApplyCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionTrue, "Forbidden", err.Error(), probeTime)
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}

		return &ctrl.Result{}, controller.IgnoreAlreadyExists(err)
	}

	return nil, nil
}

// finish takes care of removing initializers and finalizers
func (r *SetupReconciler) finish(ctx context.Context, t *optimizev1beta2.Trial) (*ctrl.Result, error) {
	// If the create job isn't finished, wait for it (unless the trial is already finished, i.e. failed)
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupCreated, corev1.ConditionFalse) {
		if !trial.IsFinished(t) && t.DeletionTimestamp.IsZero() {
			return &ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	// If the create job is finished, remove the initializer so the rest of the trial can run
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupCreated, corev1.ConditionTrue) {
		if trial.RemoveInitializer(t, setup.Initializer) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}
	}

	// Do not remove the finalizer until the delete job is finished
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionTrue) {
		if meta.RemoveFinalizer(t, setup.Finalizer) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}
	}

	// The trial is deleted and _both_ jobs are started but not completed; assume the trial job is misconfigured.
	if !t.DeletionTimestamp.IsZero() &&
		trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupCreated, corev1.ConditionFalse) &&
		trial.CheckCondition(&t.Status, optimizev1beta2.TrialSetupDeleted, corev1.ConditionFalse) {
		// To get into this state, just delete a trial that has a setup job with an invalid volume map (e.g. missing config map).
		// TODO Is it possible we got here because the create job just never had a chance to finish?
		if meta.RemoveFinalizer(t, setup.Finalizer) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}
	}

	return nil, nil
}
