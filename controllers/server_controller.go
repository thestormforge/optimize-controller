/*
Copyright 2020 GramLabs, Inc.

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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/controller"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/meta"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	"github.com/thestormforge/optimize-controller/v2/internal/validation"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var (
	defaultServerTrialTTLSecondsAfterFinished = int32((4 * time.Hour) / time.Second)
	defaultServerTrialTTLSecondsAfterFailure  = int32((48 * time.Hour) / time.Second)
)

// trialCreationRateLimit returns the configured rate for allowing trial creations, the
// default (and minimum allowed) rate is 1 trial per second.
func trialCreationRateLimit(log logr.Logger) rate.Limit {
	// NOTE: If we are changing this to a lot of different values, it should be moved to the configuration
	trialCreationInterval, ok := os.LookupEnv("STORMFORGE_TRIAL_CREATION_INTERVAL")
	if !ok {
		return rate.Every(time.Second)
	}

	d, err := time.ParseDuration(trialCreationInterval)
	if err != nil || d < time.Second {
		log.Info("Ignoring invalid custom trial creation interval", "trialCreationInterval", trialCreationInterval)
		return rate.Every(time.Second)
	}

	log.Info("Using custom trial creation interval", "trialCreationInterval", trialCreationInterval)
	return rate.Every(d)
}

// ServerReconciler reconciles a experiment and trial objects with a remote server
type ServerReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	ExperimentsAPI experimentsv1alpha1.API

	trialCreation *rate.Limiter
}

// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=experiments,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=trials,verbs=list;watch;create;update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list

func (r *ServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("experiment", req.NamespacedName)

	// Fetch the experiment state from the cluster
	exp := &optimizev1beta2.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, exp); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Check to see if the experiment is exempt from server operations
	if !server.IsServerSyncEnabled(exp) {
		return ctrl.Result{}, nil
	}

	// Create the experiment on the server
	if result, err := r.createExperiment(ctx, log, exp); result != nil {
		return *result, err
	}

	// Get the current list of trials
	// NOTE: No need to use limits, the cache will just return the full list anyway
	trialList := &optimizev1beta2.TrialList{}
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
			// TODO Combine report and abandon into one function
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
	if exp.GetAnnotations()[optimizev1beta2.AnnotationNextTrialURL] != "" && activeTrials < exp.Replicas() {
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
	if r.ExperimentsAPI == nil {
		// Compute the UA string comment using the Kube API server information
		var comment string
		if dc, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig()); err == nil {
			if serverVersion, err := dc.ServerVersion(); err == nil && serverVersion.GitVersion != "" {
				comment = fmt.Sprintf("Kubernetes %s", strings.TrimPrefix(serverVersion.GitVersion, "v"))
			}
		}

		api, err := server.NewExperimentAPI(context.Background(), comment)
		if err != nil {
			return err
		}

		r.ExperimentsAPI = api
	}

	// Enforce trial creation rate limit (no burst! that is the whole point)
	r.trialCreation = rate.NewLimiter(trialCreationRateLimit(r.Log), 1)

	// To search for namespaces by name, we need to index them
	_ = mgr.GetCache().IndexField(&corev1.Namespace{}, "metadata.name", func(obj runtime.Object) []string { return []string{obj.(*corev1.Namespace).Name} })

	return ctrl.NewControllerManagedBy(mgr).
		Named("server").
		For(&optimizev1beta2.Experiment{}).
		WithEventFilter(&createFilter{}).
		Complete(r)
}

// createFilter ignores the experiment create event to allow the experiment status to stabilize more naturally
type createFilter struct{}

func (*createFilter) Create(event.CreateEvent) bool   { return false }
func (*createFilter) Delete(event.DeleteEvent) bool   { return true }
func (*createFilter) Update(event.UpdateEvent) bool   { return true }
func (*createFilter) Generic(event.GenericEvent) bool { return true }

// listTrials retrieves the list of trial objects matching the specified selector
func (r *ServerReconciler) listTrials(ctx context.Context, trialList *optimizev1beta2.TrialList, selector *metav1.LabelSelector) error {
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}
	return r.List(ctx, trialList, client.MatchingLabelsSelector{Selector: s})
}

// createExperiment will create a new experiment on the server using the cluster state; any default values from the
// server will be copied back into cluster along with the URLs needed for future interactions with server.
func (r *ServerReconciler) createExperiment(ctx context.Context, log logr.Logger, exp *optimizev1beta2.Experiment) (*ctrl.Result, error) {
	// If the server finalizer is already present, do not try to recreate the experiment on the server
	if meta.HasFinalizer(exp, server.Finalizer) {
		return nil, nil
	}

	// Rely on the status to prevent re-running an already run experiment
	if experiment.IsFinished(exp) {
		return nil, nil
	}

	// Convert the cluster state into a server representation
	n, e, b, err := server.FromCluster(exp)
	if err != nil {
		if experiment.FailExperiment(exp, "InvalidExperimentDefinition", err) {
			err := r.Update(ctx, exp)
			return controller.RequeueConflict(err)
		}
		return &ctrl.Result{}, err
	}

	// Create the experiment remotely
	var ee experimentsv1alpha1.Experiment
	if u := exp.GetAnnotations()[optimizev1beta2.AnnotationExperimentURL]; u != "" {
		ee, err = r.ExperimentsAPI.CreateExperiment(ctx, u, *e)
	} else {
		ee, err = r.ExperimentsAPI.CreateExperimentByName(ctx, n, *e)
	}
	if err != nil {
		if experiment.FailExperiment(exp, "ServerCreateFailed", err) {
			err := r.Update(ctx, exp)
			return controller.RequeueConflict(err)
		}
		return &ctrl.Result{}, err
	}

	// Check that the server and the cluster have a compatible experiment definition
	if err := validation.CheckDefinition(exp, &ee); err != nil {
		return &ctrl.Result{}, err
	}

	// Best effort to send a baseline suggestion along with the experiment creation
	if b != nil {
		if _, err := r.ExperimentsAPI.CreateTrial(ctx, ee.Link(api.RelationTrials), *b); err != nil {
			log.Error(err, "Failed to suggest experiment baseline")
		}
	}

	// Apply the server response to the cluster state
	server.ToCluster(exp, &ee)

	// Update the experiment
	if err = r.Update(ctx, exp); err != nil {
		return controller.RequeueConflict(err)
	}

	log.Info("Created remote experiment", "experimentURL", exp.Annotations[optimizev1beta2.AnnotationExperimentURL])
	return nil, nil
}

// unlinkExperiment will delete the experiment from the server using the URLs recorded in the cluster; the finalizer
// added when the experiment was created on the server will also be removed
func (r *ServerReconciler) unlinkExperiment(ctx context.Context, log logr.Logger, exp *optimizev1beta2.Experiment) (*ctrl.Result, error) {
	// Try to remove the finalizer, if it is already gone we do not need to do anything
	if !meta.RemoveFinalizer(exp, server.Finalizer) {
		return nil, nil
	}

	// Check to see if we should delete the experiment on the server
	// NOTE: Deleting the server experiment is unusual, we normally want to preserve the server data
	if u := exp.GetAnnotations()[optimizev1beta2.AnnotationExperimentURL]; u != "" && server.DeleteServerExperiment(exp) {
		if err := r.ExperimentsAPI.DeleteExperiment(ctx, u); controller.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to delete server experiment")
		}
	}

	// Update the in-cluster experiment to reflect it is no-longer linked to the server
	experiment.ApplyCondition(&exp.Status, optimizev1beta2.ExperimentComplete, corev1.ConditionTrue, "", "", nil)
	delete(exp.GetAnnotations(), optimizev1beta2.AnnotationExperimentURL)
	delete(exp.GetAnnotations(), optimizev1beta2.AnnotationNextTrialURL)
	delete(exp.GetAnnotations(), optimizev1beta2.AnnotationReportTrialURL)
	if err := r.Update(ctx, exp); err != nil {
		return controller.RequeueConflict(err)
	}

	log.Info("Unlinked remote experiment")
	return nil, nil
}

// nextTrial will try to obtain a suggestion from the server and create the corresponding cluster state in the form of
// a trial; if the cluster can not accommodate additional trials at the time of invocation, not action will be taken
func (r *ServerReconciler) nextTrial(ctx context.Context, log logr.Logger, exp *optimizev1beta2.Experiment, trialList *optimizev1beta2.TrialList) (*ctrl.Result, error) {
	// Enforce a rate limit on trial creation
	res := r.trialCreation.Reserve()
	if !res.OK() {
		// This should never happen, if it does, just drop the reconciliation
		log.Info("Trial creation reservation failed", "limit", r.trialCreation.Limit(), "burst", r.trialCreation.Burst())
		return nil, nil
	}
	if d := res.Delay(); d > 0 {
		res.Cancel()
		return &ctrl.Result{RequeueAfter: d}, nil
	}

	// Determine the namespace (if any) to use for the trial
	namespace, err := experiment.NextTrialNamespace(ctx, r, exp, trialList)
	if err != nil {
		return &ctrl.Result{}, err
	}
	if namespace == "" {
		return nil, nil
	}

	// Obtain a suggestion from the server
	suggestion, err := r.ExperimentsAPI.NextTrial(ctx, exp.GetAnnotations()[optimizev1beta2.AnnotationNextTrialURL])
	if err != nil {
		if experiment.StopExperiment(exp, err) {
			err := r.Update(ctx, exp)
			return controller.RequeueConflict(err)
		}
		return controller.RequeueIfUnavailable(err)
	}

	// Generate a new trial from the template on the experiment and apply the server response
	t := &optimizev1beta2.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t)
	t.Namespace = namespace
	server.ToClusterTrial(t, &suggestion)

	// Since the trial originated from the server, we can delete it out of the cluster (require both TTLs to be unset)
	if t.Spec.TTLSecondsAfterFinished == nil && t.Spec.TTLSecondsAfterFailure == nil {
		t.Spec.TTLSecondsAfterFinished = &defaultServerTrialTTLSecondsAfterFinished
		t.Spec.TTLSecondsAfterFailure = &defaultServerTrialTTLSecondsAfterFailure
	}

	// Log a warning if the reportTrialURL is missing
	reportTrialURL := t.GetAnnotations()[optimizev1beta2.AnnotationReportTrialURL]
	if reportTrialURL == "" {
		log.Info("Trial is missing a reporting URL")
	}

	// Create the trial
	if err := r.Create(ctx, t); err != nil {
		// If creation fails, abandon the suggestion (ignoring those errors)
		if reportTrialURL != "" {
			_ = r.ExperimentsAPI.AbandonRunningTrial(ctx, reportTrialURL)
		}
		return &ctrl.Result{}, err
	}

	log.Info("Created new trial", "reportTrialURL", reportTrialURL, "assignments", t.Spec.Assignments)
	return nil, nil
}

// reportTrial will report the values from a finished in cluster trial back to the server
func (r *ServerReconciler) reportTrial(ctx context.Context, log logr.Logger, t *optimizev1beta2.Trial) (*ctrl.Result, error) {
	if !meta.RemoveFinalizer(t, server.Finalizer) {
		return nil, nil
	}

	// Update the log with additional context about the trial
	trialValues := server.FromClusterTrial(t)
	log = log.WithValues("values", trialValues)
	for i := range t.Status.Conditions {
		c := t.Status.Conditions[i]
		if c.Type == optimizev1beta2.TrialFailed && c.Status == corev1.ConditionTrue {
			log = log.WithValues("failureReason", c.Reason, "failureMessage", c.Message)
			break
		}
	}

	// If there is a report trial URL, report the values
	reportTrialURL := t.GetAnnotations()[optimizev1beta2.AnnotationReportTrialURL]
	log = log.WithValues("reportTrialURL", reportTrialURL)
	if reportTrialURL != "" {
		err := r.ExperimentsAPI.ReportTrial(ctx, reportTrialURL, *trialValues)
		if controller.IgnoreReportError(err) != nil {
			return &ctrl.Result{}, err
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
func (r *ServerReconciler) abandonTrial(ctx context.Context, log logr.Logger, t *optimizev1beta2.Trial) (*ctrl.Result, error) {
	if !meta.RemoveFinalizer(t, server.Finalizer) {
		return nil, nil
	}

	if reportTrialURL := t.GetAnnotations()[optimizev1beta2.AnnotationReportTrialURL]; reportTrialURL != "" {
		err := r.ExperimentsAPI.AbandonRunningTrial(ctx, reportTrialURL)
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
