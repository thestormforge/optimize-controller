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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ReadExperiment unmarshals a single experiment's data. In a multidoc yaml file, only the first
// experiment is read.
func ReadExperiment(filename string, defaultReader io.Reader) (exp *redskyv1beta1.Experiment, err error) {
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
		return nil, err
	}

	reader := bytes.NewReader(data)
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	baseObj := &metav1.TypeMeta{}
	if err = decoder.Decode(baseObj); err != nil {
		return nil, err
	}

	// Reset the reader
	reader.Seek(0, 0)
	exp = &redskyv1beta1.Experiment{}

	// This seems janky?
	// For whatever reason these values arent populated
	exp.GetObjectKind().SetGroupVersionKind(redskyv1beta1.GroupVersion.WithKind("Experiment"))

	switch baseObj.APIVersion {
	case redskyv1alpha1.GroupVersion.String():
		redo := &redskyv1alpha1.Experiment{}
		if err = decoder.Decode(redo); err != nil {
			return nil, err
		}

		if err = redo.ConvertTo(exp); err != nil {
			return nil, err
		}
	case redskyv1beta1.GroupVersion.String():
		// We could probably do a fallthrough here, but would prefer to be explicit
		if err = decoder.Decode(exp); err != nil {
			return nil, err
		}
	default:
		// Attempt to decode it as the hub/most recent version if not explicit
		if err = decoder.Decode(exp); err != nil {
			return nil, err
		}
	}

	return exp, nil
}
