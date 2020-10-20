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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO This whole thing should move over to redskyops-go

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

	// Cost is used to identify which parts of the application impact the cost of running the application.
	Cost *CostConfig `json:"cost,omitempty"`

	// CloudProvider is used to provide details about the hosting environment the application is run in.
	CloudProvider *CloudProvider `json:"cloudProvider,omitempty"`
}

// +kubebuilder:object:generate=true
type CostConfig struct {
	// Labels of the pods which should be considered when collecting cost information.
	Labels map[string]string `json:"labels,omitempty"`
}

// +kubebuilder:object:generate=true
type CloudProvider struct {
	// Cloud provider name, may be used to adjust defaults
	Name string `json:"name,omitempty"`
	// Per-resource cost weightings
	Cost corev1.ResourceList `json:"cost,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Application{})
}
