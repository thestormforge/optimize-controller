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
	"regexp"
	"strings"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"
)

// resourceLists represents the location of the resource lists for a specific resource.
type resourceLists struct {
	targetRef      corev1.ObjectReference
	resourcesPaths []fieldPath
}

// parameterName returns the name for the parameter used to tune the specified path.
func (p *resourceLists) parameterName(name string, path fieldPath, needsPrefix bool) string {
	var pn []string
	if needsPrefix {
		pn = append(pn, strings.ToLower(p.targetRef.Kind), p.targetRef.Name)
	}
	if len(p.resourcesPaths) > 1 {
		pn = append(pn, path.mergeKeyValues()...)
	}
	pn = append(pn, name)
	return strings.Join(pn, "_")
}

// generate returns the strategic merge patches and parameters the resource lists of a particular resource.
func (p *resourceLists) generate(needsPrefix bool) ([]redskyv1beta1.Parameter, *redskyv1beta1.PatchTemplate, error) {
	parameters := make([]redskyv1beta1.Parameter, 0, len(p.resourcesPaths)*2)
	patches := make([]strategicpatch.JSONMap, len(p.resourcesPaths))

	for i := range p.resourcesPaths {
		cpu := redskyv1beta1.Parameter{
			Name: p.parameterName("cpu", p.resourcesPaths[i], needsPrefix),
		}
		memory := redskyv1beta1.Parameter{
			Name: p.parameterName("memory", p.resourcesPaths[i], needsPrefix),
		}
		resourceList := map[string]interface{}{
			"cpu":    "{{ .Values." + cpu.Name + " }}m",
			"memory": "{{ .Values." + memory.Name + " }}Mi",
		}

		parameters = append(parameters, cpu, memory)
		patches[i] = p.resourcesPaths[i].build(map[string]interface{}{
			"requests": resourceList,
			"limits":   resourceList,
		})
	}

	// We will merge all of the individual patches into a single patch when we marshal it
	patch, err := marshalPatch(patches...)
	if err != nil {
		return nil, nil, err
	}

	return parameters, &redskyv1beta1.PatchTemplate{
		Patch:     patch,
		TargetRef: &p.targetRef,
	}, nil
}

// fieldPath represents a path to a particular field in a YAML document. Each path element represents a
// mapping key; however path elements can also correspond to a merge list if a key/value is placed
// in curly braces (e.g. "containers{name=postgres}" would identify the merge list "containers" with an
// element whose "name" merge key has a value of "postgres").
type fieldPath []string

var fieldPathRegexp = regexp.MustCompile(`([^{}]+)(:?{(.+)=(.+)})?`)

// String returns a string representation of the field path.
func (fp fieldPath) String() string {
	return strings.Join(fp, "/")
}

// build returns the minimal object representing the path to the supplied leaf value.
func (fp fieldPath) build(leaf interface{}) strategicpatch.JSONMap {
	if len(fp) == 0 {
		return nil
	}

	// Establish the root so we can return it at the end
	root := make(map[string]interface{}, 1)

	// Iterate through all but the last element of the path
	head := root
	for i := 0; i < len(fp)-1; i++ {
		m := make(map[string]interface{}, 1)

		// Check for a straight mapping vs. a merge list
		pp := fieldPathRegexp.FindStringSubmatch(fp[i])
		if pp[2] != "" {
			m[pp[3]] = pp[4]
			head[pp[1]] = []interface{}{m}
		} else {
			head[pp[1]] = m
		}

		// Move the head for the next iteration
		head = m
	}

	// Set the last element of the path to the leaf value
	pp := fieldPathRegexp.FindStringSubmatch(fp[len(fp)-1])
	// TODO We do not handle the `pp[2] != ""` case here
	head[pp[1]] = leaf

	return root
}

// read returns the value on the supplied object after following this path. It returns
// nil if the nothing is found.
func (fp fieldPath) read(from interface{}) interface{} {
	if len(fp) == 0 {
		return from
	}
	f, ok := from.(map[string]interface{})
	if !ok {
		return nil
	}

	pp := fieldPathRegexp.FindStringSubmatch(fp[0])
	if pp[2] == "" {
		return fp[1:].read(f[pp[1]])
	}

	l, ok := f[pp[1]].([]interface{})
	if !ok {
		return nil
	}

	if pp[4] == "*" {
		all := make([]interface{}, len(l))
		for i := range l {
			all[i] = fp[1:].read(l[i])
		}
		return all
	}

	for i := range l {
		if pp[4] == fieldPath(pp[3:4]).read(l[i]) {
			return fp[1:].read(l[i])
		}
	}
	return nil
}

// readInto performs a YAML serialization round trip on a value.
func (fp fieldPath) readInto(from, into interface{}) error {
	v := fp.read(from)
	if v == nil {
		return nil
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, into)
}

// mergeKeyValues returns only the values of the merge keys present in the field path. This is useful
// for disambiguation of similar paths.
func (fp fieldPath) mergeKeyValues() (values []string) {
	for _, p := range fp {
		if pp := fieldPathRegexp.FindStringSubmatch(p); pp[2] != "" {
			values = append(values, pp[4])
		}
	}
	return
}

// marshalPatch returns a YAML representation of the combined patches. This is useful so you can generate
// individual, per-container patches and combine them later.
func marshalPatch(p ...strategicpatch.JSONMap) (string, error) {
	// Merge all of the patches into one
	var patch strategicpatch.JSONMap
	for _, pp := range p {
		if patch == nil {
			patch = pp
		} else {
			patch = doMerge(map[string]interface{}(patch), map[string]interface{}(pp)).(map[string]interface{})
		}
	}

	// Marshal the YAML
	b, err := yaml.Marshal(patch)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func doMerge(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			return x1
		}
		for k, v2 := range x2 {
			if v1, ok := x1[k]; ok {
				x1[k] = doMerge(v1, v2)
			} else {
				x1[k] = v2
			}
		}
	case []interface{}:
		x2, ok := x2.([]interface{})
		if !ok {
			return x1
		}
		// TODO This is where we are doing dumb merge instead of strategic merge
		return append(x1, x2...)
	}
	return x1
}
