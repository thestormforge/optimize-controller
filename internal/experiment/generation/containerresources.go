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

	"github.com/thestormforge/konjure/pkg/filters"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ContainerResourcesSelector scans for container resources specifications (requests/limits).
type ContainerResourcesSelector struct {
	scan.GenericSelector
	// Regular expression matching the container name.
	ContainerName string `json:"containerName,omitempty"`
	// Path to the resource requirements.
	Path string `json:"path,omitempty"`
	// Names of the resources to select, defaults to ["cpu", "memory"].
	Resources []corev1.ResourceName `json:"resources,omitempty"`
	// Create container resource requirements even if the original object does not contain them.
	CreateIfNotPresent bool `json:"create,omitempty"`
	// Per-namespace limit ranges for containers.
	ContainerLimitRange map[string]corev1.LimitRangeItem `json:"containerLimitRange,omitempty"`
}

var _ scan.Selector = &ContainerResourcesSelector{}

// Default applies default values to the selector.
func (s *ContainerResourcesSelector) Default() {
	if s.Kind == "" {
		s.Group = "apps|extensions"
		s.Kind = "Deployment|StatefulSet"
		s.Path = "/spec/template/spec/containers/[name={ .ContainerName }]/resources"
	}

	if len(s.Resources) == 0 {
		s.Resources = []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}
	}
}

// Select matches all of the generically match nodes plus any `LimitRange` resources
// that we can use to collect default values from.
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

	// Evaluate and validate the path
	path, err := sfio.FieldPath(s.Path, map[string]string{"ContainerName": s.ContainerName})
	if err != nil {
		return nil, err
	}
	if len(path) < 2 {
		return nil, fmt.Errorf("path %q is invalid, must contain at least two elements", s.Path)
	}
	lastPath := len(path) - 1
	if yaml.IsListIndex(path[lastPath]) {
		return nil, fmt.Errorf("path %q is invalid, a sequence of resources is not supported", s.Path)
	}

	// Create the matchers (we use two matchers so we can support "create if not present" on the "resources" field)
	containerMatcher := yaml.PathMatcher{Path: path[:lastPath]}
	resourcesMatcher := yaml.FieldMatcher{Name: path[lastPath]}
	if s.CreateIfNotPresent {
		resourcesMatcher.Create = yaml.NewMapRNode(nil)
	}

	return result, node.PipeE(sfio.TeeMatched(
		containerMatcher,
		sfio.PreserveFieldMatcherPath(resourcesMatcher),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			result = append(result, &containerResourcesParameter{
				pnode: pnode{
					meta:      meta,
					fieldPath: node.FieldPath(),
					value:     node.YNode(),
				},
				resources:  s.Resources,
				limitRange: s.ContainerLimitRange[meta.Namespace],
			})
			return node, nil
		}),
	))
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

	if s.ContainerLimitRange == nil {
		s.ContainerLimitRange = make(map[string]corev1.LimitRangeItem)
	}

	lri := s.ContainerLimitRange[namespace]
	if err := sfio.DecodeYAMLToJSON(lriNode, &lri); err != nil {
		return err
	}
	s.ContainerLimitRange[namespace] = lri

	return nil
}

// containerResourcesParameter is used to record the position of a container resources specification
// found by the selector during scanning.
type containerResourcesParameter struct {
	pnode
	resources  []corev1.ResourceName
	limitRange corev1.LimitRangeItem
}

var _ PatchSource = &containerResourcesParameter{}
var _ ParameterSource = &containerResourcesParameter{}

// Patch produces a YAML filter for updating a strategic merge patch with a parameterized
// container resources specification.
func (p *containerResourcesParameter) Patch(name ParameterNamer) (yaml.Filter, error) {
	ind, err := p.indexContainerResources()
	if err != nil {
		return nil, err
	}

	// We can build directly into the filters used to create the final patch
	path := yaml.PathGetter{Path: p.fieldPath, Create: yaml.MappingNode}
	limitsPatch := yaml.FieldSetter{Name: "limits", Value: yaml.NewMapRNode(nil)}
	requestsPatch := yaml.FieldSetter{Name: "requests", Value: yaml.NewMapRNode(nil)}

	for _, rn := range p.resources {
		// Create patch filter for each ResourceName (e.g. "cpu: {{ .Values ...")
		parameterName := name(p.meta, p.fieldPath, string(rn))
		patch := fmt.Sprintf("{{ .Values.%s }}%s", parameterName, ind[rn].Suffix())
		patchFilter := yaml.SetField(string(rn), yaml.NewStringRNode(patch))

		// Apply the same patch filter to both limits and requests
		if err := limitsPatch.Value.PipeE(patchFilter); err != nil {
			return nil, err
		}
		if err := requestsPatch.Value.PipeE(patchFilter); err != nil {
			return nil, err
		}
	}

	// Combine the filters using Tee so resulting filter won't change the traversal depth
	return yaml.Tee(path, yaml.Tee(limitsPatch), yaml.Tee(requestsPatch)), nil
}

// Parameters lists the parameters used by the patch.
func (p *containerResourcesParameter) Parameters(name ParameterNamer) ([]optimizev1beta2.Parameter, error) {
	ind, err := p.indexContainerResources()
	if err != nil {
		return nil, err
	}

	var result []optimizev1beta2.Parameter
	for _, rn := range p.resources {
		result = append(result, optimizev1beta2.Parameter{
			Name:     name(p.meta, p.fieldPath, string(rn)),
			Max:      ind[rn].Max(),
			Min:      ind[rn].Min(),
			Baseline: ind[rn].Baseline(),
		})
	}

	return result, nil
}

// indexContainerResources collects the container resources for this parameter.
func (p *containerResourcesParameter) indexContainerResources() (map[corev1.ResourceName]containerResources, error) {
	// Decode the resource requirements we found during the scan
	scannedValue := corev1.ResourceRequirements{}
	if err := sfio.DecodeYAMLToJSON(yaml.NewRNode(p.value), &scannedValue); err != nil {
		return nil, err
	}

	// For each configured resource, capture the baseline and range
	result := make(map[corev1.ResourceName]containerResources, len(p.resources))
	for _, rn := range p.resources {
		result[rn] = containerResources{
			max:          lookupQuantity(rn, p.limitRange.Max, defaultLimitRange.Max),
			min:          lookupQuantity(rn, p.limitRange.Min, defaultLimitRange.Min),
			baseline:     lookupQuantity(rn, scannedValue.Requests, p.limitRange.DefaultRequest, defaultLimitRange.DefaultRequest),
			defaultScale: defaultScale[rn],
		}
	}

	return result, nil
}

// lookupQuantity returns a quantity from the first resource list that has it.
func lookupQuantity(rn corev1.ResourceName, rl ...corev1.ResourceList) resource.Quantity {
	for i := range rl {
		if q, ok := rl[i][rn]; ok {
			return q
		}
	}
	return *resource.NewQuantity(0, resource.DecimalExponent)
}

var (
	// defaultLimitRange acts as a backstop for finding per-resource values.
	defaultLimitRange = corev1.LimitRangeItem{
		Type: "Container",
		Max: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4000m"),
			corev1.ResourceMemory: resource.MustParse("4096Mi"),
		},
		Min: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		DefaultRequest: corev1.ResourceList{
			// Even though the values are all 0, we need to capture the format
			corev1.ResourceCPU:              *resource.NewQuantity(0, resource.DecimalSI),
			corev1.ResourceEphemeralStorage: *resource.NewQuantity(0, resource.DecimalSI),
			corev1.ResourceMemory:           *resource.NewQuantity(0, resource.BinarySI),
			corev1.ResourceStorage:          *resource.NewQuantity(0, resource.DecimalSI),
		},
	}

	// defaultScale determines the default scale on which we optimize. For example,
	// instead of optimizing "2Gi" of memory as "2-4", we optimize it as "2048-4096Mi"; or
	// instead of optimizing "1.0" milli-CPUs as "0-2", we optimize it as "500-2000".
	defaultScale = map[corev1.ResourceName]resource.Scale{
		corev1.ResourceCPU:              resource.Milli, // m
		corev1.ResourceEphemeralStorage: resource.Mega,  // M
		corev1.ResourceMemory:           resource.Mega,  // Mi
		corev1.ResourceStorage:          resource.Mega,  // M
	}
)

// containerResources contains the quantity range for a resource.
type containerResources struct {
	max          resource.Quantity
	min          resource.Quantity
	baseline     resource.Quantity
	defaultScale resource.Scale
}

// Max returns the configured maximum, or twice the baseline (provided it is smaller then the max).
func (cr containerResources) Max() int32 {
	max := cr.max
	max.Format = cr.baseline.Format

	if !cr.baseline.IsZero() {
		if max.Value() == 0 || cr.baseline.Value()*2 < max.Value() {
			max.Set(cr.baseline.Value() * 2)
		}
	}

	return AsScaledInt(max, cr.scale())
}

// Min returns the the configured minimum or half the baseline.
func (cr containerResources) Min() int32 {
	if !cr.baseline.IsZero() {
		return AsScaledInt(cr.baseline, cr.scale()) / 2
	}

	min := cr.min
	min.Format = cr.baseline.Format
	return AsScaledInt(min, cr.scale())
}

// Baseline returns the non-zero baseline value or nil for a zero value.
func (cr containerResources) Baseline() *intstr.IntOrString {
	if cr.baseline.IsZero() {
		return nil
	}

	return &intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: AsScaledInt(cr.baseline, cr.scale()),
	}
}

// Suffix returns the appropriate suffix based on the scale and format. For example,
// a mega-binary is "Mi" and giga-decimal is "G".
func (cr containerResources) Suffix() string {
	return QuantitySuffix(cr.scale(), cr.baseline.Format)
}

// scale is used to determine what scale all the values should be recorded using.
// This is important because we do not want to have mismatched scales (e.g. a min
// of 1000 and a baseline of 2).
func (cr containerResources) scale() resource.Scale {
	if cr.baseline.IsZero() {
		return cr.defaultScale
	}

	// Just try to find the first scale that doesn't round to 0
	for scale := cr.defaultScale; scale >= resource.Nano; scale -= 3 {
		if AsScaledInt(cr.baseline, scale) > 0 {
			return scale
		}
	}
	return cr.defaultScale
}
