package trial

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

// Add creates a new Trial Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTrial{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("trial-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Trial
	err = c.Watch(&source.Kind{Type: &redskyv1alpha1.Trial{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to owned Jobs
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &redskyv1alpha1.Trial{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileTrial{}

// ReconcileTrial reconciles a Trial object
type ReconcileTrial struct {
	client.Client
	scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch;extensions,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch;extensions,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments;statefulsets,verbs=get;list;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=redsky.carbonrelay.com,resources=trials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redsky.carbonrelay.com,resources=trials/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a Trial object and makes changes based on the state read
// and what is in the Trial.Spec
func (r *ReconcileTrial) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Trial instance
	trial := &redskyv1alpha1.Trial{}
	err := r.Get(context.TODO(), request.NamespacedName, trial)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Ahead of everything is the setup/teardown (contains finalization logic)
	if resp, ret, err := manageSetup(r.Client, r.scheme, trial); err != nil || resp.Requeue || ret {
		return resp, err
	}

	// If we are in a finished or deleted state there is nothing for us to do
	if IsTrialFinished(trial) || trial.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// Update the display status
	syncStatus(trial)
	now := metav1.Now()

	// One time evaluation of the patch operations
	if len(trial.Spec.PatchOperations) == 0 {
		e := &redskyv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		if dirty, err := evaluatePatches(r, trial, e); err != nil {
			return reconcile.Result{}, err
		} else if dirty {
			// We know we have at least one patch to apply, use an unknown status until we start applying them
			applyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionUnknown, "", "", &now)
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Apply the patches
	for i := range trial.Spec.PatchOperations {
		p := &trial.Spec.PatchOperations[i]
		if p.AttemptsRemaining > 0 {
			u := unstructured.Unstructured{}
			u.SetName(p.TargetRef.Name)
			u.SetNamespace(p.TargetRef.Namespace)
			u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
			if err := r.Patch(context.TODO(), &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
				p.AttemptsRemaining = p.AttemptsRemaining - 1
				if p.AttemptsRemaining == 0 {
					// There are no remaining patch attempts remaining, fail the trial
					applyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "PatchFailed", err.Error(), &now)
				}
			} else {
				p.AttemptsRemaining = 0
				if p.Wait {
					// We successfully applied a patch that requires a wait, use an unknown status until we start waiting
					applyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionUnknown, "", "", &now)
				}
			}

			// We have started applying patches (success or fail), transition into a false status
			applyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionFalse, "", "", &now)
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// If there is a patched condition that is not yet true, update the status
	if cc, ok := checkCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue); ok && !cc {
		applyCondition(&trial.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue, "", "", &now)
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Wait for a stable (ish) state
	for i := range trial.Spec.PatchOperations {
		p := &trial.Spec.PatchOperations[i]
		if p.Wait {
			result := reconcile.Result{}
			if err = waitForStableState(r, context.TODO(), p); err != nil {
				if serr, ok := err.(*StabilityError); ok && serr.RetryAfter > 0 {
					// Mark the trial as not stable and wait
					applyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "Wait", serr.Error(), &now)
					result.RequeueAfter = serr.RetryAfter
					err = nil
				} else {
					// No retry delay specified, fail the whole trial
					applyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "WaitFailed", serr.Error(), &now)
				}
			} else {
				// We have successfully waited for one patch so we are no longer "unknown"
				applyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionFalse, "", "", &now)
				p.Wait = false
			}

			// TODO We have potentially two errors to report here
			if err := r.Update(context.TODO(), trial); err != nil {
				return reconcile.Result{}, err
			}
			return result, err
		}
	}

	// If there is a stable condition that is not yet true, update the status
	if cc, ok := checkCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue); ok && !cc {
		applyCondition(&trial.Status, redskyv1alpha1.TrialStable, corev1.ConditionTrue, "", "", &now)
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Find jobs labeled for this trial
	list := &batchv1.JobList{}
	opts := &client.ListOptions{}
	if trial.Spec.Selector == nil {
		if trial.Spec.Template != nil {
			opts.MatchingLabels(trial.Spec.Template.Labels)
		}
		if opts.LabelSelector == nil || opts.LabelSelector.Empty() {
			opts.MatchingLabels(trial.GetDefaultLabels())
		}
	} else if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(trial.Spec.Selector); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
		return reconcile.Result{}, err
	}
	// TODO We may need to hard filter out setup jobs to ensure there is not a labelling misconfiguration

	// Update the trial run status using the job status
	if updateStatusFromJobs(list.Items, trial, &now) {
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Create a new job if needed
	if len(list.Items) == 0 {
		job := &batchv1.Job{}
		if err = r.createJob(trial, job); err != nil {
			return reconcile.Result{}, err
		}
		err = r.Create(context.TODO(), job)
		return reconcile.Result{}, err
	}

	if trial.Status.CompletionTime != nil {
		// Look for metrics that have not been collected yet
		e := &redskyv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		for _, m := range e.Spec.Metrics {
			v := findOrCreateValue(trial, m.Name)
			if v.AttemptsRemaining == 0 {
				continue
			}

			urls, verr := r.findMetricTargets(trial, &m)
			for _, u := range urls {
				if value, stddev, retryAfter, err := captureMetric(&m, u, trial); err != nil {
					verr = err
				} else if retryAfter != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: *retryAfter}, nil
				} else if math.IsNaN(value) || math.IsNaN(stddev) {
					verr = fmt.Errorf("capturing metric %s got NaN", m.Name)
				} else {
					v.AttemptsRemaining = 0
					v.Value = strconv.FormatFloat(value, 'f', -1, 64)
					if stddev != 0 {
						v.Error = strconv.FormatFloat(stddev, 'f', -1, 64)
					}
					syncStatus(trial)
					break
				}
			}

			if verr != nil && v.AttemptsRemaining > 0 {
				v.AttemptsRemaining = v.AttemptsRemaining - 1
				if v.AttemptsRemaining == 0 {
					applyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "MetricFailed", verr.Error(), &now)
				}
			}

			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}

		// If all of the metrics are collected, mark the trial as completed
		applyCondition(&trial.Status, redskyv1alpha1.TrialComplete, corev1.ConditionTrue, "", "", &now)
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// This fall through case may occur while getting the job started
	return reconcile.Result{}, nil
}

func syncStatus(trial *redskyv1alpha1.Trial) {
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
}

func evaluatePatches(r client.Reader, trial *redskyv1alpha1.Trial, e *redskyv1alpha1.Experiment) (bool, error) {
	if dirty, err := checkAssignments(trial, e); dirty || err != nil {
		return dirty, err
	}

	var dirty bool
	for _, p := range e.Spec.Patches {
		// Evaluate the patch template
		pt, data, err := executePatchTemplate(&p, trial)
		if err != nil {
			return false, err
		}

		// Find the targets to apply the patch to
		targets, err := findPatchTargets(r, &p, trial)
		if err != nil {
			return false, err
		}

		// TODO This is a hack to allow stability checks on arbitrary objects by omitting the patch data
		attempts := 3
		if len(data) == 0 {
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
			dirty = true
		}
	}

	return dirty, nil
}

// Finds the patch targets
func findPatchTargets(r client.Reader, p *redskyv1alpha1.PatchTemplate, trial *redskyv1alpha1.Trial) ([]corev1.ObjectReference, error) {
	if trial.Spec.TargetNamespace == "" {
		trial.Spec.TargetNamespace = trial.Namespace
	}

	var targets []corev1.ObjectReference
	if p.TargetRef.Name == "" {
		ls, err := metav1.LabelSelectorAsSelector(p.Selector)
		if err != nil {
			return nil, err
		}
		opts := &client.ListOptions{LabelSelector: ls}
		if p.TargetRef.Namespace != "" {
			opts.Namespace = p.TargetRef.Namespace
		} else {
			opts.Namespace = trial.Spec.TargetNamespace
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
		if err := r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
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

func checkAssignments(trial *redskyv1alpha1.Trial, experiment *redskyv1alpha1.Experiment) (bool, error) {
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

	if len(missing) > 0 {
		return false, fmt.Errorf("trial %s is missing assignments for %s", trial.Name, missing)
	}
	return false, nil
}

func (r *ReconcileTrial) findMetricTargets(trial *redskyv1alpha1.Trial, m *redskyv1alpha1.Metric) ([]string, error) {
	// Local metrics don't need to resolve service URLs
	if m.Type == redskyv1alpha1.MetricLocal || m.Type == "" {
		return []string{""}, nil
	}

	// Find services matching the selector
	list := &corev1.ServiceList{}
	opts := &client.ListOptions{}
	if ls, err := metav1.LabelSelectorAsSelector(m.Selector); err == nil {
		opts.LabelSelector = ls
	} else {
		return nil, err
	}
	if err := r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
		return nil, err
	}

	// Construct a URL for each service (use IP literals instead of host names to avoid DNS lookups)
	var urls []string
	for _, s := range list.Items {
		port := m.Port.IntValue()
		if port < 1 {
			for _, sp := range s.Spec.Ports {
				if m.Port.StrVal == sp.Name || len(s.Spec.Ports) == 1 {
					port = int(sp.Port)
				}
			}
		}

		// TODO TLS support
		// TODO Port < 1
		// TODO Build this URL properly
		thisIsBad, err := url.Parse(fmt.Sprintf("http://%s:%d%s", s.Spec.ClusterIP, port, m.Path))
		if err != nil {
			return nil, err
		}
		urls = append(urls, thisIsBad.String())
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("unable to find metric targets for %s", m.Name)
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

// Updates a trial status based on the status of the individual job(s), returns true if any changes were necessary
func updateStatusFromJobs(jobs []batchv1.Job, trial *redskyv1alpha1.Trial, time *metav1.Time) bool {
	var dirty bool

	for _, j := range jobs {

		if trial.Status.StartTime == nil {
			// Establish a start time if available
			trial.Status.StartTime = j.Status.StartTime.DeepCopy()
			dirty = dirty || j.Status.StartTime != nil
			if dirty && trial.Spec.StartTimeOffset != nil {
				*trial.Status.StartTime = metav1.NewTime(trial.Status.StartTime.Add(trial.Spec.StartTimeOffset.Duration))
			}
		} else if j.Status.StartTime != nil && trial.Status.StartTime.Before(j.Status.StartTime) {
			// Move the start time back
			trial.Status.StartTime = j.Status.StartTime.DeepCopy()
			dirty = true
		}

		if trial.Status.CompletionTime == nil {
			// Establish an end time if available
			trial.Status.CompletionTime = j.Status.CompletionTime.DeepCopy()
			dirty = dirty || j.Status.CompletionTime != nil
		} else if j.Status.CompletionTime != nil && trial.Status.CompletionTime.Before(j.Status.CompletionTime) {
			// Move the completion time back
			trial.Status.CompletionTime = j.Status.CompletionTime.DeepCopy()
			dirty = true
		}

		// Mark the trial as failed if the job itself failed
		for _, c := range j.Status.Conditions {
			// If activeDeadlineSeconds was used a workaround for having a sidecar, ignore the failure
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue && c.Reason != "DeadlineExceeded" {
				applyCondition(&trial.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, c.Reason, c.Message, time)
				dirty = true
			}
		}
	}

	return dirty
}

func (r *ReconcileTrial) createJob(trial *redskyv1alpha1.Trial, job *batchv1.Job) error {
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

	// The default restart policy for a pod is not acceptable in the context of a job
	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	// Containers cannot be null, inject a sleep by default
	if job.Spec.Template.Spec.Containers == nil {
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
				Command: []string{"sleep", fmt.Sprintf("%.0f", s.Seconds())},
			},
		}
	}

	// Set the owner reference back to the trial
	err := controllerutil.SetControllerReference(trial, job, r.scheme)
	return err
}
