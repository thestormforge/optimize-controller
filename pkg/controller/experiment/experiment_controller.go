package experiment

import (
	"context"
	"encoding/json"
	"strconv"

	okeanosclient "github.com/gramLabs/okeanos/pkg/api"
	okeanosapi "github.com/gramLabs/okeanos/pkg/api/okeanos/v1alpha1"
	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	okeanostrial "github.com/gramLabs/okeanos/pkg/controller/trial"
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

	finalizer = "finalizer.okeanos.carbonrelay.com"
)

var log = logf.Log.WithName("controller")

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	// We need a remote address to do anything in this controller
	config, err := okeanosclient.DefaultConfig()
	if err != nil || config.Address == "" {
		return err
	}
	oc, err := okeanosclient.NewClient(*config)
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

	// Make sure we aren't deleted without a chance to clean up
	if dirty := addFinalizer(experiment); dirty {
		err := r.Update(context.TODO(), experiment)
		return reconcile.Result{}, err
	}

	// Synchronize with the server
	if dirty, err := r.syncWithServer(experiment); err != nil {
		return reconcile.Result{}, err
	} else if dirty {
		err = r.Update(context.TODO(), experiment)
		return reconcile.Result{}, err
	}

	// Find trials labeled for this experiment
	list := &okeanosv1alpha1.TrialList{}
	opts := &client.ListOptions{}
	if experiment.Spec.Selector == nil {
		opts.MatchingLabels(experiment.Spec.Template.Labels)
		if opts.LabelSelector.Empty() {
			opts.MatchingLabels(experiment.GetDefaultLabels())
		}
	} else if opts.LabelSelector, err = metav1.LabelSelectorAsSelector(experiment.Spec.Selector); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.List(context.TODO(), list, client.UseListOptions(opts)); err != nil {
		return reconcile.Result{}, err
	}

	// Add an additional trial if needed
	suggestionURL := experiment.GetAnnotations()[annotationSuggestionURL]
	if suggestionURL != "" && experiment.GetReplicas() > len(list.Items) {
		// Find an available namespace
		if namespace, err := r.findAvailableNamespace(experiment, list.Items); err != nil {
			return reconcile.Result{}, err
		} else if namespace != "" {
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
			// TODO If there is an error, notify server that we failed to adopt the suggestion?
			log.Info("Creating new trial", "namespace", trial.Namespace, "observationURL", observationURL, "assignments", trial.Spec.Assignments)
			err = r.Create(context.TODO(), trial)
			return reconcile.Result{}, err
		}
	}

	// Reconcile each trial
	for _, t := range list.Items {
		// TODO Check if the trial has a remote server annotation, if not, we need to manually create the trial on the server before we can report it
		if okeanostrial.IsTrialFinished(&t) {
			if t.DeletionTimestamp == nil {
				// Delete the trial to force finalization
				err = r.Delete(context.TODO(), &t)
				return reconcile.Result{}, err
			} else {
				// Create an observation for the remote server
				observation := okeanosapi.Observation{}
				for _, c := range t.Status.Conditions {
					if c.Type == okeanosv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
						observation.Failed = true
					}
				}
				for k, v := range t.Spec.Values {
					observation.Values = append(observation.Values, okeanosapi.Value{
						Name:  k,
						Value: v,
						// TODO Error is the standard deviation for the metric
					})
				}

				// Send the observation to the server
				observationURL := t.GetAnnotations()[annotationObservationURL]
				log.Info("Reporting observation", "namespace", t.Namespace, "observationURL", observationURL, "assignments", t.Spec.Assignments, "values", observation.Values)
				if err := r.api.ReportObservation(context.TODO(), observationURL, observation); err != nil {
					// This error only matters if the experiment itself is not deleted, otherwise ignore it so we can remove the finalizer
					if experiment.DeletionTimestamp == nil {
						return reconcile.Result{}, err
					}
				}

				// Remove the trial finalizer once we have sent information to the server
				for i, _ := range t.Finalizers {
					if t.Finalizers[i] == finalizer {
						t.Finalizers[i] = t.Finalizers[len(t.Finalizers)-1]
						t.Finalizers = t.Finalizers[:len(t.Finalizers)-1]
						err := r.Update(context.TODO(), &t)
						return reconcile.Result{}, err
					}
				}
			}
		} else if t.DeletionTimestamp != nil {
			// The trial was explicitly deleted before it finished, remove the finalizer so it can go away
			for i, _ := range t.Finalizers {
				if t.Finalizers[i] == finalizer {
					// TODO Notify the server that the trial was abandoned
					t.Finalizers[i] = t.Finalizers[len(t.Finalizers)-1]
					t.Finalizers = t.Finalizers[:len(t.Finalizers)-1]
					err := r.Update(context.TODO(), &t)
					return reconcile.Result{}, err
				}
			}
		} else if experiment.DeletionTimestamp != nil {
			// The experiment is deleted, delete the trial as well
			err = r.Delete(context.TODO(), &t)
			return reconcile.Result{}, err
		}
	}

	// Remove our finalizer if we have been deleted and all trials were reconciled
	if experiment.DeletionTimestamp != nil {
		for i, _ := range experiment.Finalizers {
			if experiment.Finalizers[i] == finalizer {
				experiment.Finalizers[i] = experiment.Finalizers[len(experiment.Finalizers)-1]
				experiment.Finalizers = experiment.Finalizers[:len(experiment.Finalizers)-1]
				err := r.Update(context.TODO(), experiment)
				return reconcile.Result{}, err
			}
		}
	}

	// No action
	return reconcile.Result{}, nil
}

func addFinalizer(experiment *okeanosv1alpha1.Experiment) bool {
	if experiment.DeletionTimestamp != nil {
		return false
	}
	for _, f := range experiment.Finalizers {
		if f == finalizer {
			return false
		}
	}
	experiment.Finalizers = append(experiment.Finalizers, finalizer)
	return true
}

func (r *ReconcileExperiment) syncWithServer(experiment *okeanosv1alpha1.Experiment) (bool, error) {
	experimentURL := experiment.GetAnnotations()[annotationExperimentURL]
	suggestionURL := experiment.GetAnnotations()[annotationSuggestionURL]

	if experiment.GetReplicas() > 0 {
		// Define the experiment on the server
		if experimentURL == "" {
			n := okeanosapi.NewExperimentName(experiment.Name)
			e := okeanosapi.Experiment{}
			copyExperimentToRemote(experiment, &e)

			log.Info("Creating remote experiment", "name", n)
			if experimentRef, err := r.api.CreateExperiment(context.TODO(), n, e); err == nil {
				experiment.GetAnnotations()[annotationExperimentURL] = experimentRef
				return true, nil
			} else {
				return false, err
			}
		}

		// Update information only populated by server after PUT
		if suggestionURL == "" && experimentURL != "" {
			e, err := r.api.GetExperiment(context.TODO(), experimentURL)
			if err != nil {
				return false, err
			}

			// Since we have the server representation, enforce a cap on the replica count
			// NOTE: Do the update in memory, we will only persist it if the suggestion URL needs updating
			if experiment.GetReplicas() > int(e.Optimization.ParallelSuggestions) && e.Optimization.ParallelSuggestions > 0 {
				*experiment.Spec.Replicas = e.Optimization.ParallelSuggestions
			}

			// The suggestion reference may be missing because the experiment isn't producing suggestions anymore
			if e.SuggestionRef != "" {
				experiment.GetAnnotations()[annotationSuggestionURL] = e.SuggestionRef
				return true, nil
			}
		}
	}

	// Notify the server of the deletion
	if experiment.DeletionTimestamp != nil && experimentURL != "" {
		if err := r.api.DeleteExperiment(context.TODO(), experimentURL); err != nil {
			log.Error(err, "Failed to delete experiment", "experimentURL", experimentURL)
		}
		delete(experiment.GetAnnotations(), annotationExperimentURL)
		delete(experiment.GetAnnotations(), annotationSuggestionURL)
		experiment.SetReplicas(0)
		return true, nil
	}

	return false, nil
}

// Copy the custom resource state into a client API representation
func copyExperimentToRemote(experiment *okeanosv1alpha1.Experiment, e *okeanosapi.Experiment) {
	e.Optimization = experiment.Spec.Optimization

	e.Parameters = nil
	for _, p := range experiment.Spec.Parameters {
		cp := okeanosapi.Parameter{Name: p.Name}
		if len(p.Values) > 0 {
			cp.Type = okeanosapi.ParameterTypeString
			if p.Default != "" {
				cp.Default = p.Default
			}
			cp.Values = p.Values
		} else if p.Min != p.Max {
			cp.Type = okeanosapi.ParameterTypeInteger
			if d, err := strconv.Atoi(p.Default); err == nil {
				cp.Default = d
			}
			cp.Bounds = okeanosapi.Bounds{
				Min: json.Number(strconv.FormatInt(p.Min, 10)),
				Max: json.Number(strconv.FormatInt(p.Max, 10)),
			}
		} else if p.MinFloat != p.MaxFloat {
			cp.Type = okeanosapi.ParameterTypeDouble
			if d, err := strconv.ParseFloat(p.Default, 64); err == nil {
				cp.Default = d
			}
			cp.Bounds = okeanosapi.Bounds{
				Min: json.Number(strconv.FormatFloat(p.MinFloat, 'f', -1, 64)),
				Max: json.Number(strconv.FormatFloat(p.MaxFloat, 'f', -1, 64)),
			}
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
	// Start with the trial template
	experiment.Spec.Template.ObjectMeta.DeepCopyInto(&trial.ObjectMeta)
	experiment.Spec.Template.Spec.DeepCopyInto(&trial.Spec)

	// Overwrite the target namespace unless we are only running a single trial on the cluster
	if experiment.GetReplicas() > 1 || experiment.Spec.NamespaceSelector != nil || experiment.Spec.Template.Namespace != "" {
		trial.Spec.TargetNamespace = namespace
	}

	trial.Finalizers = append(trial.Finalizers, finalizer)

	if trial.Namespace == "" {
		trial.Namespace = namespace
	}

	if trial.Name == "" {
		if trial.Namespace != experiment.Namespace {
			trial.Name = experiment.Name
		} else if trial.GenerateName == "" {
			trial.GenerateName = experiment.Name + "-"
		}
	}

	if len(trial.Labels) == 0 {
		trial.Labels = experiment.GetDefaultLabels()
	}

	if trial.Annotations == nil {
		trial.Annotations = make(map[string]string)
	}

	if trial.Spec.Values == nil {
		trial.Spec.Values = make(map[string]float64)
	}

	if trial.Spec.ExperimentRef == nil {
		trial.Spec.ExperimentRef = experiment.GetSelfReference()
	}

	if trial.Spec.Selector == nil && experiment.Spec.JobTemplate != nil {
		trial.Spec.Selector = metav1.SetAsLabelSelector(experiment.Spec.JobTemplate.Labels)
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
