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
	"sigs.k8s.io/kustomize/api/types"
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

// selector returns this selector as a Kustomize Selector.
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

func (rs *ContainerResourcesSelector) findParameters(node *yaml.RNode) ([]applicationResourceParameter, error) {
	var result []applicationResourceParameter

	path := rs.fieldSpec().PathSlice() // Path to the containers list
	err := node.PipeE(
		yaml.Lookup(path...),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			return nil, node.VisitElements(func(node *yaml.RNode) error {
				rl := node.Field("resources")
				if rl == nil && !rs.CreateIfNotPresent {
					return nil
				}

				name := node.Field("name").Value.YNode().Value
				if !rs.matchesContainerName(name) {
					return nil
				}

				p := containerResourcesParameter{}
				p.fieldPath = append(path, "[name="+name+"]", "resources")
				if rl != nil {
					p.value = rl.Value.YNode()
				}

				result = append(result, &p)
				return nil
			})
		}))

	if err != nil {
		return nil, err
	}

	return result, nil
}

type containerResourcesParameter struct {
	anode
}

func (p *containerResourcesParameter) patch(name resNameGen) (yaml.Filter, error) {
	values := yaml.NewMapRNode(&map[string]string{
		"memory": "{{ .Values." + name(p.fieldPath, "memory") + " }}M",
		"cpu":    "{{ .Values." + name(p.fieldPath, "cpu") + " }}m",
	})

	return yaml.Tee(
		&yaml.PathGetter{Path: p.fieldPath, Create: yaml.MappingNode},
		yaml.Tee(yaml.SetField("limits", values)),
		yaml.Tee(yaml.SetField("requests", values)),
	), nil
}

func (p *containerResourcesParameter) parameters(name resNameGen) ([]redskyv1beta1.Parameter, error) {
	// ResourceList (actually Quantities) don't unmarshal as YAML so we need to round trip through JSON
	data, err := yaml.NewRNode(p.value).MarshalJSON()
	if err != nil {
		return nil, err
	}
	r := struct {
		Limits   corev1.ResourceList `json:"limits"`
		Requests corev1.ResourceList `json:"requests"`
	}{}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	baselineMemory, minMemory, maxMemory := toIntWithRange(r.Requests, corev1.ResourceMemory)
	baselineCPU, minCPU, maxCPU := toIntWithRange(r.Requests, corev1.ResourceCPU)

	return []redskyv1beta1.Parameter{{
		Name:     name(p.fieldPath, "memory"),
		Min:      minMemory,
		Max:      maxMemory,
		Baseline: baselineMemory,
	}, {
		Name:     name(p.fieldPath, "cpu"),
		Min:      minCPU,
		Max:      maxCPU,
		Baseline: baselineCPU,
	}}, nil
}

// toIntWithRange returns the quantity as an int value with a default range.
func toIntWithRange(resources corev1.ResourceList, name corev1.ResourceName) (value *intstr.IntOrString, min int32, max int32) {
	q, ok := resources[name]
	if ok {
		value = new(intstr.IntOrString)
	}

	switch name {
	case corev1.ResourceMemory:
		min, max = 128, 4096
		if value != nil {
			*value = intstr.FromInt(int(q.ScaledValue(resource.Mega)))
			min = int32(math.Pow(2, math.Floor(math.Log2(float64(value.IntVal/2)))))
			setMax(&max, value.IntVal, int32(math.Pow(2, math.Ceil(math.Log2(float64(value.IntVal*2))))))
		}

	case corev1.ResourceCPU:
		min, max = 100, 4000
		if value != nil {
			*value = intstr.FromInt(int(q.ScaledValue(resource.Milli)))
			min = int32(math.Floor(float64(value.IntVal)/20)) * 10
			setMax(&max, value.IntVal, int32(math.Ceil(float64(value.IntVal)/10))*20)
		}
	}

	return value, min, max
}

// setMax ensures the computed upper bound is capped by the larger of the baseline value or the default maximum.
func setMax(m *int32, v, b int32) {
	if v > *m {
		*m = v
	} else if b < *m {
		*m = b
	}
}
