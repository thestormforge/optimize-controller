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

package patch

import (
	"encoding/json"
	"fmt"
	"log"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/template"
	"github.com/redskyops/redskyops-controller/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Patcher struct {
	Engine *template.Engine
}

func NewPatcher() *Patcher {
	return &Patcher{
		Engine: template.New(),
	}
}

func (p *Patcher) CreatePatchOperation(t *redsky.Trial, pt *redsky.PatchTemplate) (patchOp *redsky.PatchOperation, err error) {
	// Determine the patch type
	var patchType types.PatchType
	switch pt.Type {
	case redsky.PatchStrategic, "":
		patchType = types.StrategicMergePatchType
	case redsky.PatchMerge:
		patchType = types.MergePatchType
	case redsky.PatchJSON:
		patchType = types.JSONPatchType
	default:
		return nil, fmt.Errorf("unknown patch type: %s", pt.Type)
	}

	patchBytes, err := p.Engine.RenderPatch(pt, t)
	if err != nil {
		return nil, err
	}

	ptMeta := &metav1.PartialObjectMetadata{}
	if err = json.Unmarshal(patchBytes, ptMeta); err != nil {
		return nil, err
	}

	// Default the namespace to the trial namespace
	if ptMeta.Namespace == "" {
		ptMeta.Namespace = t.Namespace
	}

	objRef, err := getObjectRefFromPatchTemplate(ptMeta, pt)
	if err != nil {
		return nil, err
	}

	patchOp = &redsky.PatchOperation{
		TargetRef:         *objRef,
		Data:              patchBytes,
		AttemptsRemaining: 3,
		PatchType:         patchType,
	}

	return patchOp, nil
}

// TODO: Remove as things migrate to Patcher
// RenderTemplate determines the patch target and renders the patch template
func RenderTemplate(te *template.Engine, t *redsky.Trial, p *redsky.PatchTemplate) (*corev1.ObjectReference, []byte, error) {
	// Render the actual patch data
	data, err := te.RenderPatch(p, t)
	if err != nil {
		return nil, nil, err
	}

	// Determine the reference, possibly extracting it from the rendered data
	ref := &corev1.ObjectReference{}
	switch {
	case p.TargetRef != nil:
		log.Println("targetref = nil")
		p.TargetRef.DeepCopyInto(ref)
	case p.Type == redsky.PatchStrategic, p.Type == "":
		log.Println("patch type")
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

// TODO: Remove as things migrate to Patcher
// createPatchOperation creates a new patch operation from a patch template and it's (fully rendered) patch data
func CreatePatchOperation(t *redsky.Trial, p *redsky.PatchTemplate, ref *corev1.ObjectReference, data []byte) (*redsky.PatchOperation, error) {
	po := &redsky.PatchOperation{
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
	case redsky.PatchStrategic, "":
		po.PatchType = types.StrategicMergePatchType
	case redsky.PatchMerge:
		po.PatchType = types.MergePatchType
	case redsky.PatchJSON:
		po.PatchType = types.JSONPatchType
	default:
		return nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// TODO: probably should move this to the controller
	// If the patch is for the trial job itself, it cannot be applied (since the job won't exist until well after patches are applied)
	if trial.IsTrialJobReference(t, &po.TargetRef) {
		po.AttemptsRemaining = 0
		if po.PatchType != types.StrategicMergePatchType {
			return nil, fmt.Errorf("trial job patch must be a strategic merge patch")
		}
	}

	return po, nil
}

// getObjectRefFromPatchTemplate constructs an corev1.ObjectReference from a patch template.
func getObjectRefFromPatchTemplate(ptMeta *metav1.PartialObjectMetadata, pt *redsky.PatchTemplate) (ref *corev1.ObjectReference, err error) {
	// Determine the reference, possibly extracting it from the rendered data
	ref = &corev1.ObjectReference{}

	switch {
	case pt.TargetRef != nil:
		pt.TargetRef.DeepCopyInto(ref)
	case pt.Type == redsky.PatchStrategic, pt.Type == "":
		ref.APIVersion = ptMeta.APIVersion
		ref.Kind = ptMeta.Kind
		ref.Name = ptMeta.Name
	default:
		return nil, fmt.Errorf("invalid patch reference")
	}

	if ref.Namespace == "" {
		ref.Namespace = ptMeta.Namespace
	}

	// Validate the reference
	if ref.Name == "" || ref.Kind == "" {
		return nil, fmt.Errorf("invalid patch reference")
	}

	return ref, nil
}
