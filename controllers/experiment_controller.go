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
	redskyapi "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskyexperiment "github.com/redskyops/k8s-experiment/pkg/controller/experiment"
	redskytrial "github.com/redskyops/k8s-experiment/pkg/controller/trial"
	"github.com/redskyops/k8s-experiment/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ExperimentReconciler reconciles a Experiment object
type ExperimentReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	RedSkyAPI redskyapi.API
}

func (r *ExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Experiment{}).
		Owns(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments/status,verbs=get;update;patch

func (r *ExperimentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("experiment", req.NamespacedName)

	// Fetch the Experiment instance
	experiment := &redskyv1alpha1.Experiment{}
	err := r.Get(ctx, req.NamespacedName, experiment)
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
		err := r.Update(ctx, experiment)
		return reconcile.Result{}, err
	}

	// Synchronize with the server
	if dirty, err := r.syncWithServer(experiment, ctx, log); err != nil {
		return reconcile.Result{}, err
	} else if dirty {
		err = r.Update(ctx, experiment)
		return reconcile.Result{}, err
	}

	// Find trials labeled for this experiment
	list := &redskyv1alpha1.TrialList{}
	matchingSelector, err := util.MatchingSelector(experiment.GetTrialSelector())
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := r.List(ctx, list, matchingSelector); err != nil {
		return reconcile.Result{}, err
	}

	// Add an additional trial if needed
	nextTrialURL := experiment.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL]
	if nextTrialURL != "" {
		// Find an available namespace
		// TODO If namespace comes back empty should we requeue with a delay instead of falling through?
		if namespace, err := redskyexperiment.FindAvailableNamespace(r, experiment, list.Items); err != nil {
			return reconcile.Result{}, err
		} else if namespace != "" {
			trial := &redskyv1alpha1.Trial{}
			redskyexperiment.PopulateTrialFromTemplate(experiment, trial, namespace)
			if err := controllerutil.SetControllerReference(experiment, trial, r.Scheme); err != nil {
				return reconcile.Result{}, err
			}

			// Obtain a suggestion from the server
			suggestion, err := r.RedSkyAPI.NextTrial(ctx, nextTrialURL)
			if err != nil {
				if aerr, ok := err.(*redskyapi.Error); ok {
					switch aerr.Type {
					case redskyapi.ErrExperimentStopped:
						// The experiment is stopped, set replicas to 0 to prevent further interaction with the server
						experiment.SetReplicas(0)
						delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL) // HTTP "Gone" semantics require us to purge this
						err = r.Update(ctx, experiment)
						return reconcile.Result{}, err
					case redskyapi.ErrTrialUnavailable:
						// No suggestions available, wait to requeue until after the retry delay
						return reconcile.Result{Requeue: true, RequeueAfter: aerr.RetryAfter}, nil
					}
				}
				return reconcile.Result{}, err
			}

			// Add the information from the server
			trial.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL] = suggestion.ReportTrial
			for _, a := range suggestion.Assignments {
				if v, err := a.Value.Int64(); err == nil {
					trial.Spec.Assignments = append(trial.Spec.Assignments, redskyv1alpha1.Assignment{
						Name:  a.ParameterName,
						Value: v,
					})
				}
			}

			// Create the trial
			// TODO If there is an error, notify server that we failed to adopt the suggestion?
			r.Log.Info("Creating new trial", "namespace", trial.Namespace, "reportTrialURL", suggestion.ReportTrial, "assignments", trial.Spec.Assignments)
			err = r.Create(ctx, trial)
			return reconcile.Result{}, err
		}
	}

	// Reconcile each trial
	for _, t := range list.Items {
		// TODO Check if the trial has a remote server annotation, if not, we need to manually create the trial on the server before we can report it
		if redskytrial.IsTrialFinished(&t) {
			if t.DeletionTimestamp == nil {
				// Delete the trial to force finalization
				err = r.Delete(ctx, &t)
				return reconcile.Result{}, err
			} else {
				// Create an observation for the remote server
				trialValues := redskyapi.TrialValues{}
				if err := redskyexperiment.ConvertTrialValues(&t, &trialValues); err != nil {
					return reconcile.Result{}, err
				}

				// Send the observation to the server
				reportTrialURL := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]
				r.Log.Info("Reporting trial", "namespace", t.Namespace, "reportTrialURL", reportTrialURL, "assignments", t.Spec.Assignments, "values", trialValues)
				if err := r.RedSkyAPI.ReportTrial(ctx, reportTrialURL, trialValues); err != nil {
					// This error only matters if the experiment itself is not deleted, otherwise ignore it so we can remove the finalizer
					if experiment.DeletionTimestamp == nil {
						return reconcile.Result{}, err
					}
				}

				// Remove the trial finalizer once we have sent information to the server
				for i := range t.Finalizers {
					if t.Finalizers[i] == redskyexperiment.Finalizer {
						t.Finalizers[i] = t.Finalizers[len(t.Finalizers)-1]
						t.Finalizers = t.Finalizers[:len(t.Finalizers)-1]
						err := r.Update(ctx, &t)
						return reconcile.Result{}, err
					}
				}
			}
		} else if t.DeletionTimestamp != nil {
			// The trial was explicitly deleted before it finished, remove the finalizer so it can go away
			for i := range t.Finalizers {
				if t.Finalizers[i] == redskyexperiment.Finalizer {
					// TODO Notify the server that the trial was abandoned
					t.Finalizers[i] = t.Finalizers[len(t.Finalizers)-1]
					t.Finalizers = t.Finalizers[:len(t.Finalizers)-1]
					err := r.Update(ctx, &t)
					return reconcile.Result{}, err
				}
			}
		} else if experiment.DeletionTimestamp != nil {
			// The experiment is deleted, delete the trial as well
			err = r.Delete(ctx, &t)
			return reconcile.Result{}, err
		}
	}

	// Remove our finalizer if we have been deleted and all trials were reconciled
	if experiment.DeletionTimestamp != nil {
		for i := range experiment.Finalizers {
			if experiment.Finalizers[i] == redskyexperiment.Finalizer {
				experiment.Finalizers[i] = experiment.Finalizers[len(experiment.Finalizers)-1]
				experiment.Finalizers = experiment.Finalizers[:len(experiment.Finalizers)-1]
				err := r.Update(ctx, experiment)
				return reconcile.Result{}, err
			}
		}
	}

	// No action
	return reconcile.Result{}, nil
}

func addFinalizer(experiment *redskyv1alpha1.Experiment) bool {
	if experiment.DeletionTimestamp != nil {
		return false
	}
	for _, f := range experiment.Finalizers {
		if f == redskyexperiment.Finalizer {
			return false
		}
	}
	experiment.Finalizers = append(experiment.Finalizers, redskyexperiment.Finalizer)
	return true
}

func (r *ExperimentReconciler) syncWithServer(experiment *redskyv1alpha1.Experiment, ctx context.Context, log logr.Logger) (bool, error) {
	experimentURL := experiment.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL]

	if experiment.GetReplicas() > 0 {
		// Define the experiment on the server
		if experimentURL == "" {
			n := redskyapi.NewExperimentName(experiment.Name)
			e := redskyapi.Experiment{}
			if err := redskyexperiment.ConvertExperiment(experiment, &e); err != nil {
				return false, err
			}

			log.Info("Creating remote experiment", "name", n)
			if ee, err := r.RedSkyAPI.CreateExperiment(ctx, n, e); err == nil {
				if experiment.GetAnnotations() == nil {
					experiment.SetAnnotations(make(map[string]string))
				}
				experiment.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL] = ee.Self
				experiment.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL] = ee.NextTrial
				if experiment.GetReplicas() > int(ee.Optimization.ParallelTrials) && ee.Optimization.ParallelTrials > 0 {
					*experiment.Spec.Replicas = ee.Optimization.ParallelTrials
				}
				return true, nil
			} else {
				return false, err
			}
		}
	}

	// Notify the server of the deletion
	if experiment.DeletionTimestamp != nil && experimentURL != "" {
		if err := r.RedSkyAPI.DeleteExperiment(ctx, experimentURL); err != nil {
			log.Error(err, "Failed to delete experiment", "experimentURL", experimentURL)
		}
		delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationExperimentURL)
		delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL)
		experiment.SetReplicas(0)
		return true, nil
	}

	return false, nil
}
