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

// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=list;watch;create
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=services,verbs=list

func (r *TrialReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	// Fetch the Trial instance
	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Update the status if necessary
	if result, err := r.updateStatus(ctx, t); result != nil {
		return *result, err
	}

	// If the trial is being initialized, or is already finished or deleted there is nothing for us to do
	if t.HasInitializer() || trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// List the trial jobs; really, there should only ever be 0 or 1 matching jobs
	jobList, err := r.listJobs(ctx, t)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the trial status using the job status
	if result, err := r.updateStatusFromJob(ctx, t, jobList, &now); result != nil {
		return *result, err
	}

	// If there are no trial jobs, try to create one
	if len(jobList.Items) == 0 {
		if result, err := r.createJob(ctx, t); result != nil {
			return *result, err
		}
	}

	// Nothing changed
	return ctrl.Result{}, nil
}

func (r *TrialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("trial").
		For(&redskyv1alpha1.Trial{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// updateStatus will ensure the trial status matches the current state
func (r *TrialReconciler) updateStatus(ctx context.Context, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	if trial.UpdateStatus(t) {
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}
	return nil, nil
}

func (r *TrialReconciler) updateStatusFromJob(ctx context.Context, t *redskyv1alpha1.Trial, jobList *batchv1.JobList, probeTime *metav1.Time) (*ctrl.Result, error) {
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

func (r *TrialReconciler) createJob(ctx context.Context, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	// TODO This is all basically to avoid a race condition creating the job too early
	// TODO We should always add unknown so we can just return when the condition == ConditionFalse
	if len(t.Spec.SetupTasks) > 0 {
		if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialSetupCreated, corev1.ConditionTrue); !ok || !cc {
			return nil, nil
		}
	}
	if len(t.Spec.PatchOperations) > 0 {
		if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue); !ok || !cc {
			return nil, nil
		}
		if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue); !ok || !cc {
			return nil, nil
		}
	}

	job := trial.NewJob(t)
	if err := controllerutil.SetControllerReference(t, job, r.Scheme); err != nil {
		return &ctrl.Result{}, err
	}

	err := r.Create(ctx, job)
	return &ctrl.Result{}, err
}

// listJobs will return all of the jobs for the trial
func (r *TrialReconciler) listJobs(ctx context.Context, t *redskyv1alpha1.Trial) (*batchv1.JobList, error) {
	jobList := &batchv1.JobList{}
	matchingSelector, err := meta.MatchingSelector(t.GetJobSelector())
	if err != nil {
		return nil, err
	}
	if err := r.List(ctx, jobList, matchingSelector); err != nil {
		return nil, err
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

	return jobList, nil
}

func (r *TrialReconciler) applyJobStatus(ctx context.Context, t *redskyv1alpha1.Trial, job *batchv1.Job, time *metav1.Time) (bool, bool) {
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

	// TODO Also set TrialCompleted, but not until the metric controller is updated to deal with that

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
