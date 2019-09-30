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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list
// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments/status,verbs=get;update;patch

func (r *ExperimentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("experiment", req.NamespacedName)

	// Fetch the Experiment instance
	experiment := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, experiment); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	// Define the experiment on the server
	if experiment.GetReplicas() > 0 {
		if experimentURL := experiment.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL]; experimentURL == "" {
			n := redskyapi.NewExperimentName(experiment.Name)
			e := redskyapi.Experiment{}
			if err := redskyexperiment.ConvertExperiment(experiment, &e); err != nil {
				return ctrl.Result{}, err
			}

			log.Info("Creating remote experiment", "name", n)
			if ee, err := r.RedSkyAPI.CreateExperiment(ctx, n, e); err != nil {
				return ctrl.Result{}, err
			} else {
				// Update the experiment with information from the server
				if experiment.GetAnnotations() == nil {
					experiment.SetAnnotations(make(map[string]string))
				}
				experiment.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL] = ee.Self
				experiment.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL] = ee.NextTrial
				if experiment.GetReplicas() > ee.Optimization.ParallelTrials && ee.Optimization.ParallelTrials > 0 {
					experiment.Spec.Replicas = &ee.Optimization.ParallelTrials
				}
				if experiment.Spec.Budget == nil && ee.Optimization.ExperimentBudget > 0 {
					experiment.Spec.Budget = &ee.Optimization.ExperimentBudget
				}
				if experiment.Spec.BurnIn == nil && ee.Optimization.BurnIn > 0 {
					experiment.Spec.BurnIn = &ee.Optimization.BurnIn
				}
				util.AddFinalizer(experiment, redskyexperiment.ExperimentFinalizer)
				err = r.Update(ctx, experiment)
				return ctrl.Result{}, err
			}
		}
	}

	// Find trials labeled for this experiment
	list := &redskyv1alpha1.TrialList{}
	matchingSelector, err := util.MatchingSelector(experiment.GetTrialSelector())
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.List(ctx, list, matchingSelector); err != nil {
		return ctrl.Result{}, err
	}

	// Update the active trial count
	var activeTrials int32
	for i := range list.Items {
		if redskytrial.IsTrialActive(&list.Items[i]) {
			activeTrials++
		}
	}
	if experiment.Status.ActiveTrials != activeTrials {
		experiment.Status.ActiveTrials = activeTrials
		err := r.Update(ctx, experiment)
		return ctrl.Result{}, err
	}

	// Add an additional trial if needed
	if nextTrialURL := experiment.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL]; nextTrialURL != "" {
		// Find an available namespace
		if namespace, err := redskyexperiment.FindAvailableNamespace(r, experiment, list.Items); err != nil {
			return ctrl.Result{}, err
		} else if namespace != "" {
			// Create a new trial from the template on the experiment
			trial := &redskyv1alpha1.Trial{}
			redskyexperiment.PopulateTrialFromTemplate(experiment, trial, namespace)
			util.AddFinalizer(trial, redskyexperiment.ExperimentFinalizer)
			if err := controllerutil.SetControllerReference(experiment, trial, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}

			// Obtain a suggestion from the server
			suggestion, err := r.RedSkyAPI.NextTrial(ctx, nextTrialURL)
			if err != nil {
				if rse, ok := err.(*redskyapi.Error); ok && rse.Type == redskyapi.ErrExperimentStopped {
					// The experiment is stopped, set replicas to 0 to prevent further interaction with the server
					experiment.SetReplicas(0)
					delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL) // HTTP "Gone" semantics require us to purge this
					err := r.Update(ctx, experiment)
					return ctrl.Result{}, err
				}
				return util.RequeueTrialUnavailable(ctrl.Result{}, err)
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
			log.Info("Creating new trial", "namespace", trial.Namespace, "reportTrialURL", suggestion.ReportTrial, "assignments", trial.Spec.Assignments)
			err = r.Create(ctx, trial)
			return ctrl.Result{}, err
		}
	}

	// Reconcile each trial
	for i := range list.Items {
		trial := &list.Items[i]
		if redskytrial.IsTrialFinished(trial) {
			if reportTrialURL := trial.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]; reportTrialURL != "" {
				// Create an observation for the remote server
				trialValues := redskyapi.TrialValues{}
				if err := redskyexperiment.ConvertTrialValues(trial, &trialValues); err != nil {
					return ctrl.Result{}, err
				}

				// Remove the report trial URL from the trial before updating the server
				// TODO If the server operation were idempotent (i.e. a PUT instead of a POST), this would go after the server update
				delete(trial.GetAnnotations(), redskyv1alpha1.AnnotationReportTrialURL)
				if err := r.Update(ctx, trial); err != nil {
					return util.RequeueConflict(ctrl.Result{}, err)
				}

				// Include the reason for failure in the log message (note we return from the block so `log` goes out of scope)
				// TODO Should this just iterate over the conditions and include all non-empty reason/messages?
				if trialValues.Failed {
					for i := range trial.Status.Conditions {
						c := trial.Status.Conditions[i]
						if c.Type == redskyv1alpha1.TrialFailed {
							log = log.WithValues("failureReason", c.Reason, "failureMessage", c.Message)
						}
					}
				}

				// Send the observation to the server
				log.Info("Reporting trial", "namespace", trial.Namespace, "reportTrialURL", reportTrialURL, "assignments", trial.Spec.Assignments, "values", trialValues)
				if err := r.RedSkyAPI.ReportTrial(ctx, reportTrialURL, trialValues); err != nil && experiment.DeletionTimestamp.IsZero() {
					// This error only matters if the experiment itself is not deleted, otherwise ignore it so we can remove the finalizer
					// TODO Restore `reportTrialURL` annotation to retry?
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			// Remove the trial finalizer once we have sent information to the server
			if util.RemoveFinalizer(trial, redskyexperiment.ExperimentFinalizer) {
				err := r.Update(ctx, trial)
				return util.RequeueConflict(ctrl.Result{}, err)
			}

			// Delete the trial if necessary
			if redskyexperiment.NeedsCleanup(trial) {
				err = r.Delete(ctx, trial)
				return ctrl.Result{}, err
			}
		} else if !trial.DeletionTimestamp.IsZero() {
			// The trial was explicitly deleted before it finished, remove the finalizer from the trial so it can be garbage collected
			if util.RemoveFinalizer(trial, redskyexperiment.ExperimentFinalizer) {
				// TODO Notify the server that the trial was abandoned (ignore errors in case the whole experiment was abandoned)
				log.Info("Trial deleted before finishing", "name", trial.Name, "namespace", trial.Namespace)
				err := r.Update(ctx, trial)
				return util.RequeueConflict(ctrl.Result{}, err)
			}
		} else if !experiment.DeletionTimestamp.IsZero() {
			// The experiment was deleted before the trial finished, explicitly delete the trial so setup tasks can be cleaned up
			err = r.Delete(ctx, trial)
			return ctrl.Result{}, err
		}
	}

	// If any trial still has a finalizer, we need to wait for it to be removed
	for i := range list.Items {
		if util.HasFinalizer(&list.Items[i], redskyexperiment.ExperimentFinalizer) {
			return ctrl.Result{}, nil
		}
	}

	// Remove our finalizer if we have been deleted and all trials were reconciled
	if !experiment.DeletionTimestamp.IsZero() && util.RemoveFinalizer(experiment, redskyexperiment.ExperimentFinalizer) {
		// Also delete the experiment on the server if necessary
		if experimentURL := experiment.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL]; experimentURL != "" {
			if err := r.RedSkyAPI.DeleteExperiment(ctx, experimentURL); err != nil {
				if rserr, ok := err.(*redskyapi.Error); ok && rserr.Type != redskyapi.ErrExperimentNotFound {
					return ctrl.Result{}, err
				}
			}
			delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationExperimentURL)
			delete(experiment.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL)

			// Set the replicas to 0 to prevent us from recreating the remote experiment on the subsequent reconciliation
			experiment.SetReplicas(0)
		}
		err := r.Update(ctx, experiment)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
