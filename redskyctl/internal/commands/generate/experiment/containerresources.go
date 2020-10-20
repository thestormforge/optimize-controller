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
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
)

// ContainerResourcesSelector identifies zero or more container resources specifications.
// NOTE: This object is basically a combination of a Kustomize FieldSpec and a Selector.
type ContainerResourcesSelector struct {
	// Type information of the resources to consider
	resid.Gvk `json:",inline,omitempty"`
	// Namespace of the resources to consider
	Namespace string `json:"namespace,omitempty"`
	// Name of the resources to consider
	Name string `json:"name,omitempty"`
	// Annotation selector of resources to consider
	AnnotationSelector string `json:"annotationSelector,omitempty"`
	// Label selector of resources to consider
	LabelSelector string `json:"labelSelector,omitempty"`
	// Path to the list of containers
	Path string `json:"path,omitempty"`
	// Create container resource specifications even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
}

// fieldSpec returns this ContainerResourcesSelector as a Kustomize FieldSpec.
func (rs *ContainerResourcesSelector) fieldSpec() types.FieldSpec {
	return types.FieldSpec{
		Gvk:                rs.Gvk,
		Path:               rs.Path,
		CreateIfNotPresent: rs.CreateIfNotPresent,
	}
}

// selector resturns this ContainerResourcesSelector as a Kustomize Selector.
func (rs *ContainerResourcesSelector) selector() types.Selector {
	return types.Selector{
		Gvk:                rs.Gvk,
		Namespace:          rs.Namespace,
		Name:               rs.Name,
		AnnotationSelector: rs.AnnotationSelector,
		LabelSelector:      rs.LabelSelector,
	}
}

// DefaultContainerResourcesSelectors returns the default container resource selectors. These selectors match
// the default role created by the `grant_permissions` code.
func DefaultContainerResourcesSelectors() []ContainerResourcesSelector {
	return []ContainerResourcesSelector{
		{
			Gvk:                resid.Gvk{Group: "apps", Kind: "Deployment"},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "extensions", Kind: "Deployment"},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "apps", Kind: "StatefulSet"},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
		{
			Gvk:                resid.Gvk{Group: "extensions", Kind: "StatefulSet"},
			Path:               "/spec/template/spec/containers",
			CreateIfNotPresent: true,
		},
	}
}
