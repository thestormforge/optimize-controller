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

package generation

import (
	"fmt"
	"math"
	"regexp"

	"github.com/thestormforge/konjure/pkg/filters"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ContainerResourcesSelector scans for container resources specifications (requests/limits).
type ContainerResourcesSelector struct {
	scan.GenericSelector
	// Path to the list of containers.
	Path string `json:"path,omitempty"`
	// Create container resource specifications even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
	// Regular expression matching the container name.
	ContainerName string `json:"containerName,omitempty"`
	// Names of the resources to select, defaults to ["memory", "cpu"].
	Resources []corev1.ResourceName `json:"resources,omitempty"`

	// Per-namespace limit ranges for containers as encountered during a scan.
	containerLimitRange map[string]corev1.LimitRangeItem
}

var _ scan.Selector = &ContainerResourcesSelector{}

func (s *ContainerResourcesSelector) Select(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	var result []*yaml.RNode

	// In addition to the actual nodes, we collect the limit ranges and put them
	// at the front of the list so we can process them first
	limitRangeSelector := &filters.ResourceMetaFilter{Version: "v1", Kind: "LimitRange"}
	limitRangeNodes, err := limitRangeSelector.Filter(nodes)
	if err != nil {
		return nil, err
	}
	result = append(result, limitRangeNodes...)

	// Select the actual nodes
	resourceNodes, err := s.GenericSelector.Select(nodes)
	if err != nil {
		return nil, err
	}
	result = append(result, resourceNodes...)

	return result, nil
}

// Map inspects the supplied resource for container resources specifications.
func (s *ContainerResourcesSelector) Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error) {
	var result []interface{}

	// Capture container limit ranges. Because all the limit range nodes were at
	// the front of the list returned by Select, we should have a chance to process
	// all of them before we start getting real nodes.
	if meta.APIVersion == "v1" && meta.Kind == "LimitRange" {
		return nil, s.saveContainerLimitRange(meta.Namespace, node)
	}

	path := splitPath(s.Path) // Path to the containers list
	err := node.PipeE(
		yaml.Lookup(path...),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			return nil, node.VisitElements(func(node *yaml.RNode) error {
				rl := node.Field("resources")
				if rl == nil {
					if !s.CreateIfNotPresent {
						return nil
					}
					rl = &yaml.MapNode{Value: yaml.NewMapRNode(nil)}
				}

				name := node.Field("name").Value.YNode().Value
				if !s.matchesContainerName(name) {
					return nil
				}

				p := containerResourcesParameter{
					pnode: pnode{
						meta:      meta,
						fieldPath: append(path, "[name="+name+"]", "resources"),
						value:     rl.Value.YNode(),
					},
					resources: s.Resources,
					limits:    s.containerLimitRange[meta.Namespace],
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

// matchesContainerName checks to see if the specified container name is matched.
func (s *ContainerResourcesSelector) matchesContainerName(name string) bool {
	if s.ContainerName == "" {
		// Treat empty like ".*"
		return true
	}

	containerName, err := regexp.Compile("^(?:" + s.ContainerName + ")$")
	if err != nil {
		// Invalid regexp may still be an exact match on the actual name
		return s.ContainerName == name
	}

	return containerName.MatchString(name)
}

// saveContainerLimitRange captures the container specific limit range item for
// the specified namespace so that it can be used for defaults later.
func (s *ContainerResourcesSelector) saveContainerLimitRange(namespace string, node *yaml.RNode) error {
	lriNode, err := node.Pipe(yaml.Lookup("spec", "limits", "[type=Container]"))
	if err != nil {
		return err
	}
	if lriNode == nil {
		return nil
	}

	lri := corev1.LimitRangeItem{}
	if err := scan.DecodeYAMLToJSON(lriNode, &lri); err != nil {
		return err
	}

	if s.containerLimitRange == nil {
		s.containerLimitRange = make(map[string]corev1.LimitRangeItem)
	}
	s.containerLimitRange[namespace] = lri

	return nil
}

// containerResourcesParameter is used to record the position of a container resources specification
// found by the selector during scanning.
type containerResourcesParameter struct {
	pnode
	resources []corev1.ResourceName
	limits    corev1.LimitRangeItem
}

var _ PatchSource = &containerResourcesParameter{}
var _ ParameterSource = &containerResourcesParameter{}

// Patch produces a YAML filter for updating a strategic merge patch with a parameterized
// container resources specification.
func (p *containerResourcesParameter) Patch(name ParameterNamer) (yaml.Filter, error) {
	patches := make(map[string]string)
	for _, rn := range p.getResources() {
		format := toPatchPattern(rn)
		patches[string(rn)] = fmt.Sprintf(format, name(p.meta, p.fieldPath, string(rn)))
	}

	return yaml.Tee(
		&yaml.PathGetter{Path: p.fieldPath, Create: yaml.MappingNode},
		yaml.Tee(yaml.SetField("limits", yaml.NewMapRNode(&patches))),
		yaml.Tee(yaml.SetField("requests", yaml.NewMapRNode(&patches))),
	), nil
}

// Parameters lists the parameters used by the patch.
func (p *containerResourcesParameter) Parameters(name ParameterNamer) ([]redskyv1beta1.Parameter, error) {
	// ResourceList (actually, Quantities) won't unmarshal as YAML so we need to round trip through JSON
	r := corev1.ResourceRequirements{}
	if err := scan.DecodeYAMLToJSON(yaml.NewRNode(p.value), &r); err != nil {
		return nil, err
	}

	// Build parameters around the requests, taking into consideration the defaults
	requests := corev1.ResourceList{}
	p.limits.DefaultRequest.DeepCopyInto(&requests)
	for rn, q := range r.Requests {
		requests[rn] = q
	}

	var result []redskyv1beta1.Parameter
	for _, rn := range p.getResources() {
		baseline, min, max := toIntWithRange(requests, rn)
		result = append(result, redskyv1beta1.Parameter{
			Name:     name(p.meta, p.fieldPath, string(rn)),
			Min:      min,
			Max:      max,
			Baseline: baseline,
		})
	}

	return result, nil
}

// getResources returns the names of the resources to optimize on.
func (p *containerResourcesParameter) getResources() []corev1.ResourceName {
	if len(p.resources) > 0 {
		return p.resources
	}

	return []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceCPU}
}

// toPatchPattern returns the fmt pattern for a patch of the specified resource name.
func toPatchPattern(name corev1.ResourceName) string {
	switch name {
	case corev1.ResourceMemory:
		return "{{ .Values.%s }}M"
	case corev1.ResourceCPU:
		return "{{ .Values.%s }}m"
	default:
		return ""
	}
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
