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

package scan

import (
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1alpha1 "github.com/thestormforge/optimize-controller/api/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Helpers to facilitate conversion between Kube API Machinery runtime objects
// and KYAML resource nodes. The ObjectSlice allows a slice of object to be
// read as a kio.Reader; the ObjectList allows a generic List to be written as
// a kio.Writer.

// Scheme is used globally to ensure the runtime helpers produce and consume
// RNodes with type metadata (kind and apiVersion) and Go typing. In general,
// there isn't a convenient way to parameterize the Read/Write calls (i.e. no
// Context), thus the global variable. By default, the scheme is loaded with the
// known Kubernetes API types and the types from this project (e.g Experiments).
var Scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = redskyv1alpha1.AddToScheme(Scheme)
	_ = redskyv1beta1.AddToScheme(Scheme)
	_ = redskyappsv1alpha1.AddToScheme(Scheme)
}

// ObjectSlices allows a slice of object instances to be read as resource nodes.
type ObjectSlice []runtime.Object

var _ kio.Reader = ObjectSlice{}

// Read converts the objects to JSON and then to YAML RNodes.
func (os ObjectSlice) Read() ([]*yaml.RNode, error) {
	var result []*yaml.RNode
	for _, o := range os {
		u := &unstructured.Unstructured{}
		if err := Scheme.Convert(o, u, runtime.InternalGroupVersioner); err != nil {
			return nil, err
		}

		data, err := json.Marshal(u)
		if err != nil {
			return nil, err
		}

		node, err := yaml.ConvertJSONToYamlNode(string(data))
		if err != nil {
			return nil, err
		}

		if err := node.PipeE(yaml.FilterFunc(clearCreationTimestamp)); err != nil {
			return nil, err
		}

		result = append(result, node)
	}

	return result, nil
}

// ObjectList allows a generic list type to be used as a KYAML writer to provide
// interoperability between the YAML streaming and runtime objects.
type ObjectList corev1.List

var _ kio.Writer = &ObjectList{}

// Write converts the resource nodes into runtime objects.
func (o *ObjectList) Write(nodes []*yaml.RNode) error {
	for _, node := range nodes {
		data, err := node.MarshalJSON()
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(data); err != nil {
			return err
		}

		obj := newObjectForKind(u)
		if err := Scheme.Convert(u, obj, runtime.InternalGroupVersioner); err != nil {
			return err
		}

		o.Items = append(o.Items, runtime.RawExtension{Object: obj})
	}

	return nil
}

// clearCreationTimestamp is a dumb problem to have. Creation times are always
// serialized as JSON "null" (the Go JSON encoder does not have "omitempty" for
// generic structs); they should have been specified as a pointer to avoid this
// problem. But they weren't.
func clearCreationTimestamp(node *yaml.RNode) (*yaml.RNode, error) {
	var err error
	switch node.YNode().Kind {
	case yaml.MappingNode:
		if removed, err := node.Pipe(yaml.Clear("creationTimestamp")); err != nil || removed != nil {
			return node, err
		}

		err = node.VisitFields(func(node *yaml.MapNode) error {
			return node.Value.PipeE(yaml.FilterFunc(clearCreationTimestamp))
		})
	case yaml.SequenceNode:
		err = node.VisitElements(func(node *yaml.RNode) error {
			return node.PipeE(yaml.FilterFunc(clearCreationTimestamp))
		})
	}

	return node, err
}

// newObjectForKind uses the scheme to create a new typed object of the kind
// of the supplied object. If a new typed object cannot be created, the supplied
// object is returned.
func newObjectForKind(obj runtime.Object) runtime.Object {
	if gvks, _, _ := Scheme.ObjectKinds(obj); len(gvks) > 0 {
		if typed, err := Scheme.New(gvks[0]); err == nil {
			return typed
		}
	}

	return obj
}
