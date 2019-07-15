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
package patch

import (
	"context"
	"fmt"
	"net/http"

	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var log = logf.Log.WithName("webhook")

func Add(mgr manager.Manager) error {
	return add(mgr, "/trial-patches", newHandler(mgr))
}

func newHandler(mgr manager.Manager) admission.Handler {
	return &TrialHandler{Client: mgr.GetClient()}
}

func add(mgr manager.Manager, p string, h admission.Handler) error {
	//mgr.GetWebhookServer().Register(p, &webhook.Admission{Handler: h})
	return nil
}

var _ admission.Handler = &TrialHandler{}

type TrialHandler struct {
	client.Client
}

// kubebuilder:webhook:failurePolicy=fail,groups=apps,resources=deployments;statefulsets,verbs=create,versions=v1,name=trial-patches.carbonrelay.com,path=/trial-patches,mutating=true

func (h *TrialHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	// We are only interested in patching create operations
	if request.Operation == v1beta1.Create {
		list := &v1alpha1.TrialList{}
		if err := h.List(ctx, list); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, t := range list.Items {
			// TODO If patch operations is empty, we need to compute it here instead of in the controller (requires refactoring common code)
			for _, p := range t.Spec.PatchOperations {
				if p.AttemptsRemaining > 0 && matches(&p, &request) {
					if response, err := allowed(&p); err != nil {
						log.Error(err, "Ignoring failed patch conversion")
					} else {
						return response
					}
				}
			}
		}
	}

	return admission.Allowed("")
}

func matches(operation *v1alpha1.PatchOperation, request *admission.Request) bool {
	gvk := operation.TargetRef.GroupVersionKind()
	return gvk.Group == request.Kind.Group &&
		gvk.Version == request.Kind.Version &&
		gvk.Kind == request.Kind.Kind &&
		operation.TargetRef.Namespace == request.Namespace &&
		operation.TargetRef.Name == request.Name
}

func allowed(p *v1alpha1.PatchOperation) (admission.Response, error) {
	// Convert from the standard patch type to the admission patch type
	var pt v1beta1.PatchType
	switch p.PatchType {
	case types.JSONPatchType:
		pt = v1beta1.PatchTypeJSONPatch
	default:
		// Currently only JSONPatch is supported
		return admission.Response{}, fmt.Errorf("unsupported patch type: %s", p.PatchType)
	}

	response := admission.Allowed("")
	response.PatchType = &pt
	response.Patch = p.Data
	return response, nil
}
