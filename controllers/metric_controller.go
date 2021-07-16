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
	"math"
	"strconv"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/controller"
	"github.com/thestormforge/optimize-controller/v2/internal/meta"
	"github.com/thestormforge/optimize-controller/v2/internal/metric"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	"github.com/thestormforge/optimize-controller/v2/internal/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MetricReconciler reconciles the metrics on a Trial object
type MetricReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=experiments,verbs=get;list;watch
// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list

func (r *MetricReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &optimizev1beta2.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil || r.ignoreTrial(t) {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	if result, err := r.evaluateMetrics(ctx, t, &now); result != nil {
		return *result, err
	}

	if result, err := r.collectMetrics(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *MetricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("metric").
		For(&optimizev1beta2.Trial{}).
		Complete(r)
}

func (r *MetricReconciler) ignoreTrial(t *optimizev1beta2.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue) {
		return true
	}

	// Ignore trials to do not have defined start/completion times
	// NOTE: This checks the status to prevent needing to reproduce job start/completion lookup logic
	if t.Status.StartTime == nil || t.Status.CompletionTime == nil {
		return true
	}

	// Do not ignore trials that have metrics pending collection
	for i := range t.Spec.Values {
		if t.Spec.Values[i].AttemptsRemaining > 0 {
			return false
		}
	}

	// Do not ignore trials if we haven't finished processing them
	if !(trial.CheckCondition(&t.Status, optimizev1beta2.TrialObserved, corev1.ConditionTrue)) {
		return false
	}

	// Ignore everything else
	return true
}

func (r *MetricReconciler) evaluateMetrics(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// TODO This check precludes manual additions of Values
	if len(t.Spec.Values) > 0 {
		return nil, nil
	}

	// Get the experiment
	exp := &optimizev1beta2.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Evaluate the metrics
	for _, m := range exp.Spec.Metrics {
		t.Spec.Values = append(t.Spec.Values, optimizev1beta2.Value{
			Name:              m.Name,
			AttemptsRemaining: 3,
		})
	}

	// Update the status to indicate that we will be collecting metrics
	if len(t.Spec.Values) > 0 {
		trial.ApplyCondition(&t.Status, optimizev1beta2.TrialObserved, corev1.ConditionUnknown, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func (r *MetricReconciler) collectMetrics(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Fetch the experiment
	exp := &optimizev1beta2.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Index a DEEP COPY of the metric definitions so we can safely make changes
	metrics := make(map[string]*optimizev1beta2.Metric, len(exp.Spec.Metrics))
	for i := range exp.Spec.Metrics {
		metrics[exp.Spec.Metrics[i].Name] = exp.Spec.Metrics[i].DeepCopy()
	}

	// Add more context to the log to help with debugging
	log := r.Log.WithValues(
		"trial", fmt.Sprintf("%s/%s", t.Namespace, t.Name),
		"startTime", t.Status.StartTime.Time,
		"completionTime", t.Status.CompletionTime.Time,
	)

	// Iterate over the metric values, looking for remaining attempts
	for i := range t.Spec.Values {
		v := &t.Spec.Values[i]
		if v.AttemptsRemaining <= 0 {
			continue
		}

		// Apply defaults to our local copy of the metric definition
		m := metrics[v.Name]
		if err := r.applyMetricDefaults(ctx, t, m); err != nil {
			return r.collectionAttempt(ctx, log, t, v, probeTime, err)
		}

		// Do any Kube API lookups while we have the API client
		target, err := r.target(ctx, t, m)
		if err != nil {
			return r.collectionAttempt(ctx, log, t, v, probeTime, err)
		}

		// Capture the metric value
		value, valueError, err := metric.CaptureMetric(ctx, log, t, m, target)
		if err != nil {
			return r.collectionAttempt(ctx, log, t, v, probeTime, err)
		}

		// Success, record the value
		v.Value = strconv.FormatFloat(value, 'f', -1, 64)
		if !math.IsNaN(valueError) {
			v.Error = strconv.FormatFloat(valueError, 'f', -1, 64)
		}

		return r.collectionAttempt(ctx, log, t, v, probeTime, nil)
	}

	// Wait until all metrics have been collected to fail the trial for an out of bounds metric
	// NOTE: We allow baseline trials to go through no matter what
	if !trial.IsBaseline(t, exp) {
		for i := range t.Spec.Values {
			v := &t.Spec.Values[i]
			if err := validation.CheckMetricBounds(metrics[v.Name], v); err != nil {
				trial.ApplyCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue, "MetricBound", err.Error(), probeTime)
				err := r.Update(ctx, t)
				return controller.RequeueConflict(err)
			}
		}
	}

	// We made it through all of the metrics without needing additional changes
	trial.ApplyCondition(&t.Status, optimizev1beta2.TrialObserved, corev1.ConditionTrue, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// collectionAttempt updates the status of the trial based on the outcome of an attempt to collect metric values.
func (r *MetricReconciler) collectionAttempt(ctx context.Context, log logr.Logger, t *optimizev1beta2.Trial, v *optimizev1beta2.Value, probeTime *metav1.Time, err error) (*ctrl.Result, error) {
	// Do not count retries against the remaining attempts
	if merr, ok := err.(*metric.CaptureError); ok && merr.RetryAfter > 0 {
		return &ctrl.Result{RequeueAfter: merr.RetryAfter}, nil
	}

	// Update the number of remaining attempts
	v.AttemptsRemaining--
	if err == nil || v.AttemptsRemaining < 0 {
		v.AttemptsRemaining = 0
	}

	// Update the probe time and ensure that trial observed is still explicitly false (i.e. we have started observation but it is not complete)
	trial.ApplyCondition(&t.Status, optimizev1beta2.TrialObserved, corev1.ConditionFalse, "", "", probeTime)

	// Fail the trial if there is an error and no attempts are left
	if err != nil && v.AttemptsRemaining == 0 {
		trial.ApplyCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue, "MetricFailed", err.Error(), probeTime)

		// Metric errors contain additional information which should be logged for debugging
		if merr, ok := err.(*metric.CaptureError); ok {
			log.Error(merr, "Metric collection failed", "address", merr.Address, "query", merr.Query)
		}
	}

	// Record the update
	return controller.RequeueConflict(r.Update(ctx, t))
}

// target looks up the Kubernetes object (if any) associated with a metric.
func (r *MetricReconciler) target(ctx context.Context, t *optimizev1beta2.Trial, m *optimizev1beta2.Metric) (runtime.Object, error) {
	if m.Type != optimizev1beta2.MetricKubernetes && m.Type != "" {
		return nil, nil
	}

	// No target, just return the trial
	if m.Target == nil {
		return t, nil
	}

	// If this is a request for the trial itself, then don't bother using the API
	if m.Target.GroupVersionKind() == t.GroupVersionKind() &&
		m.Target.Name == t.Name && m.Target.Namespace == t.Namespace {
		return t, nil
	}

	// If a name is specified, just get a single object
	if m.Target.Name != "" {
		target := &unstructured.Unstructured{}
		target.SetGroupVersionKind(m.Target.GroupVersionKind())
		if err := r.Get(ctx, m.Target.NamespacedName(), target); err != nil {
			return nil, err
		}
		return target, nil
	}

	// Convert the selector from a Kubernetes object to something the client can use
	sel, err := meta.MatchingSelector(m.Target.LabelSelector)
	if err != nil {
		return nil, err
	}

	// Fetch the list of matching resources
	target := &unstructured.UnstructuredList{}
	target.SetGroupVersionKind(m.Target.GroupVersionKind())
	if err := r.List(ctx, target, client.InNamespace(m.Target.Namespace), sel); err != nil {
		return nil, err
	}
	return target, nil
}

// applyMetricDefaults fills in default values for the supplied metric.
func (r *MetricReconciler) applyMetricDefaults(ctx context.Context, t *optimizev1beta2.Trial, m *optimizev1beta2.Metric) error {
	// Give Prometheus metrics a default URL
	if m.Type == optimizev1beta2.MetricPrometheus && m.URL == "" {
		m.URL = fmt.Sprintf("http://optimize-%[1]s-prometheus.%[1]s:9090/", t.Namespace)
	}

	if m.Target != nil {
		// If there is no kind on the target, assume they want the trial
		if m.Target.Kind == "" {
			m.Target.SetGroupVersionKind(t.GroupVersionKind())
			m.Target.Name, m.Target.Namespace = t.Name, t.Namespace
		}

		// If this is a reference to the trial job, make sure all the fields are filled out
		ref := &corev1.ObjectReference{Name: m.Target.Name, Namespace: m.Target.Namespace}
		ref.SetGroupVersionKind(m.Target.GroupVersionKind())
		if trial.IsTrialJobReference(t, ref) {
			m.Target.SetGroupVersionKind(batchv1.SchemeGroupVersion.WithKind("Job"))
			m.Target.Name, m.Target.Namespace = t.Name, t.Namespace
			if t.Spec.JobTemplate != nil && t.Spec.JobTemplate.Name != "" {
				m.Target.Name = t.Spec.JobTemplate.Name
			}
		}

		// Default to the trial namespace
		if m.Target.Namespace == "" {
			m.Target.Namespace = t.Namespace
		}
	}

	return nil
}
