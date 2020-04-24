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
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/internal/controller"
	"github.com/redskyops/redskyops-controller/internal/meta"
	"github.com/redskyops/redskyops-controller/internal/metric"
	"github.com/redskyops/redskyops-controller/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	t := &redskyv1alpha1.Trial{}
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
		For(&redskyv1alpha1.Trial{}).
		Complete(r)
}

func (r *MetricReconciler) ignoreTrial(t *redskyv1alpha1.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue) {
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
	if !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue) {
		return false
	}

	// Ignore everything else
	return true
}

func (r *MetricReconciler) evaluateMetrics(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// TODO This check precludes manual additions of Values
	if len(t.Spec.Values) > 0 {
		return nil, nil
	}

	// Get the experiment
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Evaluate the metrics
	for _, m := range exp.Spec.Metrics {
		t.Spec.Values = append(t.Spec.Values, redskyv1alpha1.Value{
			Name:              m.Name,
			AttemptsRemaining: 3,
		})
	}

	// Update the status to indicate that we will be collecting metrics
	if len(t.Spec.Values) > 0 {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionUnknown, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func (r *MetricReconciler) collectMetrics(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Index the metric definitions
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}
	metrics := make(map[string]*redskyv1alpha1.Metric, len(exp.Spec.Metrics))
	for i := range exp.Spec.Metrics {
		metrics[exp.Spec.Metrics[i].Name] = &exp.Spec.Metrics[i]
	}

	// Iterate over the metric values, looking for remaining attempts
	log := r.Log.WithValues("trial", fmt.Sprintf("%s/%s", t.Namespace, t.Name))
	for i := range t.Spec.Values {
		v := &t.Spec.Values[i]
		if v.AttemptsRemaining == 0 {
			continue
		}

		// Capture the metric
		var captureError error
		if target, err := r.target(ctx, t.Namespace, metrics[v.Name]); err != nil {
			captureError = err
		} else if value, stddev, err := metric.CaptureMetric(metrics[v.Name], t, target); err != nil {
			if merr, ok := err.(*metric.CaptureError); ok && merr.RetryAfter > 0 {
				// Do not count retries against the remaining attempts
				return &ctrl.Result{RequeueAfter: merr.RetryAfter}, nil
			}
			captureError = err
		} else {
			v.AttemptsRemaining = 0
			v.Value = strconv.FormatFloat(value, 'f', -1, 64)
			if stddev != 0 {
				v.Error = strconv.FormatFloat(stddev, 'f', -1, 64)
			}
		}

		// Handle any errors the occurred while collecting the value
		if captureError != nil && v.AttemptsRemaining > 0 {
			v.AttemptsRemaining = v.AttemptsRemaining - 1
			if v.AttemptsRemaining == 0 {
				trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "MetricFailed", captureError.Error(), probeTime)
				if merr, ok := captureError.(*metric.CaptureError); ok {
					// Metric errors contain additional information which should be logged for debugging
					log.Error(merr, "Metric collection failed", "address", merr.Address, "query", merr.Query, "completionTime", merr.CompletionTime)
				}
			}
		}

		// We have started collecting metrics (success or fail), transition into a false status
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionFalse, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// We made it through all of the metrics without needing additional changes
	trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

func (r *MetricReconciler) target(ctx context.Context, namespace string, m *redskyv1alpha1.Metric) (runtime.Object, error) {
	switch m.Type {
	case redskyv1alpha1.MetricPods:
		// Use the selector to get a list of pods
		target := &corev1.PodList{}
		if sel, err := meta.MatchingSelector(m.Selector); err != nil {
			return nil, err
		} else if err := r.List(ctx, target, client.InNamespace(namespace), sel); err != nil {
			return nil, err
		}
		return target, nil
	case redskyv1alpha1.MetricPrometheus, redskyv1alpha1.MetricJSONPath:
		// Both Prometheus and JSONPath target a service
		target := &corev1.ServiceList{}
		if sel, err := meta.MatchingSelector(m.Selector); err != nil {
			return nil, err
		} else if err := r.List(ctx, target, client.InNamespace(namespace), sel); err != nil {
			return nil, err
		}
		return target, nil
	default:
		// Assume no target is necessary
		return nil, nil
	}
}
