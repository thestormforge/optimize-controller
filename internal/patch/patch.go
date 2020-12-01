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

	redsky "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/template"
	"github.com/thestormforge/optimize-controller/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const defaultAttemptsRemaining = 3

// RenderTemplate determines the patch target and renders the patch template
func RenderTemplate(te *template.Engine, t *redsky.Trial, p *redsky.PatchTemplate) (*corev1.ObjectReference, []byte, error) {
	// Render the actual patch data
	data, err := te.RenderPatch(p, t)
	if err != nil {
		return nil, nil, err
	}

	// Determine the reference, possibly extracting it from the rendered data
	ref := &corev1.ObjectReference{}
	if p.TargetRef != nil {
		p.TargetRef.DeepCopyInto(ref)
	} else if p.Type == redsky.PatchStrategic || p.Type == "" {
		m := &metav1.PartialObjectMetadata{}
		if err := json.Unmarshal(data, m); err != nil {
			return nil, nil, err
		}
		ref.APIVersion = m.APIVersion
		ref.Kind = m.Kind
		ref.Name = m.Name
		ref.Namespace = m.Namespace
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
func CreatePatchOperation(t *redsky.Trial, p *redsky.PatchTemplate, ref *corev1.ObjectReference, data []byte) (*redsky.PatchOperation, error) {
	// If the patch is effectively null, we do not need to evaluate it
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	po := &redsky.PatchOperation{
		TargetRef:         *ref,
		Data:              data,
		AttemptsRemaining: defaultAttemptsRemaining,
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

	// If the patch is for the trial job itself, it cannot be applied (since the job won't exist until well after patches are applied)
	if trial.IsTrialJobReference(t, &po.TargetRef) {
		po.AttemptsRemaining = 0
		if po.PatchType != types.StrategicMergePatchType {
			return nil, fmt.Errorf("trial job patch must be a strategic merge patch")
		}
	}

	return po, nil
}
