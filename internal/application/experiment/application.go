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

	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/application/experiment/k8s"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// nameGen is a function that produces parameter names.
type nameGen func(ref *corev1.ObjectReference, path []string, name string) string

type resNameGen func(path []string, name string) string

// applicationResourceParameter is a tunable parameter of an application resources.
type applicationResourceParameter interface {
	patch(name resNameGen) (yaml.Filter, error)
	parameters(name resNameGen) ([]redskyv1beta1.Parameter, error)
}

type applicationResourceSelector interface {
	selector() types.Selector
	findParameters(node *yaml.RNode) ([]applicationResourceParameter, error)
}

// applicationResource is an individual application resource with one or more tunable parameters.
type applicationResource struct {
	// targetRef is the reference to the resource.
	targetRef corev1.ObjectReference
	// params is the list of parameters to tune on the resource.
	params []applicationResourceParameter
}

// anode is an application resource parameter value node in a YAML document.
type anode struct {
	// TODO Does having the target ref here help instead of having applicationResource at all?

	fieldPath []string
	value     *yaml.Node
}

// saveTargetReference updates the resource reference from the supplied document node.
func (r *applicationResource) saveTargetReference(node *yaml.RNode) error {
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

// patch returns a patch for the discovered resources sections.
func (r *applicationResource) patch(name nameGen) (*redskyv1beta1.PatchTemplate, error) {
	// Create an empty patch
	patch := yaml.NewRNode(&yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode}},
	})

	// Aggregate the filters and apply them to the patch node
	resName := func(p []string, n string) string { return name(&r.targetRef, p, n) }
	filters := make([]yaml.Filter, 0, len(r.params))
	for _, p := range r.params {
		f, err := p.patch(resName)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	if err := patch.PipeE(filters...); err != nil {
		return nil, err
	}

	// Render the patch and add it to the list of patches
	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(patch.Document()); err != nil {
		return nil, err
	}

	// Since the patch template doesn't need to be valid YAML we can cleanup tagged integers
	data := regexp.MustCompile(`!!int '(.*)'`).ReplaceAll(buf.Bytes(), []byte("$1"))

	return &redskyv1beta1.PatchTemplate{
		Patch:     string(data),
		TargetRef: &r.targetRef,
	}, nil
}

// parameters returns the experiment parameters for the resource.
func (r *applicationResource) parameters(name nameGen) ([]redskyv1beta1.Parameter, error) {
	// Just aggregate all of the individual parameters
	resName := func(p []string, n string) string { return name(&r.targetRef, p, n) }
	var result []redskyv1beta1.Parameter
	for _, p := range r.params {
		ps, err := p.parameters(resName)
		if err != nil {
			return nil, err
		}
		result = append(result, ps...)
	}

	return result, nil
}

func patchExperiment(ars []*applicationResource, list *corev1.List) error {
	if len(ars) == 0 {
		return nil
	}

	prefix := parameterNamePrefix(ars)
	exp := k8s.FindOrAddExperiment(list)
	for _, ar := range ars {
		// Create a parameter naming function that accounts for both objects and paths
		name := parameterName(prefix, ar)

		patch, err := ar.patch(name)
		if err != nil {
			return err
		}
		exp.Spec.Patches = append(exp.Spec.Patches, *patch)

		parameters, err := ar.parameters(name)
		if err != nil {
			return err
		}
		exp.Spec.Parameters = append(exp.Spec.Parameters, parameters...)
	}

	return nil
}

func scan(ars []*applicationResource, rm resmap.ResMap, sel applicationResourceSelector) ([]*applicationResource, error) {
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

		// Find the parameters
		p, err := sel.findParameters(node)
		if err != nil {
			return nil, err
		}
		if len(p) == 0 {
			continue
		}

		// Scan the document tree for information to add to the application resource
		ar := &applicationResource{params: p}
		if err := ar.saveTargetReference(node); err != nil {
			return nil, err
		}

		ars = mergeOrAppend(ars, ar)
	}

	return ars, nil
}

// mergeOrAppend appends an application resource pointer unless it already exists, in which case the
// distinct parts are combined.
func mergeOrAppend(ars []*applicationResource, ar *applicationResource) []*applicationResource {
	for _, r := range ars {
		if r.targetRef == ar.targetRef {
			r.params = append(r.params, ar.params...)
			return ars
		}
	}

	return append(ars, ar)
}

// parameterNamePrefix returns a name generator function that produces a prefix based
// on the distinct elements found in the object references of the supplied container resources.
func parameterNamePrefix(ars []*applicationResource) nameGen {
	// Index the object references by kind and name
	names := make(map[string]map[string]struct{})
	for _, ar := range ars {
		if ns := names[ar.targetRef.Kind]; ns == nil {
			names[ar.targetRef.Kind] = make(map[string]struct{})
		}
		names[ar.targetRef.Kind][ar.targetRef.Name] = struct{}{}
	}

	// Determine which prefixes we need
	// TODO This assumes overlapping names
	needsKind := len(names) > 1
	needsName := false
	for _, v := range names {
		needsName = needsName || len(v) > 1
	}

	return func(ref *corev1.ObjectReference, _ []string, _ string) string {
		var parts []string
		if needsKind {
			parts = append(parts, strings.ToLower(ref.Kind))
		}
		if needsName {
			parts = append(parts, strings.Split(ref.Name, "-")...)
		}
		return strings.Join(parts, "_")
	}
}

// parameterName returns a name generator function that produces a unique parameter
// name given a prefix (computed from a list of container resources) and the supplied
// container resources (which is assumed to have been considered when computing the prefix).
func parameterName(prefix nameGen, ar *applicationResource) nameGen {
	// Determine if the path is necessary
	needsPath := len(ar.params) > 1

	return func(ref *corev1.ObjectReference, path []string, name string) string {
		var parts []string

		// Only append the prefix if it is not blank
		if prefix != nil {
			if p := prefix(ref, path, name); p != "" {
				parts = append(parts, p)
			}
		}

		// Add the list index values (e.g. the container names)
		for _, p := range path {
			if needsPath && yaml.IsListIndex(p) {
				if _, value, _ := yaml.SplitIndexNameValue(p); value != "" {
					parts = append(parts, value) // TODO Split on "-" like we do for names?
				}
			}
		}

		// Join everything together using the requested parameter name at the end
		return strings.Join(append(parts, name), "_")
	}
}
