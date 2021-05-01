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

package commander

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1alpha1 "github.com/thestormforge/optimize-controller/api/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/controller"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceReader helps properly decode Kubernetes resources on the CLI. It is meant to be a
// lighter weight alternative to the cli-runtime resource.Builder.
type ResourceReader struct {
	Scheme           *runtime.Scheme
	PreferredVersion runtime.GroupVersioner
}

// NewResourceReader returns a new resource reader for the supplied byte stream.
func NewResourceReader() *ResourceReader {
	rr := &ResourceReader{
		Scheme:           runtime.NewScheme(),
		PreferredVersion: OnlyVersion,
	}

	// Always add our types
	_ = redskyv1beta1.AddToScheme(rr.Scheme)
	_ = redskyv1alpha1.AddToScheme(rr.Scheme)
	_ = redskyappsv1alpha1.AddToScheme(rr.Scheme)

	// Allow single experiments to target an experiment list
	_ = addExperimentListConversions(rr.Scheme)

	return rr
}

// ReadInto decodes the supplied byte stream into the target runtime object. The default values
// and type information of the target object will be populated.
func (r *ResourceReader) ReadInto(reader io.ReadCloser, target runtime.Object) error {
	// Determine the GVK for the type we are supposed to be populating
	gvk, err := r.objectKind(target)
	if err != nil {
		return err
	}

	mt := mediaType(reader)
	decoder, err := r.decoder(mt)
	if err != nil {
		return err
	}

	// TODO For lists we should consider yaml.NewDocumentDecoder(reader) so we can read a stream

	// Read in the raw bytes
	defer func() { _ = reader.Close() }()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	// Decode the raw data
	obj, _, err := decoder.Decode(data, &gvk, target)
	if err != nil {
		return err
	}

	// If decode returned an object of a different type we should try to force the conversion
	if obj != target {
		if err := r.Scheme.Convert(obj, target, r.PreferredVersion); err != nil {
			return err
		}
	}

	// Fill in the default values for the target
	target.GetObjectKind().SetGroupVersionKind(gvk)
	r.Scheme.Default(target)

	return nil
}

func (r *ResourceReader) objectKind(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := r.Scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	gvk, ok := r.PreferredVersion.KindForGroupVersionKinds(gvks)
	if !ok {
		// Your code must supply a GroupVersioner to disambiguate this case
		panic("read destination type is ambiguous in scheme")
	}

	return gvk, nil
}

func (r *ResourceReader) decoder(mediaType string) (runtime.Decoder, error) {
	// Create the conversion serializer with our negotiation logic in it
	cs := controller.NewConversionSerializer(r.Scheme)
	info, ok := runtime.SerializerInfoForMediaType(cs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("could not find serializer for %s", mediaType)
	}

	return cs.DecoderToVersion(info.Serializer, r.PreferredVersion), nil
}

// mediaType returns the presumed media type for the supplied read closer.
func mediaType(r io.ReadCloser) string {
	// For now just assume YAML unless we got a JSON file
	mt := runtime.ContentTypeYAML
	if f, ok := r.(*os.File); ok {
		switch filepath.Ext(f.Name()) {
		case "json":
			mt = runtime.ContentTypeJSON
		}
	}

	return mt
}

// OnlyVersion is a group version that only resolves if there is a single possible kind
var OnlyVersion runtime.GroupVersioner = onlyVersion{}

type onlyVersion struct{}

func (onlyVersion) Identifier() string { return "only" }
func (onlyVersion) KindForGroupVersionKinds(kinds []schema.GroupVersionKind) (schema.GroupVersionKind, bool) {
	if len(kinds) == 1 {
		return kinds[0], true
	}

	return schema.GroupVersionKind{}, false
}

// addExperimentListConversions adds conversions from a single experiment to a list of experiments; for example
// when a command can handle a list but the user only supplies a single experiment.
func addExperimentListConversions(s *runtime.Scheme) error {
	// Convert from a single experiment to a list of experiments
	if err := s.AddConversionFunc((*redskyv1beta1.Experiment)(nil), (*redskyv1beta1.ExperimentList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		b.(*redskyv1beta1.ExperimentList).Items = []redskyv1beta1.Experiment{*a.(*redskyv1beta1.Experiment)}
		return nil
	}); err != nil {
		return err
	}

	// Convert from a single v1alpha1 experiment to a list of experiments
	if err := s.AddConversionFunc((*redskyv1alpha1.Experiment)(nil), (*redskyv1beta1.ExperimentList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		l := b.(*redskyv1beta1.ExperimentList)
		l.Items = make([]redskyv1beta1.Experiment, 1)
		return scope.Convert(a, &l.Items[0], scope.Flags())
	}); err != nil {
		return err
	}

	return nil
}
