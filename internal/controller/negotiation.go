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

package controller

import (
	"fmt"
	"reflect"

	"github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

// MEGA HACK ALERT!!!

// This code exists to force client side conversion between versions to avoid relying on webhook based server side
// conversions (alpha in Kube 1.13). Instead of properly conversion the resources in storage, we look at the
// representation provided by the API server and try to detect if we should override the `apiVersion`, thereby
// initiating conversion.

// WithConversion returns the supplied rest.Config with the negotiated serializer set so version conversion between
// objects occurs during client-go reads into the cache. This ensures the controller never actually sees old
// representations of the objects (and we lazily migrate storage to the latest representation).
func WithConversion(config *rest.Config, scheme *runtime.Scheme) *rest.Config {
	config.NegotiatedSerializer = NewConversionSerializer(scheme)
	return config
}

// NewConversionSerializer creates a new negotiated serializer that handles detection/conversion between versions
// of redskyops.dev objects.
func NewConversionSerializer(scheme *runtime.Scheme) runtime.NegotiatedSerializer {
	return &ConversionSerializer{
		NegotiatedSerializer: serializer.NewCodecFactory(scheme).WithoutConversion(),
		scheme:               scheme,
	}
}

// ConversionSerializer is a negotiated serializer that also handles the representation migration hack.
type ConversionSerializer struct {
	runtime.NegotiatedSerializer
	scheme *runtime.Scheme
}

func (c *ConversionSerializer) DecoderToVersion(serializer runtime.Decoder, gv runtime.GroupVersioner) runtime.Decoder {
	return &ConversionDecoder{Decoder: c.NegotiatedSerializer.DecoderToVersion(serializer, gv), scheme: c.scheme}
}

// ConversionDecoder is a decoder that decodes with the representation migration hack.
type ConversionDecoder struct {
	runtime.Decoder
	scheme *runtime.Scheme
}

func (c *ConversionDecoder) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	// Use the delegate to perform the initial decode
	obj, gvk, err := c.Decoder.Decode(data, defaults, into)
	if err != nil || gvk.Group != "redskyops.dev" {
		return obj, gvk, err
	}

	// It is possible we may need to attempt conversion from multiple types
	fromGvk := *gvk
	for _, fromVersion := range c.mayNeedConversionFrom(obj) {
		fromGvk.Version = fromVersion
		from, err := c.decodeAs(data, &fromGvk)
		if err != nil {
			return nil, nil, err
		}

		if err = c.convert(from, obj); err != nil {
			return nil, nil, err
		}
	}

	return obj, gvk, err
}

func (c *ConversionDecoder) decodeAs(data []byte, gvk *schema.GroupVersionKind) (runtime.Object, error) {
	// The decoder will recognize the version mismatch, so decode into an unstructured object first
	u, _, err := c.Decoder.Decode(data, nil, &unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}
	u.GetObjectKind().SetGroupVersionKind(*gvk)

	// Convert the unstructured object to a typed object to makes `needsConversion` easier to implement
	t, err := c.scheme.New(*gvk)
	if err != nil {
		return nil, err
	}
	if err := c.scheme.Convert(u, t, nil); err != nil {
		return nil, err
	}
	return t, nil
}

func (c *ConversionDecoder) convert(from, obj runtime.Object) error {
	// For lists, only some of the items may require conversion
	if meta.IsListType(obj) {
		return eachPairwiseListItem(from, obj, c.convert)
	}

	// Only convert if it is actually needed
	if c.needsConversion(from, obj) {
		return c.scheme.Convert(from, obj, nil)
	}
	return nil
}

// mayNeedConversionFrom is used to avoid unnecessary conversion, it returns all of the possible versions to try converting from
func (c *ConversionDecoder) mayNeedConversionFrom(obj runtime.Object) []string {
	// Assume the list version matches the items version so we can look at the items individually
	if meta.IsListType(obj) {
		versions := make(map[string]struct{})
		_ = meta.EachListItem(obj, func(item runtime.Object) error {
			for _, v := range c.mayNeedConversionFrom(item) {
				versions[v] = struct{}{}
			}
			return nil
		})
		result := make([]string, 0, len(versions))
		for v := range versions {
			result = append(result, v)
		}
		return result
	}

	// Try to identify objects which MAY need conversion

	// For example: checks can look for zero values on renamed or moved fields, if the field has a zero value
	// because it was never actually specified, we should catch that in `needsConversion` once we can look at
	// both representations at the same time.

	// Note that the cases of the inner switch statements in `needsConversion` should correspond to the list
	// of versions we return here.

	switch o := obj.(type) {
	case *v1beta1.Experiment:
		if reflect.DeepEqual(o.Spec.TrialTemplate, v1beta1.TrialTemplateSpec{}) {
			return []string{"v1alpha1"}
		}
	case *v1beta1.Trial:
		if o.Spec.JobTemplate == nil {
			return []string{"v1alpha1"}
		}
		if len(o.Status.PatchOperations) == 0 {
			return []string{"v1alpha1"}
		}
		if len(o.Status.ReadinessChecks) == 0 {
			return []string{"v1alpha1"}
		}
	}
	return nil
}

// needsConversion confirms the need to convert between two types
func (c *ConversionDecoder) needsConversion(from, obj runtime.Object) bool {
	switch o := obj.(type) {
	case *v1beta1.Experiment:
		switch f := from.(type) {
		case *v1alpha1.Experiment:
			if !reflect.DeepEqual(o.Spec.TrialTemplate, f.Spec.Template) {
				return true
			}
		}
	case *v1beta1.Trial:
		switch f := from.(type) {
		case *v1alpha1.Trial:
			if o.Spec.JobTemplate == nil && f.Spec.Template != nil {
				return true
			}
			if len(o.Status.PatchOperations) == 0 && len(f.Spec.PatchOperations) > 0 {
				return true
			}
			if len(o.Status.ReadinessChecks) == 0 && len(f.Spec.ReadinessChecks) > 0 {
				return true
			}
		}
	}
	return false
}

// eachPairwiseListItem is like EachListItem except it iterates over two identical length lists
func eachPairwiseListItem(list1, list2 runtime.Object, fn func(runtime.Object, runtime.Object) error) error {
	list1Ptr, err := meta.GetItemsPtr(list1)
	if err != nil {
		return err
	}
	items1, err := conversion.EnforcePtr(list1Ptr)
	if err != nil {
		return err
	}

	list2Ptr, err := meta.GetItemsPtr(list2)
	if err != nil {
		return err
	}
	items2, err := conversion.EnforcePtr(list2Ptr)
	if err != nil {
		return err
	}

	length := items1.Len()
	if items2.Len() != length {
		return fmt.Errorf("expected lists to contain the same number of items")
	} else if length == 0 {
		return nil
	}
	for i := 0; i < length; i++ {
		// This is a reflective shortcut based on how we intend to use this
		item1 := items1.Index(i).Addr().Interface().(runtime.Object)
		item2 := items2.Index(i).Addr().Interface().(runtime.Object)
		if err := fn(item1, item2); err != nil {
			return err
		}
	}
	return nil
}
