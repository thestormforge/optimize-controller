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
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TrialReconciler reconciles a Trial object
type TrialReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *TrialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Trial{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;services,verbs=list
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;create;update;patch;delete

func (r *TrialReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("trial", req.NamespacedName)
	now := metav1.Now()

	// Fetch the Trial instance
	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// If we are finished or deleted there is nothing for us to do
	if trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Find jobs labeled for this trial
	list := &batchv1.JobList{}
	matchingSelector, err := meta.MatchingSelector(t.GetJobSelector())
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.List(ctx, list, matchingSelector); err != nil {
		return ctrl.Result{}, err
	}

	// Update the trial run status using the job status
	needsJob := true
	for i := range list.Items {
		// Setup jobs always have "role=trialSetup" so ignore jobs with that label
		// NOTE: We do not use label selectors on search because we don't know if they are user modified
		if list.Items[i].Labels[redskyv1alpha1.LabelTrialRole] != "trialSetup" {
			if update, requeue := applyJobStatus(r, t, &list.Items[i], &now); update {
				return r.forTrialUpdate(t, ctx, log)
			} else if requeue {
				// We are watching jobs, not pods; we may need to poll the pod state before it is consistent
				return ctrl.Result{Requeue: true}, nil
			}
			needsJob = false
		}
	}

	// Create a trial run job if needed
	if needsJob {
		job := trial.NewJob(t)
		if err := controllerutil.SetControllerReference(t, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, job)
		return ctrl.Result{}, err
	}

	// Nothing changed
	return ctrl.Result{}, nil
}

// Returns from the reconcile loop after updating the supplied trial instance
func (r *TrialReconciler) forTrialUpdate(t *redskyv1alpha1.Trial, ctx context.Context, log logr.Logger) (ctrl.Result, error) {
	// If we are going to be updating the trial, make sure the status is synchronized (ignore errors)
	_ = trial.UpdateStatus(t)

	result, err := controller.RequeueConflict(r.Update(ctx, t))
	return *result, err
}

func applyJobStatus(r client.Reader, t *redskyv1alpha1.Trial, job *batchv1.Job, time *metav1.Time) (bool, bool) {
	var dirty bool

	// Get the interval of the container execution in the job pods
	startedAt := job.Status.StartTime
	finishedAt := job.Status.CompletionTime
	if matchingSelector, err := meta.MatchingSelector(job.Spec.Selector); err == nil {
		pods := &corev1.PodList{}
		if err := r.List(context.TODO(), pods, matchingSelector, client.InNamespace(job.Namespace)); err == nil {
			startedAt, finishedAt = containerTime(pods)

			// Check if the job has a start/completion time, but it is not yet reflected in the pod state we are seeing
			if (startedAt == nil && job.Status.StartTime != nil) || (finishedAt == nil && job.Status.CompletionTime != nil) {
				return false, true
			}
		}
	}

	// Adjust the trial start time
	if startTime, updated := latestTime(t.Status.StartTime, startedAt, t.Spec.StartTimeOffset); updated {
		t.Status.StartTime = startTime
		dirty = true
	}

	// Adjust the trial completion time
	if completionTime, updated := earliestTime(t.Status.CompletionTime, finishedAt); updated {
		t.Status.CompletionTime = completionTime
		dirty = true
	}

	// Mark the trial as failed if the job itself failed
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, c.Reason, c.Message, time)
			dirty = true
		}
	}

	return dirty, false
}

func containerTime(pods *corev1.PodList) (startedAt *metav1.Time, finishedAt *metav1.Time) {
	for i := range pods.Items {
		for j := range pods.Items[i].Status.ContainerStatuses {
			s := &pods.Items[i].Status.ContainerStatuses[j].State
			if s.Running != nil {
				startedAt, _ = earliestTime(startedAt, &s.Running.StartedAt)
			} else if s.Terminated != nil {
				startedAt, _ = earliestTime(startedAt, &s.Terminated.StartedAt)
				finishedAt, _ = latestTime(finishedAt, &s.Terminated.FinishedAt, nil)
			}
		}
	}
	return
}

func earliestTime(c, n *metav1.Time) (*metav1.Time, bool) {
	if n != nil && (c == nil || n.Before(c)) {
		return n.DeepCopy(), true
	}
	return c, false
}

func latestTime(c, n *metav1.Time, offset *metav1.Duration) (*metav1.Time, bool) {
	if n != nil && (c == nil || c.Before(n)) {
		if offset != nil {
			t := metav1.NewTime(n.Add(offset.Duration))
			return &t, true
		}
		return n.DeepCopy(), true
	}
	return c, false
}
