package experiment

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	okeanosclient "github.com/gramLabs/okeanos/pkg/api"
	okeanosapi "github.com/gramLabs/okeanos/pkg/api/okeanos/v1alpha1"
	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	annotationPrefix = "okeanos.carbonrelay.com/"

	annotationExperimentURL  = annotationPrefix + "experiment-url"
	annotationSuggestionURL  = annotationPrefix + "suggestion-url"
	annotationObservationURL = annotationPrefix + "observation-url"
)

var log = logf.Log.WithName("controller")

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	// Without a remote server address, this controller does not have anything to do
	address := os.Getenv("OKEANOS_BASE_URL")
	if address == "" {
		return nil
	}
	oc, err := okeanosclient.NewClient(okeanosclient.Config{
		Address: address,
	})
	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr, oc))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, oc okeanosclient.Client) reconcile.Reconciler {
	return &ReconcileExperiment{Client: mgr.GetClient(), scheme: mgr.GetScheme(), api: okeanosapi.NewApi(oc)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("experiment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Experiment
	err = c.Watch(&source.Kind{Type: &okeanosv1alpha1.Experiment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to owned Trials
	err = c.Watch(&source.Kind{Type: &okeanosv1alpha1.Trial{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &okeanosv1alpha1.Experiment{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileExperiment{}

// ReconcileExperiment reconciles a Experiment object
type ReconcileExperiment struct {
	client.Client
	scheme *runtime.Scheme
	api    okeanosapi.API
}

// Reconcile reads that state of the cluster for a Experiment object and makes changes based on the state read
// and what is in the Experiment.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=okeanos.carbonrelay.com,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=okeanos.carbonrelay.com,resources=experiments/status,verbs=get;update;patch
func (r *ReconcileExperiment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Experiment instance
	experiment := &okeanosv1alpha1.Experiment{}
	err := r.Get(context.TODO(), request.NamespacedName, experiment)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define the experiment on the server
	experimentURL := experiment.GetAnnotations()[annotationExperimentURL]
	if experimentURL == "" && experiment.GetReplicas() > 0 {
		n := okeanosapi.NewExperimentName(experiment.Name)
		e := okeanosapi.Experiment{}
		copyExperimentToRemote(experiment, &e)
		log.Info("Creating remote experiment", "experimentURL", experimentURL)
		if experimentURL, err = r.api.CreateExperiment(context.TODO(), n, e); err != nil {
			// Error posting the representation - requeue the request.
			return reconcile.Result{}, err
		}

		// Update the experiment URL, will create a new reconcile request
		experiment.GetAnnotations()[annotationExperimentURL] = experimentURL
		err = r.Update(context.TODO(), experiment)
		return reconcile.Result{}, err
	}

	// Update information only populated by server after PUT
	suggestionURL := experiment.GetAnnotations()[annotationSuggestionURL]
	if suggestionURL == "" && experimentURL != "" && experiment.GetReplicas() > 0 {
		e, err := r.api.GetExperiment(context.TODO(), experimentURL)
		if err != nil {
			// Unable to fetch the remote experiment - requeue the request
			return reconcile.Result{}, err
		}

		// Since we have the server representation, enforce a cap on the replica count
		// NOTE: Do the update in memory, we will only persist it if the suggestion URL needs updating
		if experiment.GetReplicas() > int(e.Optimization.Parallelism) && e.Optimization.Parallelism > 0 {
			*experiment.Spec.Replicas = e.Optimization.Parallelism
		}

		// The suggestion reference may be missing because the experiment isn't producing suggestions anymore
		if e.SuggestionRef != "" {
			experiment.GetAnnotations()[annotationSuggestionURL] = e.SuggestionRef
			err := r.Update(context.TODO(), experiment)
			return reconcile.Result{}, err
		}
	}

	// Find trials labeled for this experiment
	opts := &client.ListOptions{}
	if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(experiment.Spec.Selector); err != nil {
		return reconcile.Result{}, err
	}
	list := &okeanosv1alpha1.TrialList{}
	if err := r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
		return reconcile.Result{}, err
	}

	// Add an additional trial if needed
	if suggestionURL != "" && experiment.GetReplicas() > len(list.Items) {
		// Find an available namespace
		namespace, err := r.findAvailableNamespace(experiment, list.Items)
		if err != nil {
			return reconcile.Result{}, err
		}
		if namespace != "" {
			// The initial trial namespace may be overwritten by the template
			trial := &okeanosv1alpha1.Trial{}
			populateTrialFromTemplate(experiment, trial, namespace)
			if err := controllerutil.SetControllerReference(experiment, trial, r.scheme); err != nil {
				return reconcile.Result{}, err
			}

			// Obtain a suggestion from the server
			suggestion, observationURL, err := r.api.NextSuggestion(context.TODO(), suggestionURL)
			if err != nil {
				if aerr, ok := err.(*okeanosapi.Error); ok {
					switch aerr.Type {
					case okeanosapi.ErrExperimentStopped:
						// The experiment is stopped, set replicas to 0 to prevent further interaction with the server
						experiment.SetReplicas(0)
						delete(experiment.GetAnnotations(), annotationSuggestionURL) // HTTP "Gone" semantics require us to purge this
						err = r.Update(context.TODO(), experiment)
						return reconcile.Result{}, err
					case okeanosapi.ErrSuggestionUnavailable:
						// No suggestions available, wait to requeue until after the retry delay
						return reconcile.Result{Requeue: true, RequeueAfter: aerr.RetryAfter}, nil
					}
				}
				return reconcile.Result{}, err
			}

			// Add the information from the server
			trial.Spec.Assignments = suggestion.Assignments
			trial.GetAnnotations()[annotationObservationURL] = observationURL

			// Create the trial
			log.Info("Creating new trial", "name", trial.Name, "namespace", trial.Namespace, "observationURL", observationURL)
			err = r.Create(context.TODO(), trial)
			// TODO If there is an error, notify server that we failed to adopt the suggestion?
			return reconcile.Result{}, err
		}
	}

	// Record finished trials
	// TODO Is this something that should be implemented using a finalizer on the trial? (e.g. just delete the trial here and have a finalizer that does the post?)
	for _, t := range list.Items {
		if len(t.Spec.Values) == len(t.Status.MetricQueries) || t.Status.Failed {
			// Create an observation
			observation := okeanosapi.Observation{
				Failed: t.Status.Failed,
				Values: make([]okeanosapi.Value, len(t.Spec.Values)),
			}
			for k, v := range t.Spec.Values {
				observation.Values = append(observation.Values, okeanosapi.Value{
					Name:  k,
					Value: v,
					// TODO Error is the standard deviation for the metric
				})
			}

			// Send the observation to the server
			log.Info("Reporting trial observation", "name", t.Name, "namespace", t.Namespace, "values", observation.Values)
			err = r.api.ReportObservation(context.TODO(), t.GetAnnotations()[annotationObservationURL], observation)
			if err != nil {
				// The observation was not accepted, requeue the request
				return reconcile.Result{}, err
			}

			// Only delete the trial once it has been sent to the server
			err = r.Delete(context.TODO(), &t)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// Copy the custom resource state into a client API representation
func copyExperimentToRemote(experiment *okeanosv1alpha1.Experiment, e *okeanosapi.Experiment) {
	e.Optimization = experiment.Spec.Optimization

	e.Parameters = nil
	for _, p := range experiment.Spec.Parameters {
		cp := okeanosapi.Parameter{Name: p.Name}
		// TODO Default value?
		if len(p.Values) == 0 {
			cp.Bounds = okeanosapi.Bounds{
				Min: json.Number(strconv.Itoa(p.Min)),
				Max: json.Number(strconv.Itoa(p.Max)),
			}
		} else {
			cp.Values = p.Values
		}
		e.Parameters = append(e.Parameters, cp)
	}

	e.Metrics = nil
	for _, m := range experiment.Spec.Metrics {
		e.Metrics = append(e.Metrics, okeanosapi.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
		})
	}
}

// Creates a new trial for an experiment
func populateTrialFromTemplate(experiment *okeanosv1alpha1.Experiment, trial *okeanosv1alpha1.Trial, namespace string) {
	experiment.Spec.Template.ObjectMeta.DeepCopyInto(&trial.ObjectMeta)
	experiment.Spec.Template.Spec.DeepCopyInto(&trial.Spec)

	// Overwrite the target namespace unless we are only running a single trial on the cluster
	if experiment.GetReplicas() > 1 || experiment.Spec.NamespaceSelector != nil || experiment.Spec.Template.Namespace != "" {
		trial.Spec.TargetNamespace = namespace
	}

	// Provide a default for the namespace
	if trial.Namespace == "" {
		trial.Namespace = namespace
	}

	// Provide a default name for the trial
	if trial.Name == "" {
		if trial.Namespace != experiment.Namespace {
			trial.Name = experiment.Name
		} else {
			trial.GenerateName = experiment.Name + "-"
		}
	}

	// Provide a default reference back to the experiment
	if trial.Spec.ExperimentRef == nil {
		trial.Spec.ExperimentRef = experiment.GetSelfReference()
	}

	// TODO Handle labeling, is this correct?
	trial.Labels["experiment"] = experiment.Name
	if trial.Spec.Selector == nil {
		trial.Spec.Selector = metav1.SetAsLabelSelector(trial.Labels)
	}
}

// Searches for a namespace to run a new trial in, returning an empty string if no such namespace can be found
func (r *ReconcileExperiment) findAvailableNamespace(experiment *okeanosv1alpha1.Experiment, trials []okeanosv1alpha1.Trial) (string, error) {
	// Determine which namespaces are already in use
	inuse := make(map[string]bool, len(trials))
	for _, t := range trials {
		inuse[t.Namespace] = true
	}

	// Find eligible namespaces
	if experiment.Spec.NamespaceSelector != nil {
		ls, err := metav1.LabelSelectorAsSelector(experiment.Spec.NamespaceSelector)
		if err != nil {
			return "", err
		}
		list := &corev1.NamespaceList{}
		if err := r.List(context.TODO(), list, client.UseListOptions(&client.ListOptions{LabelSelector: ls})); err != nil {
			return "", err
		}

		// Find the first available namespace
		for _, item := range list.Items {
			if !inuse[item.Name] {
				return item.Name, nil
			}
		}
		return "", nil
	}

	// Check if the experiment namespace is available
	if inuse[experiment.Namespace] {
		return "", nil
	}
	return experiment.Namespace, nil
}
