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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion       = schema.GroupVersion{Group: "redskyops.dev", Version: "v1alpha1"}
	SchemeBuilder      = &scheme.Builder{GroupVersion: GroupVersion}
	localSchemeBuilder = &SchemeBuilder.SchemeBuilder
)

// Things to consider adding:
// 1. A label selector for objects to include in the scan (alternately, an annotation to exclude?)

// Application represents the configuration of the experiment generator. The generator will consider
// these values when constructing a new `Experiment` resource.
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Resources are references to application resources to consider in the generation of the experiment.
	// These strings are the same format as used by Kustomize.
	Resources []string `json:"resources,omitempty"`

	Cost *CostConfig `json:"cost,omitempty"`

	CloudProvider *CloudProvider `json:"cloudProvider,omitempty"`
}

type CostConfig struct {
	// Labels of the pods which should be considered when collecting cost information.
	Labels map[string]string `json:"labels,omitempty"`
}

type CloudProvider struct {
	// Cloud provider name, may be used to adjust defaults
	Name string `json:"name,omitempty"`
	// Per-resource cost weightings
	Cost corev1.ResourceList `json:"cost,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Application{})
	localSchemeBuilder.Register(RegisterDefaults)
}

func RegisterDefaults(s *runtime.Scheme) error {
	s.AddTypeDefaultingFunc(&Application{}, func(obj interface{}) { obj.(*Application).Default() })
	return nil
}

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
