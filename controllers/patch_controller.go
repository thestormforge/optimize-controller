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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/internal/controller"
	"github.com/redskyops/redskyops-controller/internal/ready"
	"github.com/redskyops/redskyops-controller/internal/template"
	"github.com/redskyops/redskyops-controller/internal/trial"
	"github.com/redskyops/redskyops-controller/internal/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchReconciler reconciles the patches on a Trial object
type PatchReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redskyops.dev,resources=experiments,verbs=get;list;watch
// +kubebuilder:rbac:groups=redskyops.dev,resources=trials,verbs=get;list;watch;update

// Reconcile inspects a trial to see if patches need to be applied. The "trial patched" status condition
// is used to control what actions need to be taken. If the status is "unknown" then the experiment is fetched
// and the patch templates will be rendered into the list of patch operations on the trial; once the patches
// are evaluated the status will be "false". If the status is "false" then patch operations will be applied
// to the cluster; once all the patches are applied the status will be "true".
func (r *PatchReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil || r.ignoreTrial(t) {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	if result, err := r.evaluatePatchOperations(ctx, t, &now); result != nil {
		return *result, err
	}

	if result, err := r.applyPatches(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers a new patch reconciler with the supplied manager
func (r *PatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("patch").
		For(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// ignoreTrial determines which trial objects can be ignored by this reconciler
func (r *PatchReconciler) ignoreTrial(t *redskyv1alpha1.Trial) bool {
	// Ignore deleted trials
	if !t.DeletionTimestamp.IsZero() {
		return true
	}

	// Ignore failed trials
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue) {
		return true
	}

	// Ignore uninitialized trials
	if t.HasInitializer() {
		return true
	}

	// Ignore trials that have setup tasks which haven't run yet
	// TODO This is to solve a specific race condition with establishing an initializer, is there a better check?
	if len(t.Spec.SetupTasks) > 0 && !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialSetupCreated, corev1.ConditionTrue) {
		return true
	}

	// Ignore patched trials
	if trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue) {
		return true
	}

	// Reconcile everything else
	return false
}

// evaluatePatchOperations will render the patch templates from the experiment using the trial assignments to create "patch operations" on the trial
func (r *PatchReconciler) evaluatePatchOperations(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Only evaluate patches if the "patched" status is "unknown"
	if !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionUnknown) {
		return nil, nil
	}

	// Get the experiment
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, t.ExperimentNamespacedName(), exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Make sure the assignments are valid
	if err := validation.CheckAssignments(t, exp); err != nil {
		return &ctrl.Result{}, err
	}

	// Readiness checks from patches should always be applied first
	readinessChecks := t.Spec.ReadinessChecks
	t.Spec.ReadinessChecks = nil

	// Evaluate the patches
	te := template.New()
	for i := range exp.Spec.Patches {
		p := &exp.Spec.Patches[i]

		// Render the patch template
		ref, data, err := r.renderTemplate(te, t, p)
		if err != nil {
			return &ctrl.Result{}, err
		}

		// Add a patch operation if necessary
		if po, err := r.createPatchOperation(t, p, ref, data); err != nil {
			return &ctrl.Result{}, err
		} else if po != nil {
			t.Spec.PatchOperations = append(t.Spec.PatchOperations, *po)
		}

		// Add a readiness check if necessary
		if rc, err := r.createReadinessCheck(p, ref); err != nil {
			return &ctrl.Result{}, err
		} else if rc != nil {
			t.Spec.ReadinessChecks = append(t.Spec.ReadinessChecks, *rc)
		}
	}

	// Add back any pre-existing readiness checks
	t.Spec.ReadinessChecks = append(t.Spec.ReadinessChecks, readinessChecks...)

	// Update the status to indicate that patches are evaluated
	trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionFalse, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// applyPatches will actually patch the objects from the patch operations
func (r *PatchReconciler) applyPatches(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Only apply patches if the "patched" status is "false"
	if !trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionFalse) {
		return nil, nil
	}

	// Iterate over the patches, looking for remaining attempts
	for i := range t.Spec.PatchOperations {
		p := &t.Spec.PatchOperations[i]
		if p.AttemptsRemaining == 0 {
			continue
		}

		// Construct a patch on an unstructured object
		// RBAC: We assume that we have "patch" permission from a customer defined role so we do not limit what types we can patch
		u := &unstructured.Unstructured{}
		u.SetName(p.TargetRef.Name)
		u.SetNamespace(p.TargetRef.Namespace)
		u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
		if err := r.Patch(ctx, u, client.RawPatch(p.PatchType, p.Data)); err != nil {
			p.AttemptsRemaining = p.AttemptsRemaining - 1
			if p.AttemptsRemaining == 0 {
				// There are no remaining patch attempts remaining, fail the trial
				trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "PatchFailed", err.Error(), probeTime)
			}
		} else {
			p.AttemptsRemaining = 0
		}

		// Update the patch operation status
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	// We made it through all of the patches without needing additional changes
	trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// renderTemplate determines the patch target and renders the patch template
func (r *PatchReconciler) renderTemplate(te *template.Engine, t *redskyv1alpha1.Trial, p *redskyv1alpha1.PatchTemplate) (*corev1.ObjectReference, []byte, error) {
	// Render the actual patch data
	data, err := te.RenderPatch(p, t)
	if err != nil {
		return nil, nil, err
	}

	// Determine the reference, possibly extracting it from the rendered data
	ref := &corev1.ObjectReference{}
	if p.TargetRef != nil {
		p.TargetRef.DeepCopyInto(ref)
	} else if p.Type == redskyv1alpha1.PatchStrategic || p.Type == "" {
		m := &struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata,omitempty"`
		}{}
		if err := json.Unmarshal(data, m); err == nil {
			ref.APIVersion = m.APIVersion
			ref.Kind = m.Kind
			ref.Name = m.Name
			ref.Namespace = m.Namespace
		}
	}

	// Default the namespace to the trial namespace
	if ref.Namespace == "" {
		ref.Namespace = t.Namespace
	}

	// Validate the reference
	if ref.Name == "" || ref.Kind == "" {
		return nil, nil, fmt.Errorf("invalid patch reference")
	}

	return ref, data, nil
}

// createPatchOperation creates a new patch operation from a patch template and it's (fully rendered) patch data
func (r *PatchReconciler) createPatchOperation(t *redskyv1alpha1.Trial, p *redskyv1alpha1.PatchTemplate, ref *corev1.ObjectReference, data []byte) (*redskyv1alpha1.PatchOperation, error) {
	po := &redskyv1alpha1.PatchOperation{
		TargetRef:         *ref,
		Data:              data,
		AttemptsRemaining: 3,
	}

	// If the patch is effectively null, we do not need to evaluate it
	if len(po.Data) == 0 || string(po.Data) == "null" {
		return nil, nil
	}

	// Determine the patch type
	switch p.Type {
	case redskyv1alpha1.PatchStrategic, "":
		po.PatchType = types.StrategicMergePatchType
	case redskyv1alpha1.PatchMerge:
		po.PatchType = types.MergePatchType
	case redskyv1alpha1.PatchJSON:
		po.PatchType = types.JSONPatchType
	default:
		return nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// If the patch is for the trial job itself, it cannot be applied (since the job won't exist until well after patches are applied)
	if trial.IsTrialJobReference(t, &po.TargetRef) {
		po.AttemptsRemaining = 0
		if po.PatchType != types.StrategicMergePatchType {
			return nil, fmt.Errorf("trial job patch must be a strategic merge patch")
		}
	}

	return po, nil
}

// createReadinessCheck creates a readiness check for a patch operation
func (r *PatchReconciler) createReadinessCheck(p *redskyv1alpha1.PatchTemplate, ref *corev1.ObjectReference) (*redskyv1alpha1.ReadinessCheck, error) {
	// NOTE: There is a cardinality mismatch between the `PatchReadinessGate` type and the `ReadinessCheck` type in
	// regard to condition types. We purposely do not expose user facing configuration for these checks (users can
	// skip patch readiness checks and specify them manually for fine grained control).
	rc := &redskyv1alpha1.ReadinessCheck{
		TargetRef:         *ref,
		PeriodSeconds:     5,
		AttemptsRemaining: 36, // ...targeting a 3 minute max for applications to come back after a patch
	}

	// Add configured and default readiness conditions
	for i := range p.ReadinessGates {
		rc.ConditionTypes = append(rc.ConditionTypes, p.ReadinessGates[i].ConditionType)
	}

	// Check for a "legacy" patch that has no explicit (not even empty) readiness gates and apply settings consistent
	// with earlier versions of the product (we should re-visit this)
	if p.ReadinessGates == nil {
		rc.ConditionTypes = append(rc.ConditionTypes, ready.ConditionTypeAppReady)
		rc.InitialDelaySeconds = 1
	}

	// If there are no conditions to check, we do not need to add a readiness check
	if len(rc.ConditionTypes) == 0 {
		return nil, nil
	}
	return rc, nil
}
