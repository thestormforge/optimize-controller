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
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// nameGen is a function that produces parameter names.
type nameGen func(ref *corev1.ObjectReference, path []string, name string) string

// applicationResource is an individual application resource that requires patching.
type applicationResource struct {
	// targetRef is the reference to the resource.
	targetRef corev1.ObjectReference

	// containerResourcesPaths are the YAML paths to the `resources` elements.
	containerResourcesPaths [][]string
	// containerResources are the actual `resources` found at the corresponding path index.
	containerResources []corev1.ResourceList

	// replicaPaths are the YAML paths to the `replicas` fields.
	replicaPaths [][]string
	// replicas are the actual replica values found at the corresponding path index.
	replicas []int32
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

	for i := range r.replicaPaths {
		replicas := "{{ .Values." + name(&r.targetRef, r.replicaPaths[i], "replicas") + " }}"
		value := yaml.NewScalarRNode(replicas)
		value.YNode().Tag = yaml.IntTag
		if err := patch.PipeE(
			&yaml.PathGetter{Path: r.replicaPaths[i], Create: yaml.ScalarNode},
			yaml.FieldSetter{Value: value, OverrideStyle: true},
		); err != nil {
			return nil, err
		}
	}

	for i := range r.containerResourcesPaths {
		// Construct limits/requests values
		memory := "{{ .Values." + name(&r.targetRef, r.containerResourcesPaths[i], "memory") + " }}M"
		cpu := "{{ .Values." + name(&r.targetRef, r.containerResourcesPaths[i], "cpu") + " }}m"
		values, err := yaml.NewRNode(&yaml.Node{Kind: yaml.MappingNode}).Pipe(
			yaml.Tee(yaml.SetField("memory", yaml.NewScalarRNode(memory))),
			yaml.Tee(yaml.SetField("cpu", yaml.NewScalarRNode(cpu))),
		)
		if err != nil {
			return nil, err
		}

		// Aggregate the limits/requests on the patch
		if err := patch.PipeE(
			&yaml.PathGetter{Path: r.containerResourcesPaths[i], Create: yaml.MappingNode},
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

	// Since the patch template doesn't need to be valid YAML we can cleanup tagged integers
	data := regexp.MustCompile(`!!int '(.*)'`).ReplaceAll(buf.Bytes(), []byte("$1"))

	return &redskyv1beta1.PatchTemplate{
		Patch:     string(data),
		TargetRef: &r.targetRef,
	}, nil
}

func patchExperiment(ars []*applicationResource, list *corev1.List) error {
	if len(ars) == 0 {
		return nil
	}

	prefix := parameterNamePrefix(ars)
	exp := findOrAddExperiment(list)
	for _, ar := range ars {
		// Create a parameter naming function that accounts for both objects and paths
		name := parameterName(prefix, ar)

		patch, err := ar.patch(name)
		if err != nil {
			return err
		}

		exp.Spec.Patches = append(exp.Spec.Patches, *patch)
		exp.Spec.Parameters = append(exp.Spec.Parameters, ar.containerResourcesParameters(name)...)
		exp.Spec.Parameters = append(exp.Spec.Parameters, ar.replicasParameters(name)...)
	}

	return nil
}

// mergeOrAppend appends an application resource pointer unless it already exists, in which case the
// distinct parts are combined.
func mergeOrAppend(ars []*applicationResource, ar *applicationResource) []*applicationResource {
	for _, r := range ars {
		if r.targetRef != ar.targetRef {
			continue
		}

		// Merge the container resources
		for i := range ar.containerResourcesPaths {
			if hasPath(r.containerResourcesPaths, ar.containerResourcesPaths[i]) {
				continue
			}

			r.containerResourcesPaths = append(r.containerResourcesPaths, ar.containerResourcesPaths[i])
			r.containerResources = append(r.containerResources, ar.containerResources[i])
		}

		// Merge the replicas
		for i := range ar.replicaPaths {
			if hasPath(r.replicaPaths, ar.replicaPaths[i]) {
				continue
			}

			r.replicaPaths = append(r.replicaPaths, ar.replicaPaths[i])
			r.replicas = append(r.replicas, ar.replicas[i])
		}

		return ars
	}

	return append(ars, ar)
}

func hasPath(paths [][]string, path []string) bool {
OUTER:
	for i := range paths {
		if len(paths[i]) != len(path) {
			continue OUTER
		}

		for j := range paths[i] {
			if paths[i][j] != path[j] {
				continue OUTER
			}
		}

		return true
	}

	return false
}

// parameterNamePrefix returns a name generator function that produces a prefix based
// on the distinct elements found in the object references of the supplied container resources.
func parameterNamePrefix(crs []*applicationResource) nameGen {
	// Index the object references by kind and name
	names := make(map[string]map[string]struct{})
	for _, cr := range crs {
		if ns := names[cr.targetRef.Kind]; ns == nil {
			names[cr.targetRef.Kind] = make(map[string]struct{})
		}
		names[cr.targetRef.Kind][cr.targetRef.Name] = struct{}{}
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
func parameterName(prefix nameGen, cr *applicationResource) nameGen {
	// Determine if the path is necessary
	needsPath := len(cr.containerResourcesPaths) > 1

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
