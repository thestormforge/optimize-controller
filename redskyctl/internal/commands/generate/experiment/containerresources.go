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
	"encoding/json"
	"math"
	"regexp"

	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ContainerResourcesSelector identifies zero or more container resources specifications.
// NOTE: This object is basically a combination of a Kustomize FieldSpec and a Selector.
type ContainerResourcesSelector struct {
	// Type information of the resources to consider.
	resid.Gvk `json:",inline,omitempty"`
	// Namespace of the resources to consider.
	Namespace string `json:"namespace,omitempty"`
	// Name of the resources to consider.
	Name string `json:"name,omitempty"`
	// Annotation selector of resources to consider.
	AnnotationSelector string `json:"annotationSelector,omitempty"`
	// Label selector of resources to consider.
	LabelSelector string `json:"labelSelector,omitempty"`
	// Path to the list of containers.
	Path string `json:"path,omitempty"`
	// Create container resource specifications even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
	// Regular expression matching the container name.
	ContainerName string `json:"containerName,omitempty"`
}

// fieldSpec returns this selector as a Kustomize FieldSpec.
func (rs *ContainerResourcesSelector) fieldSpec() types.FieldSpec {
	return types.FieldSpec{
		Gvk:                rs.Gvk,
		Path:               rs.Path,
		CreateIfNotPresent: rs.CreateIfNotPresent,
	}
}

// selector resturns this selector as a Kustomize Selector.
func (rs *ContainerResourcesSelector) selector() types.Selector {
	return types.Selector{
		Gvk:                rs.Gvk,
		Namespace:          rs.Namespace,
		Name:               rs.Name,
		AnnotationSelector: rs.AnnotationSelector,
		LabelSelector:      rs.LabelSelector,
	}
}

// matchesContainerName checks to see if the specified container name is matched.
func (rs *ContainerResourcesSelector) matchesContainerName(name string) bool {
	// Treat empty like ".*"
	if rs.ContainerName == "" {
		return true
	}

	containerName, err := regexp.Compile("^" + rs.ContainerName + "$")
	if err != nil {
		// Kustomize panics. Not sure where it validates. We will just fall back to exact match
		return rs.ContainerName == name
	}

	return containerName.MatchString(name)
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

// scanForContainerResources scans the supplied resource map for container resources matching the selector.
func (g *Generator) scanForContainerResources(ars []*applicationResource, rm resmap.ResMap) ([]*applicationResource, error) {
	for _, sel := range g.ContainerResourcesSelectors {
		// Select the matching resources
		resources, err := rm.Select(sel.selector())
		if err != nil {
			return nil, err
		}

		for _, r := range resources {
			// Get the YAML tree representation of the resource
			node, err := filtersutil.GetRNode(r)
			if err != nil {
				return nil, err
			}

			// Scan the document tree for information to add to the application resource
			ar := &applicationResource{}
			if err := ar.saveTargetReference(node); err != nil {
				return nil, err
			}
			if err := ar.saveContainerResourcesPaths(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return nil, err
			}
			if len(ar.containerResourcesPaths) == 0 {
				continue
			}

			// Make sure we only get the newly discovered parts
			ars = mergeOrAppend(ars, ar)
		}
	}

	return ars, nil
}

// saveResourcesPaths extracts the paths to the `resources` elements from the supplied node.
func (r *applicationResource) saveContainerResourcesPaths(node *yaml.RNode, sel ContainerResourcesSelector) error {
	path := sel.fieldSpec().PathSlice()
	return node.PipeE(
		yaml.Lookup(path...),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			return nil, node.VisitElements(func(node *yaml.RNode) error {
				rl := node.Field("resources")
				if rl == nil && !sel.CreateIfNotPresent {
					return nil
				}

				name := node.Field("name").Value.YNode().Value
				if !sel.matchesContainerName(name) {
					return nil
				}

				r.containerResourcesPaths = append(r.containerResourcesPaths, append(path, "[name="+name+"]", "resources"))
				r.containerResources = append(r.containerResources, materializeResourceList(rl))
				return nil
			})
		}))
}

// resourcesParameters returns the parameters required for optimizing the discovered resources sections.
func (r *applicationResource) containerResourcesParameters(name nameGen) []redskyv1beta1.Parameter {
	parameters := make([]redskyv1beta1.Parameter, 0, len(r.containerResourcesPaths)*2)
	for i := range r.containerResourcesPaths {
		var baselineMemory, baselineCPU *intstr.IntOrString
		var minMemory, maxMemory int32 = 128, 4096
		var minCPU, maxCPU int32 = 100, 4000

		if q, ok := r.containerResources[i][corev1.ResourceMemory]; ok {
			baselineMemory, minMemory, maxMemory = toIntWithRange(corev1.ResourceMemory, q)
		}

		if q, ok := r.containerResources[i][corev1.ResourceCPU]; ok {
			baselineCPU, minCPU, maxCPU = toIntWithRange(corev1.ResourceCPU, q)
		}

		parameters = append(parameters, redskyv1beta1.Parameter{
			Name:     name(&r.targetRef, r.containerResourcesPaths[i], "memory"),
			Min:      minMemory,
			Max:      maxMemory,
			Baseline: baselineMemory,
		}, redskyv1beta1.Parameter{
			Name:     name(&r.targetRef, r.containerResourcesPaths[i], "cpu"),
			Min:      minCPU,
			Max:      maxCPU,
			Baseline: baselineCPU,
		})
	}
	return parameters
}

// materializeResourceList returns a resource list from the supplied node.
func materializeResourceList(node *yaml.MapNode) corev1.ResourceList {
	resources := struct {
		Limits   corev1.ResourceList `json:"limits"`
		Requests corev1.ResourceList `json:"requests"`
	}{}
	if node != nil {
		if data, err := node.Value.MarshalJSON(); err == nil {
			_ = json.Unmarshal(data, &resources)
		}
	}
	return resources.Requests
}

// toIntWithRange returns the quantity as an int value with a default range.
func toIntWithRange(name corev1.ResourceName, q resource.Quantity) (value *intstr.IntOrString, min int32, max int32) {
	var scaled intstr.IntOrString

	switch name {
	case corev1.ResourceMemory:
		scaled = intstr.FromInt(int(q.ScaledValue(resource.Mega)))
		min = int32(math.Pow(2, math.Floor(math.Log2(float64(scaled.IntVal/2)))))
		max = int32(math.Pow(2, math.Ceil(math.Log2(float64(scaled.IntVal*2)))))
	case corev1.ResourceCPU:
		scaled = intstr.FromInt(int(q.ScaledValue(resource.Milli)))
		min = int32((float64(scaled.IntVal) / 200) * 100)
		max = int32((float64(scaled.IntVal) / 50) * 100)
	}

	return &scaled, min, max
}
