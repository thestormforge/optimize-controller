package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	okeanosclient "github.com/gramLabs/okeanos/pkg/apis/okeanos/client"
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

var log = logf.Log.WithName("controller")

// TODO Make this configurable at start up or as part of the manager configuration
// TODO An empty string effectively disables server communication
var baseUrl *url.URL

// TODO We need some type of client util to encapsulate this
var httpClient = &http.Client{Timeout: 10 * time.Second}

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileExperiment{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
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
	experimentURL := experiment.GetAnnotations()["okeanos.carbonrelay.com/experiment-url"]
	if experimentURL == "" && baseUrl != nil {
		// TODO This is bad, how do we avoid constructing the URL?
		remoteUrl := baseUrl
		if remoteUrl, err = url.Parse("/experiment/"); err != nil {
			return reconcile.Result{}, err
		}
		if remoteUrl, err = url.Parse(url.PathEscape(experiment.Name)); err != nil {
			return reconcile.Result{}, err
		}

		e := &okeanosclient.Experiment{}
		experiment.CopyToRemote(e)
		log.Info("Creating remote experiment", "remoteUrl", remoteUrl)
		if err = putJSON(remoteUrl.String(), e); err != nil {
			// Error posting the representation - requeue the request.
			return reconcile.Result{}, err
		}

		// Update the experiment URL, will create a new reconcile request
		experiment.GetAnnotations()["okeanos.carbonrelay.com/experiment-url"] = remoteUrl.String()
		err := r.Update(context.TODO(), experiment)
		return reconcile.Result{}, err
	}

	// Update the URL used to obtain suggestions (only populated by server after PUT)
	suggestionURL := experiment.GetAnnotations()["okeanos.carbonrelay.com/suggestion-url"]
	if suggestionURL == "" && experimentURL != "" && experiment.GetReplicas() > 0 {
		e := &okeanosclient.Experiment{}
		if err := getJSON(experimentURL, e); err != nil {
			// Unable to fetch the remote experiment - requeue the request
			return reconcile.Result{}, err
		}

		// TODO Perform additional validation on the local/remote state

		// The suggestion reference may be missing because the experiment isn't producing suggestions anymore
		if e.SuggestionRef != "" {
			log.Info("Obtaining suggestions from: %s", e.SuggestionRef)
			experiment.GetAnnotations()["okeanos.carbonrelay.com/suggestion-url"] = e.SuggestionRef
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

	// Record finished trials
	// TODO Is this something that should be implemented as a finalizer on the trial?
	for _, t := range list.Items {
		// TODO Is it necessary to filter deleted objects? Should that be a field selector on the List operation?
		if (t.Status.CompletionTime != nil || t.Status.Failed) && t.DeletionTimestamp == nil {
			// Post the observation
			log.Info("Creating remote observation", "trialName", t.Name)
			err := postJSON(t.GetAnnotations()["okeanos.carbonrelay.com/suggestion-url"], &okeanosclient.Observation{
				// TODO If the time is missing, use metadata from the trial itself (e.g. patch or wait failed)
				Start:   &t.Status.StartTime.Time,
				End:     &t.Status.CompletionTime.Time,
				Failed:  t.Status.Failed,
				Metrics: t.Spec.Metrics,
			})
			if err != nil {
				// The observation was not accepted, requeue the request
				return reconcile.Result{}, err
			}

			// Delete the trial, will create a new reconcile request
			err = r.Delete(context.TODO(), &t)
			return reconcile.Result{}, err
		}
	}

	// Add additional trials as needed
	if suggestionURL != "" && len(list.Items) < experiment.GetReplicas() {
		trial := &okeanosv1alpha1.Trial{}
		experiment.Spec.Template.ObjectMeta.DeepCopyInto(&trial.ObjectMeta)
		experiment.Spec.Template.Spec.DeepCopyInto(&trial.Spec)

		if trial.Name == "" {
			trial.Name = experiment.Name
		}
		// TODO Namespace?

		trial.Labels["experiment"] = experiment.Name
		if trial.Spec.Selector == nil {
			trial.Spec.Selector = metav1.SetAsLabelSelector(trial.Labels)
		}

		if trial.Spec.ExperimentRef == nil {
			// TODO There isn't a function that does this?
			trial.Spec.ExperimentRef = &corev1.ObjectReference{
				Kind:       experiment.TypeMeta.Kind,
				Name:       experiment.GetName(),
				Namespace:  experiment.GetNamespace(),
				APIVersion: experiment.TypeMeta.APIVersion,
			}
		}

		s, err := httpClient.Post(suggestionURL, "application/octet-stream", nil)
		if err != nil {
			return reconcile.Result{}, err
		}
		defer s.Body.Close()

		data := &okeanosclient.Suggestion{}
		if s.StatusCode >= 200 && s.StatusCode < 300 {
			// Add the suggestions to the trial
			if err = json.NewDecoder(s.Body).Decode(data); err != nil {
				return reconcile.Result{}, err
			}
			trial.Spec.Suggestions = data.Values

			// Preserve the location of the suggestion we obtained
			trial.GetAnnotations()["okeanos.carbonrelay.com/suggestion-url"] = s.Header.Get("Location")
		} else {
			switch s.StatusCode {
			case http.StatusGone:
				// There are no more suggestions, stop the experiment
				experiment.SetReplicas(0)
				experiment.GetAnnotations()["okeanos.carbonrelay.com/suggestion-url"] = ""
				err = r.Update(context.TODO(), experiment)
				return reconcile.Result{}, err
			case http.StatusServiceUnavailable:
				// The optimization service does not have available suggestions, give it a few seconds
				// TODO Get the expected timeout from the error response
				return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
			default:
				return reconcile.Result{}, fmt.Errorf("failed to obtain a suggestion: %s", s.Status)
			}
		}
		if err := controllerutil.SetControllerReference(experiment, trial, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		err = r.Create(context.TODO(), trial)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func getJSON(url string, target interface{}) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func putJSON(url string, request interface{}) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	r, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return nil
}

func postJSON(url string, request interface{}) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	r, err := httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return nil
}
