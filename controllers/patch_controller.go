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

	"github.com/go-logr/logr"
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/template"
	"github.com/redskyops/k8s-experiment/internal/trial"
	"github.com/redskyops/k8s-experiment/internal/validation"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
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

func (r *PatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// TODO Update RBAC
// get,list,watch,update trials
// get,list,watch experiments

func (r *PatchReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	now := metav1.Now()

	// Fetch the Trial instance
	t := &redskyv1alpha1.Trial{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// If the trial is already finished or deleted there is nothing for us to do
	if trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Patches can be evaluated while the initializer is still present
	if result, err := r.evaluatePatches(ctx, t, &now); result != nil {
		return *result, err
	}

	// Check the "initializer" annotation, do not progress unless it is empty (don't requeue, wait for a change)
	if t.HasInitializer() {
		return ctrl.Result{}, nil
	}

	// Apply the patches, one at a time, until they all have no remaining attempts
	if result, err := r.applyPatches(ctx, t, &now); result != nil {
		return *result, err
	}

	// Finish up any status updates
	if result, err := r.finish(ctx, t, &now); result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// evaluatePatches will render the patch templates from the experiment using the trial assignments to create "patch operations" on the trial
func (r *PatchReconciler) evaluatePatches(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// TODO This check precludes manual additions of PatchOperations, but getting the experiment every time seems excessive
	if len(t.Spec.PatchOperations) > 0 {
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

	// Skip updating status if there are no patches on the experiment
	if len(exp.Spec.Patches) == 0 {
		return nil, nil
	}

	// Evaluate the patches
	te := template.New()
	for _, p := range exp.Spec.Patches {
		data, err := te.RenderPatch(&p, t)
		if err != nil {
			return &ctrl.Result{}, err
		}

		po, err := createPatchOperation(&p, data)
		if err != nil {
			return &ctrl.Result{}, err
		}

		// Fix the namespace
		if po.TargetRef.Namespace == "" {
			po.TargetRef.Namespace = t.TargetNamespace()
		}

		t.Spec.PatchOperations = append(t.Spec.PatchOperations, *po)
	}

	// Update the status to indicate that we will be patching
	trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionUnknown, "", "", probeTime)
	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// applyPatches will actually patch the objects from the patch operations
func (r *PatchReconciler) applyPatches(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// Iterate over the patches, looking for remaining attempts
	for i := range t.Spec.PatchOperations {
		p := &t.Spec.PatchOperations[i]
		if p.AttemptsRemaining == 0 {
			continue
		}

		// Construct a patch on an unstructured object
		// RBAC: We assume that we have "patch" permission from a customer defined role so we do not limit what types we can patch
		u := unstructured.Unstructured{}
		u.SetName(p.TargetRef.Name)
		u.SetNamespace(p.TargetRef.Namespace)
		u.SetGroupVersionKind(p.TargetRef.GroupVersionKind())
		if err := r.Patch(ctx, &u, client.ConstantPatch(p.PatchType, p.Data)); err != nil {
			p.AttemptsRemaining = p.AttemptsRemaining - 1
			if p.AttemptsRemaining == 0 {
				// There are no remaining patch attempts remaining, fail the trial
				trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "PatchFailed", err.Error(), probeTime)
			}
		} else {
			p.AttemptsRemaining = 0
			if p.Wait {
				// We successfully applied a patch that requires a wait, use an unknown status until we actually start waiting
				trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialStable, corev1.ConditionUnknown, "", "", probeTime)
			}
		}

		// We have started applying patches (success or fail), transition into a false status
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionFalse, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

// finish will update the trial status to indicate we are finished patching
func (r *PatchReconciler) finish(ctx context.Context, t *redskyv1alpha1.Trial, probeTime *metav1.Time) (*ctrl.Result, error) {
	// This will not add the patched condition if it is not there (e.g. the experiment had no patches)
	if cc, ok := trial.CheckCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue); ok && !cc {
		trial.ApplyCondition(&t.Status, redskyv1alpha1.TrialPatched, corev1.ConditionTrue, "", "", probeTime)
		err := r.Update(ctx, t)
		return controller.RequeueConflict(err)
	}

	return nil, nil
}

func createPatchOperation(p *redskyv1alpha1.PatchTemplate, data []byte) (*redskyv1alpha1.PatchOperation, error) {
	// The default patch operation has 3 retries and triggers a stability check ("wait")
	po := &redskyv1alpha1.PatchOperation{
		Data:              data,
		AttemptsRemaining: 3,
		Wait:              true,
	}

	// If the patch is effectively null, we do not need to evaluate it
	if len(po.Data) == 0 || string(po.Data) == "null" {
		po.AttemptsRemaining = 0
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

	// Attempt to populate the target reference
	if p.TargetRef != nil {
		p.TargetRef.DeepCopyInto(&po.TargetRef)
	}

	// TODO Allow strategic merge patches to specify the target reference (only if p.TargetRef == nil ?)
	// Need to unmarshal data into an ObjectMeta and extract the results into the reference, checking for conflicts

	return po, nil
}
