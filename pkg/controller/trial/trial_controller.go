package trial

import (
	"context"
	"fmt"
	"net/url"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	// Evaluate the patch operations
	if len(trial.Status.PatchOperations) == 0 {
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		if err = r.evaluatePatches(trial, e); err != nil {
			return reconcile.Result{}, err
		}
		if len(trial.Status.PatchOperations) > 0 {
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Evaluate the metric queries
	if len(trial.Status.MetricQueries) == 0 {
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}
		if err = r.evaluateMetrics(trial, e); err != nil {
			return reconcile.Result{}, err
		}
		if len(trial.Status.MetricQueries) > 0 {
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Ensure we have non-nil selector
	if trial.Spec.Selector == nil {
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
			return reconcile.Result{}, err
		}

		// TODO Are we doing this labeling stuff right?
		trial.Spec.Selector = metav1.SetAsLabelSelector(map[string]string{"experiment": e.Name})

		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Apply the patches
	for i := range trial.Status.PatchOperations {
		p := &trial.Status.PatchOperations[i]
		if p.Pending {
			u := unstructured.Unstructured{}
			u.SetName(p.TargetRef.Name)
			u.SetNamespace(p.TargetRef.Namespace)
			u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
			if err := r.Patch(context.TODO(), &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
				return reconcile.Result{}, err
			}

			p.Pending = false
			err = r.Update(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Find jobs matching the selector
	// TODO What about the namespace on the job template?
	opts := &client.ListOptions{Namespace: trial.Namespace}
	if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(trial.Spec.Selector); err != nil {
		return reconcile.Result{}, err
	}
	jobs := &batchv1.JobList{}
	if err := r.List(context.TODO(), jobs, client.UseListOptions(opts)); err != nil {
		return reconcile.Result{}, err
	}

	// Update the trial run status using the job status
	if dirty, adjustStartTime := updateStatusFromJobs(jobs.Items, &trial.Status); dirty {
		if adjustStartTime {
			e := &okeanosv1alpha1.Experiment{}
			if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err != nil {
				return reconcile.Result{}, err
			}
			if e.Spec.StartTimeOffset != nil {
				*trial.Status.StartTime = metav1.NewTime(trial.Status.StartTime.Add(e.Spec.StartTimeOffset.Duration))
			}
		}
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// If we are in a failed state there is nothing more for us to do
	if trial.Status.Failed {
		return reconcile.Result{}, nil
	}

	// Create a new job if needed
	if len(jobs.Items) == 0 {
		// Wait for a stable (ish) state
		if err = waitForStableState(r, context.TODO(), trial.Status.PatchOperations); err != nil {
			if serr, ok := err.(*StabilityError); ok {
				if serr.RetryAfter > 0 {
					// We are not ready to being yet, wait the specified timeout and try again
					return reconcile.Result{Requeue: true, RequeueAfter: serr.RetryAfter}, nil
				} else {
					// The cluster is in a bad state, set the failed flag and update
					trial.Status.Failed = true
					err = r.Update(context.TODO(), trial)
					return reconcile.Result{}, err
				}
			}
			return reconcile.Result{}, err
		}

		// Create a new job
		job := &batchv1.Job{}
		r.createJob(trial, job)

		// Update the controller reference so we get updates when the job changes status
		if err = controllerutil.SetControllerReference(trial, job, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Before creating the job, make sure we are going to be able to find it again
		if !opts.LabelSelector.Matches(labels.Set(job.Labels)) {
			return reconcile.Result{}, errors.NewInvalid(trial.GroupVersionKind().GroupKind(), trial.Name, field.ErrorList{
				field.Invalid(field.NewPath("spec", "selector"), trial.Spec.Selector, "selector does not match evaluated job template"),
			})
		}

		err = r.Create(context.TODO(), job)
		return reconcile.Result{}, err
	}

	if trial.Status.CompletionTime != nil {
		// Only allocate for a single metric at a time
		if trial.Spec.Values == nil {
			trial.Spec.Values = make(map[string]float64, 1)
		}

		// Look for metrics that have not been collected yet
		for _, m := range trial.Status.MetricQueries {

			// Collect the first metric query we have not yet captured and return
			if _, ok := trial.Spec.Values[m.Name]; !ok {
				var retryAfter *time.Duration
				if trial.Spec.Values[m.Name], retryAfter, err = captureMetric(&m, trial); retryAfter != nil || err != nil {
					if retryAfter != nil {
						return reconcile.Result{Requeue: true, RequeueAfter: *retryAfter}, nil
					}
					if err != nil {
						return reconcile.Result{}, err
					}
				}

				err = r.Update(context.TODO(), trial)
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTrial) evaluatePatches(trial *okeanosv1alpha1.Trial, e *okeanosv1alpha1.Experiment) error {
	for _, p := range e.Spec.Patches {
		// Evaluate the patch template
		pt, data, err := executePatchTemplate(&p, trial)
		if err != nil {
			return err
		}

		// Find the targets to apply the patch to
		targets, err := r.findPatchTargets(&p, trial)
		if err != nil {
			return err
		}

		// For each target resource, record a copy of the patch
		for _, ref := range targets {
			trial.Status.PatchOperations = append(trial.Status.PatchOperations, okeanosv1alpha1.PatchOperation{
				TargetRef: ref,
				PatchType: pt,
				Data:      data,
				Pending:   true,
			})
		}
	}

	return nil
}

// Finds the patch targets
func (r *ReconcileTrial) findPatchTargets(p *okeanosv1alpha1.PatchTemplate, trial *okeanosv1alpha1.Trial) ([]corev1.ObjectReference, error) {
	if trial.Spec.TargetNamespace == "" {
		trial.Spec.TargetNamespace = "default"
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

func (r *ReconcileTrial) evaluateMetrics(trial *okeanosv1alpha1.Trial, e *okeanosv1alpha1.Experiment) error {
	for _, m := range e.Spec.Metrics {
		var urls []string
		if m.Selector != nil {
			// Find services matching the selector
			ls, err := metav1.LabelSelectorAsSelector(m.Selector)
			if err != nil {
				return err
			}
			services := &corev1.ServiceList{}
			if err := r.List(context.TODO(), services, client.UseListOptions(&client.ListOptions{LabelSelector: ls})); err != nil {
				return err
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
					return err
				}
				urls = append(urls, thisIsBad.String())
			}
		} else {
			// If there is no service selector, just use an empty URL
			urls = append(urls, "")
		}

		// Add a metric query for every URL
		for _, u := range urls {
			trial.Status.MetricQueries = append(trial.Status.MetricQueries, okeanosv1alpha1.MetricQuery{
				Name:       m.Name,
				MetricType: m.Type,
				Query:      m.Query,
				URL:        u,
			})
		}
	}

	return nil
}

// Updates a trial status based on the status of the individual job(s), returns true if any changes were necessary
func updateStatusFromJobs(jobs []batchv1.Job, status *okeanosv1alpha1.TrialStatus) (bool, bool) {
	var dirty bool
	var adjustStartTime bool

	for _, j := range jobs {
		if status.StartTime == nil {
			// Establish a start time if available
			status.StartTime = j.Status.StartTime.DeepCopy()
			dirty = dirty || j.Status.StartTime != nil
			adjustStartTime = true
		} else if j.Status.StartTime != nil && status.StartTime.Before(j.Status.StartTime) {
			// Move the start time back
			status.StartTime = j.Status.StartTime.DeepCopy()
			dirty = true
		}

		if status.CompletionTime == nil {
			// Establish an end time if available
			status.CompletionTime = j.Status.CompletionTime.DeepCopy()
			dirty = dirty || j.Status.CompletionTime != nil
		} else if j.Status.CompletionTime != nil && status.CompletionTime.Before(j.Status.CompletionTime) {
			// Move the completion time back
			status.CompletionTime = j.Status.CompletionTime.DeepCopy()
			dirty = true
		}

		// Mark the trial as failed if there are any failed pods
		if !status.Failed && j.Status.Failed > 0 {
			for _, c := range j.Status.Conditions {
				if c.Type == batchv1.JobFailed {
					// If activeDeadlineSeconds was used a workaround for having a sidecar, ignore the failure
					status.Failed = c.Reason != "DeadlineExceeded"
				}
			}
			dirty = dirty || status.Failed
		}
	}

	return dirty, adjustStartTime
}

func (r *ReconcileTrial) createJob(trial *okeanosv1alpha1.Trial, job *batchv1.Job) {
	// Try to get the job template off the experiment

	e := &okeanosv1alpha1.Experiment{}
	if err := r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err == nil {
		e.Spec.JobTemplate.ObjectMeta.DeepCopyInto(&job.ObjectMeta)
		e.Spec.JobTemplate.Spec.DeepCopyInto(&job.Spec)
	}

	// Provide default metadata
	if job.Name == "" {
		job.Name = trial.Name
	}
	if job.Namespace == "" {
		job.Namespace = trial.Namespace
	}

	// TODO Are we doing the labeling correctly?
	if len(job.Labels) == 0 {
		job.Labels = make(map[string]string, 1)
		if e.Name != "" {
			job.Labels["experiment"] = e.Name
		} else {
			job.Labels["experiment"] = trial.Name
		}
	}

	// The default restart policy for a pod is not acceptable in the context of a job
	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	// Containers cannot be null, inject a sleep by default
	if job.Spec.Template.Spec.Containers == nil {
		s := e.Spec.ApproximateRuntime
		if s == nil || s.Duration == 0 {
			s = &metav1.Duration{Duration: 2 * time.Minute}
		}
		if e.Spec.StartTimeOffset != nil {
			s = &metav1.Duration{Duration: s.Duration + e.Spec.StartTimeOffset.Duration}
		}
		job.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:    "default-trial-run",
				Image:   "busybox",
				Command: []string{"sleep", fmt.Sprintf("%.0f", s.Seconds())},
			},
		}
	}
}
