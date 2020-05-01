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
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/internal/controller"
	"github.com/redskyops/redskyops-controller/internal/meta"
	"github.com/redskyops/redskyops-controller/internal/trial"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TrialJobReconciler reconciles a Trial's job
type TrialJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=list;watch;create
// +kubebuilder:rbac:groups="",resources=pods,verbs=list

func (r *TrialJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil || r.ignoreTrial(t) {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// List the trial jobs (there should only ever be 0 or 1 matching jobs)
	jobList := &batchv1.JobList{}
	if err := r.listJobs(ctx, jobList, t.Namespace, t.GetJobSelector()); err != nil {
		return ctrl.Result{}, err
	}

	// Update trial status based on existing job state
	if result, err := r.updateStatus(ctx, t, jobList, &now); result != nil {
		return *result, err
	}

	// Create a new job if necessary
	if len(jobList.Items) == 0 {
		// Insert a "sleep" between "ready" and the trial job
		if ids := time.Duration(t.Spec.InitialDelaySeconds) * time.Second; ids > 0 {
			for _, c := range t.Status.Conditions {
				if c.Type == redskyv1alpha1.TrialReady {
					startTime := c.LastTransitionTime.Add(ids)
					if startTime.After(now.Time) {
						return ctrl.Result{RequeueAfter: startTime.Sub(now.Time)}, nil
					}
				}
			}
		}

		// Create the trial run job
		if result, err := r.createJob(ctx, t); result != nil {
			return *result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TrialJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("trial-job").
		For(&redskyv1alpha1.Trial{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *TrialJobReconciler) ignoreTrial(t *redskyv1alpha1.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue) {
		return true
	}

	// Ignore trials that are not ready yet
	if !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialReady, corev1.ConditionTrue) {
		return true
	}

	// Ignore trials that already have a start and completion time
	if t.Status.StartTime != nil && t.Status.CompletionTime != nil {
		return true
	}

	// Reconcile everything else
	return false
}

// updateStatus will update the trial status based on the supplied list of trial run jobs
func (r *TrialJobReconciler) updateStatus(ctx context.Context, t *redskyv1alpha1.Trial, jobList *batchv1.JobList, probeTime *metav1.Time) (*ctrl.Result, error) {
	for i := range jobList.Items {
		if update, requeue := r.applyJobStatus(ctx, t, &jobList.Items[i], probeTime); update {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		} else if requeue {
			// We are watching jobs, not pods; we may need to poll the pod state before it is consistent
			return &ctrl.Result{Requeue: true}, nil
		}
	}

	return nil, nil
}

// createJob will create a new trial run job
func (r *TrialJobReconciler) createJob(ctx context.Context, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	job := trial.NewJob(t)
	if err := controllerutil.SetControllerReference(t, job, r.Scheme); err != nil {
		return &ctrl.Result{}, err
	}

	err := r.Create(ctx, job)
	return &ctrl.Result{}, err
}

// listJobs will return all of the jobs for the trial
func (r *TrialJobReconciler) listJobs(ctx context.Context, jobList *batchv1.JobList, namespace string, selector *metav1.LabelSelector) error {
	matchingSelector, err := meta.MatchingSelector(selector)
	if err != nil {
		return err
	}
	if err := r.List(ctx, jobList, client.InNamespace(namespace), matchingSelector); err != nil {
		return err
	}

	// Setup jobs always have "role=trialSetup" so ignore jobs with that label
	// NOTE: We do not use label selectors on search because we don't know if they are user modified
	items := jobList.Items[:0]
	for i := range jobList.Items {
		if jobList.Items[i].Labels[redskyv1alpha1.LabelTrialRole] != "trialSetup" {
			items = append(items, jobList.Items[i])
		}
	}
	jobList.Items = items

	return nil
}

func (r *TrialJobReconciler) applyJobStatus(ctx context.Context, t *redskyv1alpha1.Trial, job *batchv1.Job, time *metav1.Time) (bool, bool) {
	var dirty bool

	// Get the interval of the container execution in the job pods
	startedAt := job.Status.StartTime
	finishedAt := job.Status.CompletionTime
	if matchingSelector, err := meta.MatchingSelector(job.Spec.Selector); err == nil {
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.InNamespace(job.Namespace), matchingSelector); err == nil {
			startedAt, finishedAt = containerTime(podList)

			// Check if the job has a start/completion time, but it is not yet reflected in the pod state we are seeing
			if (startedAt == nil && job.Status.StartTime != nil) || (finishedAt == nil && job.Status.CompletionTime != nil) {
				return false, true
			}

			// Look for pod failures (edge case where job controller doesn't update status properly, e.g. initContainer failure)
			for i := range podList.Items {
				s := &podList.Items[i].Status
				if s.Phase == corev1.PodFailed {
					trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, s.Reason, "", time)
					dirty = true
				}
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
