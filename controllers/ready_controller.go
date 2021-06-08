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
	"strings"
	"time"

	"github.com/go-logr/logr"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/controller"
	"github.com/thestormforge/optimize-controller/v2/internal/ready"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadyReconciler checks for readiness of the patched objects
type ReadyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Keep the raw API reader for doing stabilization checks. In that case we only have patch/get permissions
	// on the object and if we were to use the standard caching reader we would hang because cache itself also
	// requires list/watch. If we ever get a way to disable the cache or the cache becomes smart enough to handle
	// permission errors without hanging we can go back to using standard reader.
	apiReader client.Reader
}

// +kubebuilder:rbac:groups=optimize.stormforge.io,resources=trials,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list

// Reconcile inspects a trial to see if the patched objects are ready for the trial job to start
func (r *ReadyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &optimizev1beta2.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil || r.ignoreTrial(t) {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	if result, err := r.evaluateReadinessChecks(ctx, t, &now); result != nil {
		return *result, err
	}

	if result, err := r.checkReadiness(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers a new ready reconciler with the supplied manager
func (r *ReadyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return ctrl.NewControllerManagedBy(mgr).
		Named("ready").
		For(&optimizev1beta2.Trial{}).
		Complete(r)
}

// ignoreTrial determines which trial objects can be ignored by this reconciler
func (r *ReadyReconciler) ignoreTrial(t *optimizev1beta2.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue) {
		return true
	}

	// Ignore unpatched trials
	if !trial.CheckCondition(&t.Status, optimizev1beta2.TrialPatched, corev1.ConditionTrue) {
		return true
	}

	// Ignore ready trials
	if trial.CheckCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionTrue) {
		return true
	}

	// Reconcile everything else
	return false
}

// evaluateReadinessChecks will prepare all of the readiness checks for the trial
func (r *ReadyReconciler) evaluateReadinessChecks(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Only evaluate readiness checks if the "ready" status is "unknown"
	if !trial.CheckCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionUnknown) {
		return nil, nil
	}

	// NOTE: There are probably already readiness checks that were populated during patch evaluation

	// Add readiness checks for the trial itself
	for i := range t.Spec.ReadinessGates {
		c := &t.Spec.ReadinessGates[i]
		rc := optimizev1beta2.ReadinessCheck{
			TargetRef: corev1.ObjectReference{
				Kind:       c.Kind,
				Namespace:  t.Namespace,
				Name:       c.Name,
				APIVersion: c.APIVersion,
			},
			Selector:            c.Selector,
			ConditionTypes:      c.ConditionTypes,
			InitialDelaySeconds: c.InitialDelaySeconds,
			PeriodSeconds:       c.PeriodSeconds,
			AttemptsRemaining:   c.FailureThreshold,
		}

		// Adjust for defaults/minimums
		if rc.PeriodSeconds == 0 {
			rc.PeriodSeconds = 10
		} else if rc.PeriodSeconds < 0 {
			rc.PeriodSeconds = 1
		}
		if rc.AttemptsRemaining == 0 {
			rc.AttemptsRemaining = 3
		} else if rc.AttemptsRemaining < 0 {
			rc.AttemptsRemaining = 1
		}

		t.Status.ReadinessChecks = append(t.Status.ReadinessChecks, rc)
	}

	// Update the status to indicate that readiness checks are evaluated
	trial.ApplyCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionFalse, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// checkReadiness will evaluate the readiness checks for the trial
func (r *ReadyReconciler) checkReadiness(ctx context.Context, t *optimizev1beta2.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Only check readiness checks if the "ready" status is "false"
	if !trial.CheckCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionFalse) {
		return nil, nil
	}

	// Create a new "checker" to maintain state while looping over the readiness checks
	checker := newReadinessChecker(r.Client, t)
	for i := range t.Status.ReadinessChecks {
		c := &t.Status.ReadinessChecks[i]
		if checker.skipCheck(c, probeTime) {
			continue
		}

		// Get the objects to check
		ul, err := r.getCheckTargets(ctx, c)
		if err != nil {
			readinessCheckFailed(t, probeTime, err)
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}

		// Check for readiness
		if msg, isReady, err := checker.check(ctx, c, ul, probeTime); err != nil {
			readinessCheckFailed(t, probeTime, err)
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		} else if !isReady {
			// This will get overwritten with anything that isn't ready as we progress through the loop
			trial.ApplyCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionFalse, "Waiting", msg, probeTime)
		}
	}

	// We may need to requeue and try again (e.g. all of the checks are in the initial delay)
	if checker.requeue && checker.after > 0 {
		return &ctrl.Result{RequeueAfter: checker.after}, nil
	}

	// Update the trial (and the status, if all the checks are complete)
	if checker.ready {
		trial.ApplyCondition(&t.Status, optimizev1beta2.TrialReady, corev1.ConditionTrue, "", "", probeTime)
	}
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// getCheckTargets returns the list of target objects for the readiness check
func (r *ReadyReconciler) getCheckTargets(ctx context.Context, rc *optimizev1beta2.ReadinessCheck) (*unstructured.UnstructuredList, error) {
	ul := &unstructured.UnstructuredList{}

	// If there is no kind on the target reference, we can't actually fetch anything
	if rc.TargetRef.Kind == "" {
		return ul, nil
	}

	// If there is no name on the target reference, search for matching objects instead
	if rc.TargetRef.Name == "" {
		ul.SetGroupVersionKind(rc.TargetRef.GroupVersionKind())
		s, err := metav1.LabelSelectorAsSelector(rc.Selector)
		if err != nil {
			return nil, err
		}
		err = r.apiReader.List(ctx, ul, client.InNamespace(rc.TargetRef.Namespace), client.MatchingLabelsSelector{Selector: s})
		return ul, err
	}

	// Get a single object instead
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(rc.TargetRef.GroupVersionKind())
	key := types.NamespacedName{Namespace: rc.TargetRef.Namespace, Name: rc.TargetRef.Name}
	if err := r.apiReader.Get(ctx, key, &u); err != nil {
		// "Mimic" list behavior by returning an empty list if the object is not found
		if controller.IgnoreNotFound(err) != nil {
			return nil, err
		}
	} else {
		ul.Items = append(ul.Items, u)
	}
	return ul, nil
}

// readinessCheckFailed puts a trial into a failed state due to a failed readiness check
func readinessCheckFailed(t *optimizev1beta2.Trial, probeTime *metav1.Time, err error) {
	reason, message := "ReadinessCheckFailed", err.Error()
	if rerr, ok := err.(*ready.ReadinessError); ok {
		if rerr.Reason != "" {
			reason = rerr.Reason
		}
		if rerr.Message != "" {
			message = rerr.Message
		}
	}
	trial.ApplyCondition(&t.Status, optimizev1beta2.TrialFailed, corev1.ConditionTrue, reason, message, probeTime)
}

// readinessChecker is the loop state used to evaluate readiness checks
type readinessChecker struct {
	// checker is used to evaluate the conditions of a target
	checker ready.ReadinessChecker
	// epoch is the time at which readiness checks can be evaluated (i.e. when the trial transitioned into a "patched" status)
	epoch metav1.Time
	// ready is a flag indicating that all of readiness checks have been evaluated
	ready bool
	// requeue is a flag indicating that none of the readiness checks can be evaluated
	requeue bool
	// after is the delay after which all of the readiness checks can be evaluated
	after time.Duration
}

// newReadinessChecker returns a new checker for the supplied trial
func newReadinessChecker(reader client.Reader, t *optimizev1beta2.Trial) *readinessChecker {
	checker := ready.ReadinessChecker{Reader: reader}
	epoch := t.GetCreationTimestamp()
	for i := range t.Status.Conditions {
		if t.Status.Conditions[i].Type == optimizev1beta2.TrialPatched {
			epoch = t.Status.Conditions[i].LastTransitionTime
		}
	}
	return &readinessChecker{checker: checker, epoch: epoch, ready: true, requeue: true}
}

// skipCheck determines if a check should be evaluated, recording the results internally
func (rc *readinessChecker) skipCheck(c *optimizev1beta2.ReadinessCheck, now *metav1.Time) bool {
	// Determine if the check is completed
	if c.AttemptsRemaining <= 0 {
		return true
	}

	// At least one check still has remaining attempts, DO NOT mark the trial as ready
	rc.ready = false

	// Determine if we need to wait for the check
	if next := rc.nextCheckTime(c); now.Before(next) {
		d := next.Time.Sub(now.Time)

		// Avoid excessive sleeps so we can still detect failures
		if p := time.Duration(c.PeriodSeconds) * time.Second; d > p {
			d = p
		}

		// Take the largest delay if we are considering multiple checks
		if d > rc.after {
			rc.after = d
		}

		return true
	}

	// There is no delay required, instead we should update the trial to record any changes
	rc.requeue = false

	return false
}

// check evaluates a readiness check against a (possibly nil) target, returning a status message and boolean indicating
// if the target is in fact ready
func (rc *readinessChecker) check(ctx context.Context, c *optimizev1beta2.ReadinessCheck, ul *unstructured.UnstructuredList, now *metav1.Time) (string, bool, error) {
	// Evaluate the actual conditions (stop at the first one that isn't "ready")
	var msg string
	var ok bool
	var err error
	for i := range ul.Items {
		msg, ok, err = rc.checker.CheckConditions(ctx, &ul.Items[i], c.ConditionTypes)
		if !ok || err != nil {
			break
		}
	}

	// If a check is missing it's kind, just mark it as completed (e.g. if this
	// is just a "sleep" based on the initial delay)
	if c.TargetRef.Kind == "" {
		ok = true
	}

	// Check is done, it is either ok or had a hard failure
	if ok || err != nil {
		c.AttemptsRemaining = 0
		c.LastCheckTime = nil
		return "", ok, err
	}

	// If there are no items to check, try to provide a useful message
	if len(ul.Items) == 0 {
		var missingTargetMsg strings.Builder
		missingTargetMsg.WriteString("No matching resources found")
		missingTargetMsg.WriteString("; apiVersion=")
		missingTargetMsg.WriteString(c.TargetRef.APIVersion)
		missingTargetMsg.WriteString("; kind=")
		missingTargetMsg.WriteString(c.TargetRef.Kind)
		if c.TargetRef.Name != "" {
			missingTargetMsg.WriteString("; name=")
			missingTargetMsg.WriteString(c.TargetRef.Name)
		}
		if c.Selector != nil {
			missingTargetMsg.WriteString("; labels=")
			missingTargetMsg.WriteString(metav1.FormatLabelSelector(c.Selector))
		}
		msg = missingTargetMsg.String()
	}

	// Check if we exceeded the failure threshold
	c.AttemptsRemaining--
	if c.AttemptsRemaining <= 0 {
		return "", false, &ready.ReadinessError{Reason: "ReadinessFailureThreshold", Message: msg}
	}

	// Record the fact that we need to re-check
	c.LastCheckTime = now
	return msg, false, nil
}

// nextCheckTime returns the approximate time that an attempt should be made to evaluate a check
func (rc *readinessChecker) nextCheckTime(c *optimizev1beta2.ReadinessCheck) *metav1.Time {
	if c.LastCheckTime != nil {
		periodSeconds := c.PeriodSeconds
		if periodSeconds < 1 {
			periodSeconds = 1
		}
		return &metav1.Time{Time: c.LastCheckTime.Add(time.Duration(periodSeconds) * time.Second)}
	}

	return &metav1.Time{Time: rc.epoch.Add(time.Duration(c.InitialDelaySeconds) * time.Second)}
}
