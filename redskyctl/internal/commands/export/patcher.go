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

package export

import (
	"encoding/json"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createPatch(patchOp *redsky.PatchOperation, targetObj []byte) (patchData []byte, err error) {
	pOM := &metav1.PartialObjectMetadata{}
	if err = json.Unmarshal(targetObj, pOM); err != nil {
		return patchData, err
	}

	// This seems super janky and im not sure it's the best way
	// This takes a runtime.Object as []byte and generates enough of a stub
	// object to render the patch on top of base gvk object.
	// This is necessary so we can create a properly structured patch object
	// to apply via kustomize(?)
	obj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, targetObj)
	if err != nil {
		return patchData, err
	}

	target, err := scheme.Scheme.New(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return patchData, err
	}

	spec := make(map[string]interface{})
	if err = json.Unmarshal(patchOp.Data, &spec); err != nil {
		return patchData, err
	}

	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(spec)
	u.SetName(patchOp.TargetRef.Name)
	u.SetNamespace(patchOp.TargetRef.Namespace)
	u.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	patchData, err = client.MergeFrom(target).Data(u)
	if err != nil {
		return patchData, err
	}

	return patchData, err
}
