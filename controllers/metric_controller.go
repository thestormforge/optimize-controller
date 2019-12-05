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
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/metric"
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
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

	// Fetch the Trial instance
	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// If the trial is already finished or deleted or not yet complete, there is nothing for us to do
	if trial.IsFinished(t) || t.Status.CompletionTime == nil || !t.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Get the metric definitions from the experiment and collect the values
	if result, err := r.collectMetrics(ctx, t, &now); result != nil {
		return *result, err
	}

	// Update the trial status, this will mark the trial as finished
	if result, err := r.finish(ctx, t, &now); result != nil {
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

func (r *MetricReconciler) collectMetrics(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Fetch the experiment
	e := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), e); err != nil {
		return &ctrl.Result{}, err
	}
	if len(e.Spec.Metrics) == 0 {
		return nil, nil
	}

	// Make sure we have a trial observed status
	if _, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionUnknown); !ok {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionUnknown, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// Look for metrics that have not been collected yet
	log := r.Log.WithValues("trial", fmt.Sprintf("%s/%s", t.Namespace, t.Name))
	for _, m := range e.Spec.Metrics {
		v := findOrCreateValue(t, m.Name)
		if v.AttemptsRemaining == 0 {
			continue
		}

		// Capture the metric
		var captureError error
		if target, err := r.target(ctx, t.TargetNamespace(), &m); err != nil {
			captureError = err
		} else if value, stddev, err := metric.CaptureMetric(&m, t, target); err != nil {
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

		// Set the observed condition to false since we have observed at least one, but possibly not all of, the metrics
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionFalse, "", "", probeTime)

		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func (r *MetricReconciler) finish(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Only update (do not create) the observed condition
	if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue); ok && !cc {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialObserved, corev1.ConditionTrue, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// Mark the trial as completed
	// TODO This should be part of the trial controller
	if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialComplete, corev1.ConditionTrue); !ok || !cc {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialComplete, corev1.ConditionTrue, "", "", nil)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
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
		// NOTE: This purposely ignores the namespace in case Prometheus is running cluster wide
		target := &corev1.ServiceList{}
		if sel, err := meta.MatchingSelector(m.Selector); err != nil {
			return nil, err
		} else if err := r.List(ctx, target, sel); err != nil {
			return nil, err
		}
		return target, nil
	default:
		// Assume no target is necessary
		return nil, nil
	}
}

func findOrCreateValue(trial *redskyv1alpha1.Trial, name string) *redskyv1alpha1.Value {
	for i := range trial.Spec.Values {
		if trial.Spec.Values[i].Name == name {
			return &trial.Spec.Values[i]
		}
	}

	trial.Spec.Values = append(trial.Spec.Values, redskyv1alpha1.Value{Name: name, AttemptsRemaining: 3})
	return &trial.Spec.Values[len(trial.Spec.Values)-1]
}
