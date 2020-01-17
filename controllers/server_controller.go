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
	"sync"

	"github.com/go-logr/logr"
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/experiment"
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/server"
	"github.com/redskyops/k8s-experiment/internal/trial"
	"github.com/redskyops/k8s-experiment/internal/validation"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskyapi "github.com/redskyops/k8s-experiment/redskyapi/redsky/v1alpha1"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// ServerReconciler reconciles a experiment and trial objects with a remote server
type ServerReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	RedSkyAPI redskyapi.API

	trialCreation map[string]*rate.Limiter
	tcmu          sync.Mutex
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=list;watch;create;update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list

func (r *ServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("experiment", req.NamespacedName)

	// Fetch the experiment state from the cluster
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, exp); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Create the experiment on the server
	if exp.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL] == "" && exp.Replicas() > 0 {
		if result, err := r.createExperiment(ctx, log, exp); result != nil {
			return *result, err
		}
	}

	// Get the current list of trials
	// NOTE: No need to use limits, the cache will just return the full list anyway
	trialList := &redskyv1alpha1.TrialList{}
	if err := r.listTrials(ctx, trialList, exp.TrialSelector()); err != nil {
		return ctrl.Result{}, err
	}

	// Look for active, finished or abandoned trials
	var activeTrials int32
	var trialHasFinalizer bool
	for i := range trialList.Items {
		t := &trialList.Items[i]
		tlog := log.WithValues("trial", t.Namespace+"/"+t.Name)

		// Count active trials
		if trial.IsActive(t) {
			activeTrials++
		}

		// Trials that have the server finalizer may need to be reported
		if meta.HasFinalizer(t, server.Finalizer) {
			if trial.IsFinished(t) {
				if result, err := r.reportTrial(ctx, tlog, t); result != nil {
					return *result, err
				}
			} else if trial.IsAbandoned(t) {
				if result, err := r.abandonTrial(ctx, tlog, t); result != nil {
					return *result, err
				}
			} else {
				trialHasFinalizer = true
			}
		}
	}

	// Create a new trial if necessary
	if exp.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL] != "" && activeTrials < exp.Replicas() {
		if result, err := r.nextTrial(ctx, log, exp, trialList); result != nil {
			return *result, err
		}
	}

	// Unlink the experiment from the server (only when all trial finalizers are removed)
	if !exp.DeletionTimestamp.IsZero() && !trialHasFinalizer {
		if result, err := r.unlinkExperiment(ctx, log, exp); result != nil {
			return *result, err
		}
	}

	// Nothing to do
	return ctrl.Result{}, nil
}

func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if _, err := r.RedSkyAPI.Options(context.Background()); err != nil {
		// TODO We may need to ignore transient errors to prevent skipping setup in recoverable or "not ready" scenarios
		// TODO We may need to look for specific errors to skip setup, i.e. "ErrConfigAddressMissing"
		r.Log.Info("Red Sky API is unavailable, skipping setup", "message", err.Error())
		return nil
	}

	// To search for namespaces by name, we need to index them
	_ = mgr.GetCache().IndexField(&corev1.Namespace{}, ".metadata.name", func(obj runtime.Object) []string { return []string{obj.(*corev1.Namespace).Name} })

	return ctrl.NewControllerManagedBy(mgr).
		Named("server").
		For(&redskyv1alpha1.Experiment{}).
		WithEventFilter(&createFilter{}).
		Complete(r)
}

// createFilter ignores the experiment create event to allow the experiment status to stabilize more naturally
type createFilter struct{}

func (*createFilter) Create(e event.CreateEvent) bool { return false }
func (*createFilter) Delete(event.DeleteEvent) bool   { return true }
func (*createFilter) Update(event.UpdateEvent) bool   { return true }
func (*createFilter) Generic(event.GenericEvent) bool { return true }

// listTrials retrieves the list of trial objects matching the specified selector
func (r *ServerReconciler) listTrials(ctx context.Context, trialList *redskyv1alpha1.TrialList, selector *metav1.LabelSelector) error {
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}
	return r.List(ctx, trialList, client.MatchingLabelsSelector{Selector: s})
}

// createExperiment will create a new experiment on the server using the cluster state; any default values from the
// server will be copied back into cluster along with the URLs needed for future interactions with server.
func (r *ServerReconciler) createExperiment(ctx context.Context, log logr.Logger, exp *redskyv1alpha1.Experiment) (*ctrl.Result, error) {
	// Convert the cluster state into a server representation
	n, e := server.FromCluster(exp)
	ee, err := r.RedSkyAPI.CreateExperiment(ctx, n, *e)
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Check that the server and the cluster have a compatible experiment definition
	if err := validation.CheckDefinition(exp, &ee); err != nil {
		return &ctrl.Result{}, err
	}

	// Apply the server response to the cluster state
	server.ToCluster(exp, &ee)

	// Update the experiment
	if err = r.Update(ctx, exp); err != nil {
		return controller.RequeueConflict(err)
	}

	log.Info("Created remote experiment", "experimentURL", exp.Annotations[redskyv1alpha1.AnnotationExperimentURL])
	return nil, nil
}

// unlinkExperiment will delete the experiment from the server using the URLs recorded in the cluster; the finalizer
// added when the experiment was created on the server will also be removed
func (r *ServerReconciler) unlinkExperiment(ctx context.Context, log logr.Logger, exp *redskyv1alpha1.Experiment) (*ctrl.Result, error) {
	// Try to remove the finalizer, if it is already gone we do not need to do anything
	if !meta.RemoveFinalizer(exp, server.Finalizer) {
		return nil, nil
	}

	// We do not actually delete the experiment from the server to preserve the data, for example, in a multi-cluster
	// experiment we would require that the experiment still exist for all the other clusters.
	// We also would not want a reset (which deletes the CRD) to wipe out the data on the server

	delete(exp.GetAnnotations(), redskyv1alpha1.AnnotationExperimentURL)
	delete(exp.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL)

	// Update the experiment
	if err := r.Update(ctx, exp); err != nil {
		return controller.RequeueConflict(err)
	}

	// Clean up limiter
	delete(r.trialCreation, exp.Name)

	log.Info("Unlinked remote experiment")
	return nil, nil
}

// nextTrial will try to obtain a suggestion from the server and create the corresponding cluster state in the form of
// a trial; if the cluster can not accommodate additional trials at the time of invocation, not action will be taken
func (r *ServerReconciler) nextTrial(ctx context.Context, log logr.Logger, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (*ctrl.Result, error) {
	// Enforce a rate limit on trial creation
	limiter := r.trialCreationLimit(exp.Name)
	if res := limiter.Reserve(); res.OK() {
		if d := res.Delay(); d > 0 {
			res.Cancel()
			return &ctrl.Result{RequeueAfter: d}, nil
		}
	}

	// Determine the namespace (if any) to use for the trial
	namespace, err := experiment.NextTrialNamespace(r, ctx, exp, trialList)
	if err != nil {
		return &ctrl.Result{}, err
	}
	if namespace == "" {
		return nil, nil
	}

	// Obtain a suggestion from the server
	suggestion, err := r.RedSkyAPI.NextTrial(ctx, exp.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL])
	if err != nil {
		if server.StopExperiment(exp, err) {
			err := r.Update(ctx, exp)
			return controller.RequeueConflict(err)
		}
		return controller.RequeueIfUnavailable(err)
	}

	// Generate a new trial from the template on the experiment and apply the server response
	t := &redskyv1alpha1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t)
	t.Namespace = namespace
	server.ToClusterTrial(t, &suggestion)

	// Create the trial
	if err := r.Create(ctx, t); err != nil {
		// If creation fails, abandon the suggestion (ignoring those errors)
		if url := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]; url != "" {
			_ = r.RedSkyAPI.AbandonRunningTrial(ctx, url)
		}
		return &ctrl.Result{}, err
	}

	log.Info("Created new trial", "reportTrialURL", t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL], "assignments", t.Spec.Assignments)
	return nil, nil
}

// reportTrial will report the values from a finished in cluster trial back to the server
func (r *ServerReconciler) reportTrial(ctx context.Context, log logr.Logger, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	if !meta.RemoveFinalizer(t, server.Finalizer) {
		return nil, nil
	}

	if reportTrialURL := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]; reportTrialURL != "" {
		trialValues := server.FromClusterTrial(t)
		err := r.RedSkyAPI.ReportTrial(ctx, reportTrialURL, *trialValues)
		if controller.IgnoreReportError(err) != nil {
			return &ctrl.Result{}, err
		}

		// Shadow the logger reference with one that will produce more contextual details
		log = log.WithValues("reportTrialURL", reportTrialURL, "values", trialValues)
		for i := range t.Status.Conditions {
			c := t.Status.Conditions[i]
			if c.Type == redskyv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
				log = log.WithValues("failureReason", c.Reason, "failureMessage", c.Message)
				break
			}
		}
	}

	// Update the trial
	if err := r.Update(ctx, t); err != nil {
		return controller.RequeueConflict(err)
	}

	log.Info("Reported trial")
	return nil, nil
}

// abandonTrial will remove the finalizer and try to notify the server that the trial will not be reported
func (r *ServerReconciler) abandonTrial(ctx context.Context, log logr.Logger, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	if !meta.RemoveFinalizer(t, server.Finalizer) {
		return nil, nil
	}

	if reportTrialURL := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]; reportTrialURL != "" {
		err := r.RedSkyAPI.AbandonRunningTrial(ctx, reportTrialURL)
		if controller.IgnoreNotFound(err) != nil {
			return &ctrl.Result{}, err
		}

		// Shadow the logger reference with one that will produce more contextual details
		log = log.WithValues("reportTrialURL", reportTrialURL)
	}

	// Update the trial
	if err := r.Update(ctx, t); err != nil {
		return controller.RequeueConflict(err)
	}

	log.Info("Abandoned trial")
	return nil, nil
}

func (r *ServerReconciler) trialCreationLimit(name string) *rate.Limiter {
	r.tcmu.Lock()
	defer r.tcmu.Unlock()
	l, ok := r.trialCreation[name]
	if !ok {
		if r.trialCreation == nil {
			r.trialCreation = make(map[string]*rate.Limiter)
		}
		l = rate.NewLimiter(1, 1)
		r.trialCreation[name] = l
	}
	return l
}
