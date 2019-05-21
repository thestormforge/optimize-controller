package trial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"text/template"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/jsonpath"
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

// TODO We need some type of client util to encapsulate this
var httpClient = &http.Client{Timeout: 10 * time.Second}

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

type patchContext struct {
	Values map[string]interface{}
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
	if updateStatusFromJobs(jobs.Items, &trial.Status) {
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// If we are in a failed state there is nothing more for us to do
	if trial.Status.Failed {
		return reconcile.Result{}, nil
	}

	// Create a new job if needed (start may be nil if we created a job but it hasn't started yet)
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

		// Try to get the job template off the experiment
		job := &batchv1.Job{}
		e := &okeanosv1alpha1.Experiment{}
		if err = r.Get(context.TODO(), trial.ExperimentNamespacedName(), e); err == nil {
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

		// Containers cannot be null, inject a small sleep by default
		if job.Spec.Template.Spec.Containers == nil {
			job.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:    "default-trial-run",
					Image:   "busybox",
					Command: []string{"sleep", "120"},
				},
			}
		}

		// Update the controller reference so we get updates when the job changes status
		if err := controllerutil.SetControllerReference(trial, job, r.scheme); err != nil {
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

		for _, m := range trial.Status.MetricQueries {
			if _, ok := trial.Spec.Values[m.Name]; !ok {
				var value string
				switch m.MetricType {
				// TODO Add support for regex extraction over a resource

				case "local", "":
					// Evaluate the query as template against the trial itself
					tmpl, err := template.New("query").Funcs(templateFunctions()).Parse(m.Query)
					if err != nil {
						return reconcile.Result{}, err
					}
					buf := new(bytes.Buffer)
					if err = tmpl.Execute(buf, trial); err != nil { // TODO DeepCopy the trial?
						return reconcile.Result{}, err
					}
					value = buf.String()

				case "prometheus":
					// Get the Prometheus client based on the metric URL
					// TODO Cache these by URL
					c, err := prom.NewClient(prom.Config{Address: m.URL})
					if err != nil {
						return reconcile.Result{}, err
					}
					promAPI := promv1.NewAPI(c)

					// Make sure Prometheus is ready
					var promReady bool
					var requeueDelay time.Duration
					if promReady, requeueDelay, err = isPrometheusReady(promAPI, trial.Status.CompletionTime); err != nil {
						return reconcile.Result{}, err
					}
					if !promReady {
						return reconcile.Result{Requeue: true, RequeueAfter: requeueDelay}, err
					}

					// Execute query
					v, err := promAPI.Query(context.TODO(), m.Query, trial.Status.CompletionTime.Time)
					if err != nil {
						return reconcile.Result{}, err
					}

					// TODO No idea what we are looking at here...
					value = v.String()

				case "jsonpath":
					// Fetch the JSON, evaluate the JSON path
					data := make(map[string]interface{})
					if err := fetchJSON(m.URL, data); err != nil {
						return reconcile.Result{}, err
					}

					jp := jsonpath.New(m.Name)
					if err := jp.Parse(m.Query); err != nil {
						return reconcile.Result{}, err
					}
					values, err := jp.FindResults(data)
					if err != nil {
						return reconcile.Result{}, err
					}

					// TODO No idea what we are looking for here...
					for _, v := range values {
						for _, vv := range v {
							value = vv.String()
						}
					}
				}

				trial.Spec.Values[m.Name], err = strconv.ParseFloat(value, 64)
				if err != nil {
					return reconcile.Result{}, err
				}

				err := r.Update(context.TODO(), trial)
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTrial) evaluatePatches(trial *okeanosv1alpha1.Trial, e *okeanosv1alpha1.Experiment) error {
	for _, p := range e.Spec.Patches {
		// Determine the patch type
		pt, err := p.GetPatchType()
		if err != nil {
			return err
		}

		// Evaluate the template into a patch
		tmpl, err := template.New("patch").Funcs(templateFunctions()).Parse(p.Patch)
		if err != nil {
			return err
		}
		data := patchContext{Values: trial.Spec.Assignments}
		buf := new(bytes.Buffer)
		if err = tmpl.Execute(buf, data); err != nil {
			return err
		}

		// Find the targets to apply the patch to
		targets, err := r.findPatchTargets(trial, p)
		if err != nil {
			return err
		}

		// For each target resource, record a copy of the patch
		for _, ref := range targets {
			trial.Status.PatchOperations = append(trial.Status.PatchOperations, okeanosv1alpha1.PatchOperation{
				TargetRef: ref,
				PatchType: pt,
				Data:      buf.Bytes(),
				Pending:   true,
			})
		}
	}

	return nil
}

// Finds the patch targets
func (r *ReconcileTrial) findPatchTargets(trial *okeanosv1alpha1.Trial, p okeanosv1alpha1.PatchTemplate) ([]corev1.ObjectReference, error) {
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
func updateStatusFromJobs(jobs []batchv1.Job, status *okeanosv1alpha1.TrialStatus) bool {
	var dirty bool

	for _, j := range jobs {
		if status.StartTime == nil {
			// Establish a start time if available
			status.StartTime = j.Status.StartTime // TODO DeepCopy?
			dirty = dirty || j.Status.StartTime != nil
		} else if j.Status.StartTime != nil && j.Status.StartTime.Before(status.StartTime) {
			// Move the start time up
			status.StartTime = j.Status.StartTime
			dirty = true
		}

		if status.CompletionTime == nil {
			// Establish an end time if available
			status.CompletionTime = j.Status.CompletionTime
			dirty = dirty || j.Status.CompletionTime != nil
		} else if j.Status.CompletionTime != nil && status.CompletionTime.Before(j.Status.CompletionTime) {
			// Move the start time back
			status.CompletionTime = j.Status.CompletionTime
			dirty = true
		}

		// Mark the trial as failed if there are any failed pods
		if !status.Failed && j.Status.Failed > 0 {
			status.Failed = true
			dirty = true
		}
	}

	return dirty
}

func fetchJSON(url string, target interface{}) error {
	// TODO Set accept header
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

// For a Prometheus, checks that the last scrape time is after the end time
func isPrometheusReady(promAPI promv1.API, endTime *metav1.Time) (bool, time.Duration, error) {
	ts, err := promAPI.Targets(context.TODO())
	if err != nil {
		return false, 0, err
	}
	for _, t := range ts.Active {
		if t.LastScrape.Before(endTime.Time) {
			// TODO Can we make a more informed delay?
			return false, 5 * time.Second, nil
		}
	}

	return true, 0, nil
}

func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"duration": templateDuration,
	}
}

func templateDuration(start, completion metav1.Time) float64 {
	return completion.Sub(start.Time).Seconds()
}
