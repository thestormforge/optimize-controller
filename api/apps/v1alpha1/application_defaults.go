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

package v1alpha1

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	localSchemeBuilder.Register(RegisterDefaults)
}

// Register the defaulting function for the application root object.
func RegisterDefaults(s *runtime.Scheme) error {
	s.AddTypeDefaultingFunc(&Application{}, func(obj interface{}) { obj.(*Application).Default() })
	return nil
}

var _ admission.Defaulter = &Application{}

func (in *Application) Default() {
	for i := range in.Scenarios {
		in.Scenarios[i].Default()
	}

	for i := range in.Objectives {
		in.Objectives[i].Default()
	}
}

func (in *Scenario) Default() {
	if in.Name == "" {
		in.Name = "default"
	}
}

func (in *Objective) Default() {
	v := reflect.Indirect(reflect.ValueOf(in))
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() != reflect.Ptr {
			continue
		}

		f := v.Type().Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if v.Field(i).IsNil() {
			if in.Name == name {
				v.Field(i).Set(reflect.New(f.Type.Elem()))
			}
		} else if in.Name == "" {
			in.Name = name
		}
	}
}
