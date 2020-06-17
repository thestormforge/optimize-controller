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

package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/controller"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// ReadExperiments unmarshals experiment data
func ReadExperiments(filename string, defaultReader io.Reader, list *redskyv1beta1.ExperimentList) (err error) {
	var data []byte

	switch filename {
	case "":
		err = fmt.Errorf("no filename specified")
	case "-":
		data, err = ioutil.ReadAll(defaultReader)
	default:
		data, err = ioutil.ReadFile(filename)
	}
	if err != nil {
		return err
	}

	// Create a decoder
	scheme := runtime.NewScheme()
	_ = redskyv1beta1.AddToScheme(scheme)
	_ = redskyv1alpha1.AddToScheme(scheme)
	_ = registerListConversions(scheme)
	cs := controller.NewConversionSerializer(scheme)

	// TODO: see if we can switch to a better way of handling the serialization of the file
	// this seems pretty fragile
	mediaType := runtime.ContentTypeYAML
	switch filepath.Ext(filename) {
	case "json":
		mediaType = runtime.ContentTypeJSON
	}

	info, ok := runtime.SerializerInfoForMediaType(cs.SupportedMediaTypes(), mediaType)
	if !ok {
		return fmt.Errorf("could not find serializer for %s", mediaType)
	}

	decoder := cs.DecoderToVersion(info.Serializer, runtime.InternalGroupVersioner)

	// NOTE: This attempts to read an "ExperimentList", a stream of experiment YAML documents IS NOT an experiment list
	// TODO If the mediaType is YAML we should use a `yaml.NewDocumentDecoder(...)` to get individual documents
	gvk := redskyv1beta1.GroupVersion.WithKind("ExperimentList")
	obj, _, err := decoder.Decode(data, &gvk, list)
	if err != nil {
		return err
	}

	// If the decoded object was not what we were looking, attempt to convert it
	if obj != list {
		return scheme.Convert(obj, list, nil)
	}

	return nil
}

func registerListConversions(s *runtime.Scheme) error {
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
