package export

import (
	"encoding/json"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createPatch(patchOp *redsky.PatchOperation) (patchData []byte, err error) {
	switch patchOp.PatchType {
	case types.JSONPatchType:
		// Can use kustomize here
	case types.MergePatchType:
	// TODO ?
	case types.StrategicMergePatchType:
		// Can use kustomize here
	case types.ApplyPatchType:
		// TODO ?
	}

	// TODO: for offline testing
	output := pgDeployment
	pOM := &metav1.PartialObjectMetadata{}
	if err = json.Unmarshal(output, pOM); err != nil {
		return patchData, err
	}

	// This seems super janky and im not sure it's the best way
	// This takes a runtime.Object as []byte and generates enough of a stub
	// object to render the patch on top of base gvk object.
	// This is necessary so we can create a properly structured patch object
	// to apply via kustomize(?)
	obj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, output)
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

func createStrategicMergePatch(po *redsky.PatchOperation) {

}

func createMergePatch(po *redsky.PatchOperation) {

}
