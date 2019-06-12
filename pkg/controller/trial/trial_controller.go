package trial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
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
	err = c.Watch(&source.Kind{Type: &okeanosv1alpha1.Trial{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to owned Jobs
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &okeanosv1alpha1.Trial{},
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

// Reconcile reads that state of the cluster for a Trial object and makes changes based on the state read
// and what is in the Trial.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=okeanos.carbonrelay.com,resources=trials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=okeanos.carbonrelay.com,resources=trials/status,verbs=get;update;patch
func (r *ReconcileTrial) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Trial instance
	trial := &okeanosv1alpha1.Trial{}
	err := r.Get(context.TODO(), request.NamespacedName, trial)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Ahead of everything is the setup/teardown
	// TODO This was meant to be in a webhook, but for various reasons we aren't doing that, yet...
	if resp, ret, err := manageSetup(r.Client, r.scheme, trial); resp.Requeue || ret || err != nil {
		return resp, err
	}

	// If we are in a finished or deleted state there is nothing more for us to do with this trial
	if IsTrialFinished(trial) || trial.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// Update the display status
	if dirty := syncStatus(trial); dirty {
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Evaluate the patch operations
	if len(trial.Status.PatchOperations) == 0 {
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		if err = evaluatePatches(r, trial, e); err != nil {
			return reconcile.Result{}, err
		}
		if len(trial.Status.PatchOperations) > 0 {
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Apply the patches
	for i := range trial.Status.PatchOperations {
		p := &trial.Status.PatchOperations[i]
		if p.AttemptsRemaining > 0 {
			u := unstructured.Unstructured{}
			u.SetName(p.TargetRef.Name)
			u.SetNamespace(p.TargetRef.Namespace)
			u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
			if err := r.Patch(context.TODO(), &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
				p.AttemptsRemaining = p.AttemptsRemaining - 1
				if p.AttemptsRemaining > 0 {
					err = r.Update(context.TODO(), trial)
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, err
			}

			p.AttemptsRemaining = 0
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
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

	// Update the trial run status using the job status
	if dirty := updateStatusFromJobs(list.Items, trial); dirty {
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Create a new job if needed
	if len(list.Items) == 0 {
		// Wait for a stable (ish) state
		if err = waitForStableState(r, context.TODO(), trial.Status.PatchOperations); err != nil {
			if serr, ok := err.(*StabilityError); ok {
				if serr.RetryAfter > 0 {
					// We are not ready to create a job yet, wait the specified timeout and try again
					return reconcile.Result{Requeue: true, RequeueAfter: serr.RetryAfter}, nil
				} else {
					// The cluster is in a bad state, fail the experiment
					failureReason := "WaitFailed"
					trial.Status.Conditions = append(trial.Status.Conditions, newCondition(okeanosv1alpha1.TrialFailed, failureReason, serr.Error()))
					err = r.Update(context.TODO(), trial)
					return reconcile.Result{}, err
				}
			}
			return reconcile.Result{}, err
		}

		// Create a new job
		job := &batchv1.Job{}
		if err = r.createJob(trial, job); err != nil {
			return reconcile.Result{}, err
		}
		err = r.Create(context.TODO(), job)
		return reconcile.Result{}, err
	}

	if trial.Status.CompletionTime != nil {
		// Look for metrics that have not been collected yet
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		for _, m := range e.Spec.Metrics {
			if _, ok := trial.Spec.Values[m.Name]; !ok {
				var urls []string
				if urls, err = r.findMetricTargets(trial, &m); err == nil {
					for _, u := range urls {
						if value, retryAfter, err := captureMetric(&m, u, trial); retryAfter != nil {
							// The metric could not be captured at this time, wait and try again
							return reconcile.Result{Requeue: true, RequeueAfter: *retryAfter}, nil
						} else if err == nil {
							trial.Spec.Values[m.Name] = strconv.FormatFloat(value, 'f', -1, 64)
							err = r.Update(context.TODO(), trial)
							return reconcile.Result{}, err
						}
					}
				}

				// Failure either from either findMetricTargets or captureMetric
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}

		// If all of the metrics are collected, mark the trial as completed
		log.Info("Completing trial", "namespace", trial.Namespace, "name", trial.Name)
		trial.Status.Conditions = append(trial.Status.Conditions, newCondition(okeanosv1alpha1.TrialComplete, "", ""))
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// This fall through case may occur while getting the job started
	return reconcile.Result{}, nil
}

func IsTrialFinished(trial *okeanosv1alpha1.Trial) bool {
	for _, c := range trial.Status.Conditions {
		if (c.Type == okeanosv1alpha1.TrialComplete || c.Type == okeanosv1alpha1.TrialFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func syncStatus(trial *okeanosv1alpha1.Trial) bool {
	var dirty bool

	// This isn't really status, but having it nil looks ugly in the output
	if trial.Spec.Values == nil {
		trial.Spec.Values = make(map[string]string, 1)
		dirty = true
	}

	// TODO Instead of JSON should we just generate "a=b, c=d, ..." for readability

	if b, err := json.Marshal(trial.Spec.Assignments); err == nil {
		s := string(b)
		dirty = dirty || trial.Status.Assignments != s
		trial.Status.Assignments = s
	}

	if b, err := json.Marshal(trial.Spec.Values); err == nil {
		s := string(b)
		dirty = dirty || trial.Status.Values != s
		trial.Status.Values = s
	}

	return dirty
}

func newCondition(conditionType okeanosv1alpha1.TrialConditionType, reason, message string) okeanosv1alpha1.TrialCondition {
	return okeanosv1alpha1.TrialCondition{
		Type:               conditionType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func evaluatePatches(r client.Reader, trial *okeanosv1alpha1.Trial, e *okeanosv1alpha1.Experiment) error {
	for _, p := range e.Spec.Patches {
		// Evaluate the patch template
		pt, data, err := executePatchTemplate(&p, trial)
		if err != nil {
			return err
		}

		// Find the targets to apply the patch to
		targets, err := findPatchTargets(r, &p, trial)
		if err != nil {
			return err
		}

		// For each target resource, record a copy of the patch
		for _, ref := range targets {
			trial.Status.PatchOperations = append(trial.Status.PatchOperations, okeanosv1alpha1.PatchOperation{
				TargetRef:         ref,
				PatchType:         pt,
				Data:              data,
				AttemptsRemaining: 3,
			})
		}
	}

	return nil
}

// Finds the patch targets
func findPatchTargets(r client.Reader, p *okeanosv1alpha1.PatchTemplate, trial *okeanosv1alpha1.Trial) ([]corev1.ObjectReference, error) {
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

func (r *ReconcileTrial) findMetricTargets(trial *okeanosv1alpha1.Trial, m *okeanosv1alpha1.Metric) ([]string, error) {
	var urls []string
	if m.Selector != nil {
		// Find services matching the selector
		ls, err := metav1.LabelSelectorAsSelector(m.Selector)
		if err != nil {
			return nil, err
		}
		services := &corev1.ServiceList{}
		if err := r.List(context.TODO(), services, client.UseListOptions(&client.ListOptions{LabelSelector: ls})); err != nil {
			return nil, err
		}
		for _, s := range services.Items {
			port := m.Port.IntValue()
			if port < 1 {
				for _, sp := range s.Spec.Ports {
					if m.Port.StrVal == sp.Name {
						port = int(sp.Port)
					}
				}
			}

			// TODO Build this URL properly
			thisIsBad, err := url.Parse(fmt.Sprintf("http://%s:%d%s", s.Spec.ClusterIP, port, m.Path))
			if err != nil {
				return nil, err
			}
			urls = append(urls, thisIsBad.String())
		}
	} else {
		// If there is no service selector, just use an empty URL
		urls = append(urls, "")
	}

	return urls, nil
}

// Updates a trial status based on the status of the individual job(s), returns true if any changes were necessary
func updateStatusFromJobs(jobs []batchv1.Job, trial *okeanosv1alpha1.Trial) bool {
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
				trial.Status.Conditions = append(trial.Status.Conditions, newCondition(okeanosv1alpha1.TrialFailed, c.Reason, c.Message))
				dirty = true
			}
		}
	}

	return dirty
}

func (r *ReconcileTrial) createJob(trial *okeanosv1alpha1.Trial, job *batchv1.Job) error {
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
