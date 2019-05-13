package trial

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"text/template"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	appsv1 "k8s.io/api/apps/v1"
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

	// Pop a patch off the head of the list and apply it
	if len(trial.Spec.Patches) > 0 {
		p := trial.Spec.Patches[0]
		u := unstructured.Unstructured{}
		u.SetName(p.Reference.Name)
		u.SetNamespace(p.Reference.Namespace)
		u.SetGroupVersionKind(p.Reference.GroupVersionKind())
		if err := r.Patch(context.TODO(), &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
			return reconcile.Result{}, err
		}

		trial.Spec.Patches = trial.Spec.Patches[1:]
		trial.Status.Patched = append(trial.Status.Patched, p.Reference)
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Wait for a stable (ish) state
	if trial.Status.End == nil && !trial.Spec.Failed {
		for _, p := range trial.Status.Patched {
			switch p.Kind {
			case "StatefulSet":
				ss := &appsv1.StatefulSet{}
				if err := r.Get(context.TODO(), client.ObjectKey{Namespace: p.Namespace, Name: p.Name}, ss); err == nil {
					// TODO We also need to check for errors, if there are failures we never launch the job
					if ss.Status.ReadyReplicas < ss.Status.Replicas {
						return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
					}
				}
			}
		}
	}

	// Find jobs matching the selector
	opts := &client.ListOptions{Namespace: trial.Namespace}
	if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(trial.Spec.Selector); err != nil {
		return reconcile.Result{}, err
	}
	jobs := &batchv1.JobList{}
	if err := r.List(context.TODO(), jobs, client.UseListOptions(opts)); err != nil {
		return reconcile.Result{}, err
	}

	// Update the trial run status using the job status
	var dirty bool
	for _, job := range jobs.Items {
		// TODO Do we need to filter on the deletion timestamp?
		dirty = trial.Status.MergeFromJob(&job.Status)
		// TODO What about failure state? All pods? Any pods?
	}
	if dirty {
		err = r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	// Create a new job if needed (start may be nil if we created a job but it hasn't started yet)
	if len(jobs.Items) == 0 {
		if trial.Spec.Template == nil {
			return reconcile.Result{}, errors.NewInvalid(trial.GroupVersionKind().GroupKind(), trial.Name, field.ErrorList{
				field.Required(field.NewPath("spec", "template"), "missing required template"),
			})
		}

		job := &batchv1.Job{}
		job.ObjectMeta = *trial.Spec.Template.ObjectMeta.DeepCopy()
		job.Spec = *trial.Spec.Template.Spec.DeepCopy()

		job.Name = trial.Name + "-job"
		if job.Namespace == "" {
			job.Namespace = trial.Namespace
		}

		// The default restart policy for a pod is not acceptable in the context of a job
		if job.Spec.Template.Spec.RestartPolicy == "" {
			job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		}

		if err := controllerutil.SetControllerReference(trial, job, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// TODO What needs to be overwritten?
		// TODO Apply placeholders? Do we need to serialize to JSON, apply Go template, then deserialize?

		// Before creating the job, make sure we are going to be able to find it again
		if !opts.LabelSelector.Matches(labels.Set(job.Labels)) {
			return reconcile.Result{}, errors.NewInvalid(trial.GroupVersionKind().GroupKind(), trial.Name, field.ErrorList{
				field.Invalid(field.NewPath("spec", "selector"), trial.Spec.Selector, "selector does not match evaluated job template"),
			})
		}
		err = r.Create(context.TODO(), job)
		return reconcile.Result{}, err
	}

	if trial.Status.End != nil {
		// Stop querying Prometheus if we know it's ready
		var promReady bool

		trial.Spec.Metrics = make(map[string]string, len(trial.Spec.Queries))
		for _, m := range trial.Spec.Queries {
			var value string // TODO Why string?
			switch m.Type {
			case "local", "":
				// Evaluate the query as template against the trial itself
				tmpl, err := template.New("query").Parse(m.Query)
				if err != nil {
					return reconcile.Result{}, err
				}
				buf := new(bytes.Buffer)
				if err = tmpl.Execute(buf, trial); err != nil { // TODO DeepCopy the trial?
					return reconcile.Result{}, err
				}

				// TODO Is this right?
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
				if !promReady {
					var requeueDelay time.Duration
					if promReady, requeueDelay, err = isPrometheusReady(promAPI, trial.Status.End); err != nil {
						return reconcile.Result{}, err
					}
					if !promReady {
						return reconcile.Result{Requeue: true, RequeueAfter: requeueDelay}, err
					}
				}

				// Execute query
				v, err := promAPI.Query(context.TODO(), m.Query, trial.Status.End.Time)
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
			trial.Spec.Metrics[m.Name] = value
		}

		err := r.Update(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
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
