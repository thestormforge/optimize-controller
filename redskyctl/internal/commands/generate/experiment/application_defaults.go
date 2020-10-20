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

package experiment

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	if in.CloudProvider == nil {
		in.CloudProvider = &CloudProvider{}
	}
	in.CloudProvider.Default()
}

func (in *CloudProvider) Default() {
	if in.Cost == nil {
		in.Cost = corev1.ResourceList{}
	}
	if in.Cost.Cpu().IsZero() {
		switch in.Name {
		default:
			in.Cost[corev1.ResourceCPU] = resource.MustParse("22")
		}
	}
	if in.Cost.Memory().IsZero() {
		switch in.Name {
		default:
			in.Cost[corev1.ResourceMemory] = resource.MustParse("3")
		}
	}
}
