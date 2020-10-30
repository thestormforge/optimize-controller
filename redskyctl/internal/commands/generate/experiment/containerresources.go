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
	"bytes"
	"regexp"
	"strings"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	defaultParameterMin = 100
	defaultParameterMax = 4000
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
func (g *Generator) scanForContainerResources(rm resmap.ResMap, list *corev1.List) error {
	crs := make([]*containerResources, 0, rm.Size())
	for _, sel := range g.ContainerResourcesSelector {
		// Select the matching resources
		resources, err := rm.Select(sel.selector())
		if err != nil {
			return err
		}

		for _, r := range resources {
			// Get the YAML tree representation of the resource
			node, err := filtersutil.GetRNode(r)
			if err != nil {
				return err
			}

			// Scan the document tree for information to add to the application resource
			cr := &containerResources{}
			if err := cr.saveTargetReference(node); err != nil {
				return err
			}
			if err := cr.saveResourcesPaths(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return err
			}
			if cr.empty() {
				continue
			}

			// Make sure we only get the newly discovered parts
			crs = mergeOrAppend(crs, cr)
		}
	}

	if len(crs) == 0 {
		return nil
	}

	// TODO We can probably be smarter determining if a prefix is necessary
	needsPrefix := len(crs) > 1

	exp := findOrAddExperiment(list)
	for _, cr := range crs {
		patch, err := cr.resourcesPatch(needsPrefix)
		if err != nil {
			return err
		}

		exp.Spec.Patches = append(exp.Spec.Patches, *patch)
		exp.Spec.Parameters = append(exp.Spec.Parameters, cr.resourcesParameters(needsPrefix)...)
	}

	return nil
}

// containerResources is an individual application resource that specifies container resources.
type containerResources struct {
	// targetRef is the reference to the resource
	targetRef corev1.ObjectReference
	// resourcesPaths are the YAML paths to the `resources` elements
	resourcesPaths [][]string
}

// Empty checks to see if this application resource has anything useful in it.
func (r *containerResources) empty() bool {
	return len(r.resourcesPaths) == 0
}

// SaveTargetReference updates the resource reference from the supplied document node.
func (r *containerResources) saveTargetReference(node *yaml.RNode) error {
	meta, err := node.GetMeta()
	if err != nil {
		return err
	}

	r.targetRef = corev1.ObjectReference{
		APIVersion: meta.APIVersion,
		Kind:       meta.Kind,
		Name:       meta.Name,
		Namespace:  meta.Namespace,
	}

	return nil
}

// SaveResourcesPaths extracts the paths the `resources` elements from the supplied node.
func (r *containerResources) saveResourcesPaths(node *yaml.RNode, sel ContainerResourcesSelector) error {
	path := sel.fieldSpec().PathSlice()
	return node.PipeE(
		yaml.Lookup(path...),
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			return nil, node.VisitElements(func(node *yaml.RNode) error {
				// TODO Capture existing resources for a baseline?
				if node.Field("resources") == nil && !sel.CreateIfNotPresent {
					return nil
				}

				name := node.Field("name").Value.YNode().Value
				if !sel.matchesContainerName(name) {
					return nil
				}

				r.resourcesPaths = append(r.resourcesPaths, append(path, "[name="+name+"]", "resources"))
				return nil
			})
		}))
}

// ResourcesParameters returns the parameters required for optimizing the discovered resources sections.
func (r *containerResources) resourcesParameters(includeTarget bool) []redskyv1beta1.Parameter {
	parameters := make([]redskyv1beta1.Parameter, 0, len(r.resourcesPaths)*2)
	for i := range r.resourcesPaths {
		parameters = append(parameters, redskyv1beta1.Parameter{
			Name: r.parameterName(i, "memory", includeTarget),
			Min:  defaultParameterMin,
			Max:  defaultParameterMax,
		}, redskyv1beta1.Parameter{
			Name: r.parameterName(i, "cpu", includeTarget),
			Min:  defaultParameterMin,
			Max:  defaultParameterMax,
		})
	}
	return parameters
}

// ResourcesPatch returns a patch for the discovered resources sections.
// TODO This should take a Go template for generating the parameter name
func (r *containerResources) resourcesPatch(includeTarget bool) (*redskyv1beta1.PatchTemplate, error) {
	// Create an empty patch
	patch := yaml.NewRNode(&yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode}},
	})

	for i := range r.resourcesPaths {
		// Construct limits/requests values
		memory := "{{ .Values." + r.parameterName(i, "memory", includeTarget) + " }}Mi"
		cpu := "{{ .Values." + r.parameterName(i, "cpu", includeTarget) + " }}m"
		values, err := yaml.NewRNode(&yaml.Node{Kind: yaml.MappingNode}).Pipe(
			yaml.Tee(yaml.SetField("memory", yaml.NewScalarRNode(memory))),
			yaml.Tee(yaml.SetField("cpu", yaml.NewScalarRNode(cpu))),
		)
		if err != nil {
			return nil, err
		}

		// Aggregate the limits/requests on the patch
		if err := patch.PipeE(
			&yaml.PathGetter{Path: r.resourcesPaths[i], Create: yaml.MappingNode},
			yaml.Tee(yaml.SetField("limits", values)),
			yaml.Tee(yaml.SetField("requests", values)),
		); err != nil {
			return nil, err
		}
	}

	// Render the patch and add it to the list of patches
	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(patch.Document()); err != nil {
		return nil, err
	}

	return &redskyv1beta1.PatchTemplate{
		Patch:     buf.String(),
		TargetRef: &r.targetRef,
	}, nil
}

// parameterName returns the name of the parameter to use for the resources path at the specified index.
func (r *containerResources) parameterName(i int, name string, includeTarget bool) string {
	var pn []string

	// Include the target kind and name
	if includeTarget {
		pn = append(pn, strings.ToLower(r.targetRef.Kind), r.targetRef.Name)
	}

	// Include the list index values (e.g. the container names)
	if len(r.resourcesPaths) > 1 {
		for _, p := range r.resourcesPaths[i] {
			if yaml.IsListIndex(p) {
				if _, value, _ := yaml.SplitIndexNameValue(p); value != "" {
					pn = append(pn, value)
				}
			}
		}
	}

	// Add the requested parameter name and join everything together
	pn = append(pn, name)
	return strings.Join(pn, "_")
}

// mergeOrAppend appends a container resources pointer unless it already exists, in which case the
// distinct resources paths are combined.
func mergeOrAppend(crs []*containerResources, cr *containerResources) []*containerResources {
	for _, r := range crs {
		if r.targetRef == cr.targetRef {
			for i := range cr.resourcesPaths {
				r.resourcesPaths = appendDistinctStringSlice(r.resourcesPaths, cr.resourcesPaths[i])
			}
			return crs
		}
	}

	return append(crs, cr)
}

// appendDistinctStringSlice is used for merging resources paths.
func appendDistinctStringSlice(s [][]string, ss []string) [][]string {
OUTER:
	for i := range s {
		if len(s[i]) != len(ss) {
			continue OUTER
		}

		for j := range s[i] {
			if s[i][j] != ss[j] {
				continue OUTER
			}
		}

		return s
	}

	return append(s, ss)
}
