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
	"net/url"
	"strconv"

	"github.com/go-logr/logr"

	redskyv1alpha1 "github.com/thestormforge/optimize-controller/api/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/controller"
	"github.com/thestormforge/optimize-controller/internal/meta"
	"github.com/thestormforge/optimize-controller/internal/metric"
	"github.com/thestormforge/optimize-controller/internal/trial"
	"github.com/thestormforge/optimize-controller/internal/validation"
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

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=services,verbs=list

func (r *MetricReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &redskyv1beta1.Trial{}
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
		For(&redskyv1beta1.Trial{}).
		Complete(r)
}

func (r *MetricReconciler) ignoreTrial(t *redskyv1beta1.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, redskyv1beta1.TrialFailed, corev1.ConditionTrue) {
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
	if !(trial.CheckCondition(&t.Status, redskyv1beta1.TrialObserved, corev1.ConditionTrue)) {
		return false
	}

	// Ignore everything else
	return true
}

func (r *MetricReconciler) evaluateMetrics(ctx context.Context, t *redskyv1beta1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// TODO This check precludes manual additions of Values
	if len(t.Spec.Values) > 0 {
		return nil, nil
	}

	// Get the experiment
	exp := &redskyv1beta1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Evaluate the metrics
	for _, m := range exp.Spec.Metrics {
		t.Spec.Values = append(t.Spec.Values, redskyv1beta1.Value{
			Name:              m.Name,
			AttemptsRemaining: 3,
		})
	}

	// Update the status to indicate that we will be collecting metrics
	if len(t.Spec.Values) > 0 {
		trial.ApplyCondition(&t.Status, redskyv1beta1.TrialObserved, corev1.ConditionUnknown, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func (r *MetricReconciler) collectMetrics(ctx context.Context, t *redskyv1beta1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Fetch the experiment
	exp := &redskyv1beta1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Index a DEEP COPY of the metric definitions so we can safely make changes
	metrics := make(map[string]*redskyv1beta1.Metric, len(exp.Spec.Metrics))
	for i := range exp.Spec.Metrics {
		metrics[exp.Spec.Metrics[i].Name] = exp.Spec.Metrics[i].DeepCopy()
	}

	// Iterate over the metric values, looking for remaining attempts
	log := r.Log.WithValues("trial", fmt.Sprintf("%s/%s", t.Namespace, t.Name))
	for i := range t.Spec.Values {
		v := &t.Spec.Values[i]
		if v.AttemptsRemaining == 0 {
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

		// Success, record the value and mark it as collected
		v.AttemptsRemaining = 0
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
				trial.ApplyCondition(&t.Status, redskyv1beta1.TrialFailed, corev1.ConditionTrue, "MetricBound", err.Error(), probeTime)
				err := r.Update(ctx, t)
				return controller.RequeueConflict(err)
			}
		}
	}

	// We made it through all of the metrics without needing additional changes
	trial.ApplyCondition(&t.Status, redskyv1beta1.TrialObserved, corev1.ConditionTrue, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// collectionAttempt updates the status of the trial based on the outcome of an attempt to collect metric values.
func (r *MetricReconciler) collectionAttempt(ctx context.Context, log logr.Logger, t *redskyv1beta1.Trial, v *redskyv1beta1.Value, probeTime *metav1.Time, err error) (*ctrl.Result, error) {
	// Do not count retries against the remaining attempts
	if merr, ok := err.(*metric.CaptureError); ok && merr.RetryAfter > 0 {
		return &ctrl.Result{RequeueAfter: merr.RetryAfter}, nil
	}

	// Update the probe time and ensure that trial observed is still explicitly false (i.e. we have started observation but it is not complete)
	trial.ApplyCondition(&t.Status, redskyv1beta1.TrialObserved, corev1.ConditionFalse, "", "", probeTime)

	// If there was only one attempt remaining, mark the trial as failed
	if v.AttemptsRemaining == 1 {
		trial.ApplyCondition(&t.Status, redskyv1beta1.TrialFailed, corev1.ConditionTrue, "MetricFailed", err.Error(), probeTime)
		v.AttemptsRemaining = 0

		// Metric errors contain additional information which should be logged for debugging
		if merr, ok := err.(*metric.CaptureError); ok {
			log.Error(merr, "Metric collection failed", "address", merr.Address, "query", merr.Query, "completionTime", merr.CompletionTime)
		}
	} else if v.AttemptsRemaining > 0 {
		// Decrement the attempts remaining
		v.AttemptsRemaining--
	}

	// Record the update
	return controller.RequeueConflict(r.Update(ctx, t))
}

// target looks up the Kubernetes object (if any) associated with a metric.
func (r *MetricReconciler) target(ctx context.Context, t *redskyv1beta1.Trial, m *redskyv1beta1.Metric) (runtime.Object, error) {
	if m.Type != redskyv1beta1.MetricKubernetes && m.Type != "" {
		return nil, nil
	}

	// Get the target reference, default to the trial itself if there is no reference
	targetRef := m.Target
	if m.Target == nil || m.Target.Kind == "" {
		return t, nil
	}

	// If no explicit namespace is specified, use the trial namespace
	if targetRef.Namespace == "" {
		targetRef.Namespace = t.Namespace
	}

	// If a name is specified, just get a single object
	if targetRef.Name != "" {
		target := &unstructured.Unstructured{}
		target.SetGroupVersionKind(targetRef.GroupVersionKind())
		if err := r.Get(ctx, targetRef.NamespacedName(), target); err != nil {
			return nil, err
		}
		return target, nil
	}

	// Convert the selector from a Kubernetes object to something the client can use
	sel, err := meta.MatchingSelector(targetRef.LabelSelector)
	if err != nil {
		return nil, err
	}

	// Fetch the list of matching resources
	target := &unstructured.UnstructuredList{}
	target.SetGroupVersionKind(targetRef.GroupVersionKind())
	if err := r.List(ctx, target, client.InNamespace(targetRef.Namespace), sel); err != nil {
		return nil, err
	}
	return target, nil
}

// applyMetricDefaults fills in default values for the supplied metric.
func (r *MetricReconciler) applyMetricDefaults(ctx context.Context, t *redskyv1beta1.Trial, m *redskyv1beta1.Metric) error {
	// Give Prometheus metrics a default URL
	if m.Type == redskyv1beta1.MetricPrometheus && m.URL == "" {
		m.URL = fmt.Sprintf("http://redsky-%s-prometheus:9090/", t.Namespace)
	}

	// Default to the trial namespace
	if m.Target != nil && m.Target.Namespace == "" {
		// TODO How do we exclude non-namespaced kinds from this default?
		m.Target.Namespace = t.Namespace
	}

	// This is strictly for converted v1alpha1 experiments; we should remove it eventually
	return r.resolveLegacyURL(ctx, t, m)
}

// resolveLegacyURL checks for the legacy hostname placeholder and replaces it with a hostname determined by
// looking up a Kubernetes Service object. This roughly corresponds to the original behavior of the controller
// where URL based metrics were defined using Service selectors instead of actual URLs.
func (r *MetricReconciler) resolveLegacyURL(ctx context.Context, t *redskyv1beta1.Trial, m *redskyv1beta1.Metric) error {
	if m.Type != redskyv1beta1.MetricPrometheus && m.Type != redskyv1beta1.MetricJSONPath {
		return nil
	}

	// Look for the special placeholder hostname that indicates we should look up a service
	u, err := url.Parse(m.URL)
	if err != nil || u.Hostname() != redskyv1alpha1.LegacyHostnamePlaceholder {
		return err
	}

	// Default the target
	if m.Target == nil {
		m.Target = &redskyv1beta1.ResourceTarget{}
	}

	// Convert the selector
	if m.Target.LabelSelector == nil && m.Type == redskyv1beta1.MetricPrometheus {
		m.Target.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "prometheus"}}
	}
	sel, err := meta.MatchingSelector(m.Target.LabelSelector)
	if err != nil {
		return err
	}

	// Fetch the service
	namespace := m.Target.Namespace
	if namespace == "" {
		namespace = t.Namespace
	}
	list := &corev1.ServiceList{}
	if err := r.List(ctx, list, client.InNamespace(namespace), sel); err != nil {
		return err
	}

	// Mimic legacy behavior to reconstruct host names
	// NOTE: We are not doing port name resolution or matching multiple sockets: if you
	// were relying on that behavior, you must migrate to a newer schema with explicit URLs.
	if len(list.Items) > 0 {
		host := list.Items[0].Spec.ClusterIP
		if host == "None" {
			host = fmt.Sprintf("%s.%s", list.Items[0].Name, list.Items[0].Namespace)
		}
		if port := u.Port(); port != "" {
			host += ":" + port
		}
		u.Host = host
		m.URL = u.String()
	}

	return nil
}
