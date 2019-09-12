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
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/template"
	redskytrial "github.com/redskyops/k8s-experiment/pkg/controller/trial"
	"github.com/redskyops/k8s-experiment/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments;statefulsets,verbs=get;list;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=services,verbs=list
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials/status,verbs=get;update;patch

func (r *TrialReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("trial", req.NamespacedName)
	now := metav1.Now()

	// Fetch the Trial instance
	trial := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, trial); err != nil {
		return util.IgnoreNotFound(err)
	}

	// Ahead of everything is the setup/teardown (contains finalization logic)
	if result, err := redskytrial.ManageSetup(r.Client, r.Scheme, ctx, &now, trial); result != nil {
		if err != nil {
			log.Error(err, "Setup task failed")
		}
		return *result, err
	}

	// If we are in a finished or deleted state there is nothing for us to do
	if redskytrial.IsTrialFinished(trial) || !trial.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Copy the patches over from the experiment
	if len(trial.Spec.PatchOperations) == 0 {
		e := &redskyv1alpha1.Experiment{}
		if err := r.Get(ctx, trial.ExperimentNamespacedName(), e); err != nil {
			return ctrl.Result{}, err
		}
		if err := checkAssignments(trial, e, log); err != nil {
			return ctrl.Result{}, err
		}
		if err := evaluatePatches(r, trial, e); err != nil {
			return ctrl.Result{}, err
		}
		if len(trial.Spec.PatchOperations) > 0 {
			// We know we have at least one patch to apply, use an unknown status until we start applying them
			redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionUnknown, "", "", &now)
			return r.forTrialUpdate(trial, ctx, log)
		}
	}

	// Check the "initializer" annotation, do not progress unless it is empty (don't requeue, wait for a change)
	if trial.HasInitializer() {
		return ctrl.Result{}, nil
	}

	// Apply the patches
	for i := range trial.Spec.PatchOperations {
		p := &trial.Spec.PatchOperations[i]
		if p.AttemptsRemaining == 0 {
			continue
		}

		u := unstructured.Unstructured{}
		u.SetName(p.TargetRef.Name)
		u.SetNamespace(p.TargetRef.Namespace)
		u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
		if err := r.Patch(ctx, &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
			p.AttemptsRemaining = p.AttemptsRemaining - 1
			if p.AttemptsRemaining == 0 {
				// There are no remaining patch attempts remaining, fail the trial
				redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "PatchFailed", err.Error(), &now)
			}
		} else {
			p.AttemptsRemaining = 0
			if p.Wait {
				// We successfully applied a patch that requires a wait, use an unknown status until we start waiting
				redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionUnknown, "", "", &now)
			}
		}

		// We have started applying patches (success or fail), transition into a false status
		redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionFalse, "", "", &now)
		return r.forTrialUpdate(trial, ctx, log)
	}

	// If there is a patched condition that is not yet true, update the status
	if cc, ok := redskytrial.CheckCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue); ok && !cc {
		redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue, "", "", &now)
		return r.forTrialUpdate(trial, ctx, log)
	}

	// Wait for a stable (ish) state
	for i := range trial.Spec.PatchOperations {
		p := &trial.Spec.PatchOperations[i]
		if !p.Wait {
			continue
		}

		var requeueAfter time.Duration
		if err := redskytrial.WaitForStableState(r.Client, ctx, log, p); err != nil {
			if serr, ok := err.(*redskytrial.StabilityError); ok && serr.RetryAfter > 0 {
				// Mark the trial as not stable and wait
				redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "Waiting", err.Error(), &now)
				requeueAfter = serr.RetryAfter
			} else {
				// No retry delay specified, fail the whole trial
				redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "WaitFailed", err.Error(), &now)
			}
		} else {
			// We have successfully waited for one patch so we are no longer "unknown"
			redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "", "", &now)
			p.Wait = false
		}

		// Inject the retry delay if necessary
		rr, re := r.forTrialUpdate(trial, ctx, log)
		if re == nil && requeueAfter > 0 {
			rr.RequeueAfter = requeueAfter
		}
		return rr, re
	}

	// If there is a stable condition that is not yet true, update the status
	if cc, ok := redskytrial.CheckCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue); ok && !cc {
		redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue, "", "", &now)
		return r.forTrialUpdate(trial, ctx, log)
	}

	// Find jobs labeled for this trial
	list := &batchv1.JobList{}
	matchingSelector, err := util.MatchingSelector(trial.GetJobSelector())
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
		if list.Items[i].Labels[redskyv1alpha1.LabelTrialRole] != "trialSetup" {
			if applyJobStatus(trial, &list.Items[i], &now) {
				return r.forTrialUpdate(trial, ctx, log)
			}
			needsJob = false
		}
	}

	// Create a trial run job if needed
	if needsJob {
		job := createJob(trial)
		if err := controllerutil.SetControllerReference(trial, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, job)
		return ctrl.Result{}, err
	}

	// The completion time will be non-nil as soon as the (a?) trial run job finishes
	if trial.Status.CompletionTime != nil {
		e := &redskyv1alpha1.Experiment{}
		if err = r.Get(ctx, trial.ExperimentNamespacedName(), e); err != nil {
			return ctrl.Result{}, err
		}

		// If we have metrics to collect, use an unknown status to fill the gap (e.g. TCP timeout) until the transition to false
		if len(e.Spec.Metrics) > 0 {
			if _, ok := redskytrial.CheckCondition(&trial.Status, redskyv1alpha1.TrialObserved, corev1.ConditionUnknown); !ok {
				redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialObserved, corev1.ConditionUnknown, "", "", &now)
				return r.forTrialUpdate(trial, ctx, log)
			}
		}

		// Look for metrics that have not been collected yet
		for _, m := range e.Spec.Metrics {
			v := findOrCreateValue(trial, m.Name)
			if v.AttemptsRemaining == 0 {
				continue
			}

			urls, verr := findMetricTargets(r, &m)
			for _, u := range urls {
				if value, stddev, retryAfter, err := redskytrial.CaptureMetric(&m, u, trial); err != nil {
					verr = err
				} else if retryAfter > 0 {
					// Do not count retries against the remaining attempts, do not look for additional URLs
					return ctrl.Result{RequeueAfter: retryAfter}, nil
				} else if math.IsNaN(value) || math.IsNaN(stddev) {
					verr = fmt.Errorf("capturing metric %s got NaN", m.Name)
				} else {
					v.AttemptsRemaining = 0
					v.Value = strconv.FormatFloat(value, 'f', -1, 64)
					if stddev != 0 {
						v.Error = strconv.FormatFloat(stddev, 'f', -1, 64)
					}
					break
				}
			}

			if verr != nil && v.AttemptsRemaining > 0 {
				v.AttemptsRemaining = v.AttemptsRemaining - 1
				if v.AttemptsRemaining == 0 {
					redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "MetricFailed", verr.Error(), &now)
				}
			}

			redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialObserved, corev1.ConditionFalse, "", "", &now)
			return r.forTrialUpdate(trial, ctx, log)
		}

		// If all of the metrics are collected, finish the observation
		if cc, ok := redskytrial.CheckCondition(&trial.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue); ok && !cc {
			redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue, "", "", &now)
		}

		// Mark the trial as completed
		redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialComplete, corev1.ConditionTrue, "", "", &now)
		return r.forTrialUpdate(trial, ctx, log)
	}

	// If nothing changed, check again
	return ctrl.Result{Requeue: true}, nil
}

// Returns from the reconcile loop after updating the supplied trial instance
func (r *TrialReconciler) forTrialUpdate(trial *redskyv1alpha1.Trial, ctx context.Context, log logr.Logger) (ctrl.Result, error) {
	// If we are going to be updating the trial, make sure the status is synchronized
	assignments := make([]string, len(trial.Spec.Assignments))
	for i := range trial.Spec.Assignments {
		assignments[i] = fmt.Sprintf("%s=%d", trial.Spec.Assignments[i].Name, trial.Spec.Assignments[i].Value)
	}
	trial.Status.Assignments = strings.Join(assignments, ", ")

	values := make([]string, len(trial.Spec.Values))
	for i := range trial.Spec.Values {
		if trial.Spec.Values[i].AttemptsRemaining == 0 {
			values[i] = fmt.Sprintf("%s=%s", trial.Spec.Values[i].Name, trial.Spec.Values[i].Value)
		}
	}
	trial.Status.Values = strings.Join(values, ", ")

	err := r.Update(ctx, trial)
	return util.IgnoreConflict(err)
}

func evaluatePatches(r client.Reader, trial *redskyv1alpha1.Trial, e *redskyv1alpha1.Experiment) error {
	te := template.NewTemplateEngine()
	for _, p := range e.Spec.Patches {
		// Determine the patch type
		var pt types.PatchType
		switch p.Type {
		case redskyv1alpha1.PatchStrategic, "":
			pt = types.StrategicMergePatchType
		case redskyv1alpha1.PatchMerge:
			pt = types.MergePatchType
		case redskyv1alpha1.PatchJSON:
			pt = types.JSONPatchType
		default:
			return fmt.Errorf("unknown patch type: %s", p.Type)
		}

		// Evaluate the patch template
		data, err := te.RenderPatch(&p, trial)
		if err != nil {
			return err
		}

		// Find the targets to apply the patch to
		targets, err := findPatchTargets(r, &p, trial)
		if err != nil {
			return err
		}

		// If the patch is effectively null, we do not need to evaluate it
		attempts := 3
		if len(data) == 0 || string(data) == "null" {
			attempts = 0
		}

		// For each target resource, record a copy of the patch
		for _, ref := range targets {
			trial.Spec.PatchOperations = append(trial.Spec.PatchOperations, redskyv1alpha1.PatchOperation{
				TargetRef:         ref,
				PatchType:         pt,
				Data:              data,
				AttemptsRemaining: attempts,
				Wait:              true,
			})
		}
	}

	return nil
}

// Finds the patch targets
func findPatchTargets(r client.Reader, p *redskyv1alpha1.PatchTemplate, trial *redskyv1alpha1.Trial) ([]corev1.ObjectReference, error) {
	if trial.Spec.TargetNamespace == "" {
		trial.Spec.TargetNamespace = trial.Namespace
	}

	var targets []corev1.ObjectReference
	if p.TargetRef.Name == "" {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
		inNamespace := client.InNamespace(p.TargetRef.Namespace)
		if inNamespace == "" {
			inNamespace = client.InNamespace(trial.Spec.TargetNamespace)
		}
		matchingSelector, err := util.MatchingSelector(p.Selector)
		if err != nil {
			return nil, err
		}
		if err := r.List(context.TODO(), list, inNamespace, matchingSelector); err != nil {
			return nil, err
		}

		for _, item := range list.Items {
			// TODO There isn't a function that does this?
			targets = append(targets, corev1.ObjectReference{
				Kind:       item.GetKind(),
				Name:       item.GetName(),
				Namespace:  item.GetNamespace(),
				APIVersion: item.GetAPIVersion(),
			})
		}
	} else {
		ref := p.TargetRef.DeepCopy()
		if ref.Namespace == "" {
			ref.Namespace = trial.Spec.TargetNamespace
		}
		targets = []corev1.ObjectReference{*ref}
	}

	return targets, nil
}

func checkAssignments(trial *redskyv1alpha1.Trial, experiment *redskyv1alpha1.Experiment, log logr.Logger) error {
	// Index the assignments
	assignments := make(map[string]int64, len(trial.Spec.Assignments))
	for _, a := range trial.Spec.Assignments {
		assignments[a.Name] = a.Value
	}

	// Verify against the parameter specifications
	var missing []string
	for _, p := range experiment.Spec.Parameters {
		if a, ok := assignments[p.Name]; ok {
			if a < p.Min || a > p.Max {
				log.Info("Assignment out of bounds", "trialName", trial.Name, "parameterName", p.Name, "assignment", a, "min", p.Min, "max", p.Max)
			}
		} else {
			missing = append(missing, p.Name)
		}
	}

	// Fail if there are missing assignments
	if len(missing) > 0 {
		return fmt.Errorf("trial %s is missing assignments for %s", trial.Name, strings.Join(missing, ", "))
	}
	return nil
}

func findMetricTargets(r client.Reader, m *redskyv1alpha1.Metric) ([]string, error) {
	// Local metrics don't need to resolve service URLs
	if m.Type == redskyv1alpha1.MetricLocal || m.Type == "" {
		return []string{""}, nil
	}

	// Get the URL components that are independent of the service
	// TODO If m.Scheme == "file" we actually want to get pods so we can access the file system
	scheme := strings.ToLower(m.Scheme)
	if scheme == "" {
		scheme = "http"
	} else if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("scheme must be 'http' or 'https': %s", scheme)
	}
	path := "/" + strings.TrimLeft(m.Path, "/")

	// Find services matching the selector
	list := &corev1.ServiceList{}
	matchingSelector, err := util.MatchingSelector(m.Selector)
	if err != nil {
		return nil, err
	}
	if err := r.List(context.TODO(), list, matchingSelector); err != nil {
		return nil, err
	}

	// Construct a URL for each service (use IP literals instead of host names to avoid DNS lookups)
	var urls []string
	for _, s := range list.Items {
		host := s.Spec.ClusterIP
		port := m.Port.IntValue()

		if port < 1 {
			portName := m.Port.StrVal
			// TODO Default an empty portName to scheme?
			for _, sp := range s.Spec.Ports {
				if sp.Name == portName || len(s.Spec.Ports) == 1 {
					port = int(sp.Port)
				}
			}
		}

		if port < 1 {
			return nil, fmt.Errorf("metric '%s' has unresolvable port: %s", m.Name, m.Port.String())
		}

		urls = append(urls, fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path))
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("unable to find metric targets for '%s'", m.Name)
	}
	return urls, nil
}

func findOrCreateValue(trial *redskyv1alpha1.Trial, name string) *redskyv1alpha1.Value {
	for i := range trial.Spec.Values {
		if trial.Spec.Values[i].Name == name {
			return &trial.Spec.Values[i]
		}
	}

	trial.Spec.Values = append(trial.Spec.Values, redskyv1alpha1.Value{Name: name, AttemptsRemaining: 3})
	return &trial.Spec.Values[len(trial.Spec.Values)-1]
}

func applyJobStatus(trial *redskyv1alpha1.Trial, job *batchv1.Job, time *metav1.Time) bool {
	var dirty bool

	if trial.Status.StartTime == nil {
		// Establish a start time if available
		trial.Status.StartTime = job.Status.StartTime.DeepCopy()
		dirty = dirty || job.Status.StartTime != nil
		if dirty && trial.Spec.StartTimeOffset != nil {
			*trial.Status.StartTime = metav1.NewTime(trial.Status.StartTime.Add(trial.Spec.StartTimeOffset.Duration))
		}
	} else if job.Status.StartTime != nil && trial.Status.StartTime.Before(job.Status.StartTime) {
		// Move the start time back
		trial.Status.StartTime = job.Status.StartTime.DeepCopy()
		dirty = true
	}

	if trial.Status.CompletionTime == nil {
		// Establish an end time if available
		trial.Status.CompletionTime = job.Status.CompletionTime.DeepCopy()
		dirty = dirty || job.Status.CompletionTime != nil
	} else if job.Status.CompletionTime != nil && trial.Status.CompletionTime.Before(job.Status.CompletionTime) {
		// Move the completion time back
		trial.Status.CompletionTime = job.Status.CompletionTime.DeepCopy()
		dirty = true
	}

	// Mark the trial as failed if the job itself failed
	for _, c := range job.Status.Conditions {
		// If activeDeadlineSeconds was used a workaround for having a sidecar, ignore the failure
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue && c.Reason != "DeadlineExceeded" {
			redskytrial.ApplyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, c.Reason, c.Message, time)
			dirty = true
		}
	}

	return dirty
}

func createJob(trial *redskyv1alpha1.Trial) *batchv1.Job {
	job := &batchv1.Job{}

	// Start with the job template
	if trial.Spec.Template != nil {
		trial.Spec.Template.ObjectMeta.DeepCopyInto(&job.ObjectMeta)
		trial.Spec.Template.Spec.DeepCopyInto(&job.Spec)
	}

	// Provide default metadata
	if job.Name == "" {
		job.Name = trial.Name
	}
	if job.Namespace == "" {
		job.Namespace = trial.Namespace
	}

	// Provide default labels
	if len(job.Labels) == 0 {
		job.Labels = trial.GetDefaultLabels()
	}

	// TODO Also add the "trial" label to the pod template?

	// The default restart policy for a pod is not acceptable in the context of a job
	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	// Containers cannot be empty, inject a sleep by default
	if len(job.Spec.Template.Spec.Containers) == 0 {
		s := trial.Spec.ApproximateRuntime
		if s == nil || s.Duration == 0 {
			s = &metav1.Duration{Duration: 2 * time.Minute}
		}
		if trial.Spec.StartTimeOffset != nil {
			s = &metav1.Duration{Duration: s.Duration + trial.Spec.StartTimeOffset.Duration}
		}
		job.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:    "default-trial-run",
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", fmt.Sprintf("echo 'Sleeping for %s...' && sleep %.0f && echo 'Done.'", s.Duration.String(), s.Seconds())},
			},
		}
	}

	return job
}
