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
	"strings"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	defaultParameterMin = 100
	defaultParameterMax = 4000
)

// Scanner looks for resources that can be patched and adds them to an experiment.
type Scanner struct {
	// FileSystem to use when looking for resources, generally a pass through to the OS file system.
	FileSystem filesys.FileSystem
	// ContainerResourcesSelector are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelector []ContainerResourcesSelector
}

// ScanInto scans the specified resource references and adds the necessary patches and parameter
// definitions to the supplied experiment.
func (s *Scanner) ScanInto(app *Application, exp *redskyv1beta1.Experiment) error {
	// Load all of the resource references
	rm, err := s.load(app.Resources)
	if err != nil {
		return err
	}

	// Iterate of the resources in the Kustomize resource map and scan each one
	rl, err := s.scan(rm)
	if err != nil {
		return err
	}

	// Apply the accumulated results to the experiment
	if err := s.apply(rl, exp); err != nil {
		return err
	}

	return nil
}

func (s *Scanner) load(resources []string) (resmap.ResMap, error) {
	// Get the current working directory so we can intercept requests for the Kustomization
	cwd, _, err := s.FileSystem.CleanedAbs(".")
	if err != nil {
		return nil, err
	}

	// Wrap the file system so it thinks the current directory is a kustomize root with our resources.
	// This is necessary to ensure that relative paths are resolved correctly and that files are not
	// treated like directories. If the current directory really is a kustomize root, that kustomization
	// will be hidden to prefer loading just the resources that are part of the experiment configuration.
	fSys := &kustomizationFileSystem{
		FileSystem:            s.FileSystem,
		KustomizationFileName: cwd.Join(konfig.DefaultKustomizationFileName()),
		Kustomization: types.Kustomization{
			Resources: resources,
		},
	}

	// Turn off the load restrictions so we can load arbitrary files (e.g. /dev/fd/...)
	o := krusty.MakeDefaultOptions()
	o.LoadRestrictions = types.LoadRestrictionsNone
	return krusty.MakeKustomizer(fSys, o).Run(".")
}

func (s *Scanner) scan(rm resmap.ResMap) ([]*applicationResource, error) {
	result := make([]*applicationResource, 0, rm.Size())
	for _, sel := range s.ContainerResourcesSelector {
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
			if err := ar.SaveTargetReference(node); err != nil {
				return nil, err
			}
			if err := ar.SaveResourcesPaths(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return nil, err
			}
			if ar.Empty() {
				continue
			}

			// TODO We need to deal with duplicates
			result = append(result, ar)
		}
	}
	return result, nil
}

func (s *Scanner) apply(list []*applicationResource, exp *redskyv1beta1.Experiment) error {
	// TODO We can probably be smarter determining if a prefix is necessary
	needsPrefix := len(list) > 1

	for _, r := range list {
		patch, err := r.ResourcesPatch(needsPrefix)
		if err != nil {
			return err
		}
		exp.Spec.Patches = append(exp.Spec.Patches, *patch)
		exp.Spec.Parameters = append(exp.Spec.Parameters, r.ResourcesParameters(needsPrefix)...)
	}

	return nil
}

// applicationResource is an individual resource that belongs to an application.
type applicationResource struct {
	// targetRef is the reference to the resource
	targetRef corev1.ObjectReference
	// resourcesPaths are the YAML paths to the `resources` elements in the resource
	resourcesPaths [][]string
}

// Empty checks to see if this application resource has anything useful in it
func (r *applicationResource) Empty() bool {
	return len(r.resourcesPaths) == 0
}

// SaveTargetReference updates the resource reference from the supplied document node.
func (r *applicationResource) SaveTargetReference(node *yaml.RNode) error {
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
func (r *applicationResource) SaveResourcesPaths(node *yaml.RNode, sel ContainerResourcesSelector) error {
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
				r.resourcesPaths = append(r.resourcesPaths, append(path, "[name="+name+"]", "resources"))
				return nil
			})
		}))
}

// ResourcesParameters returns the parameters required for optimizing the discovered resources sections.
func (r *applicationResource) ResourcesParameters(includeTarget bool) []redskyv1beta1.Parameter {
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
func (r *applicationResource) ResourcesPatch(includeTarget bool) (*redskyv1beta1.PatchTemplate, error) {
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
func (r *applicationResource) parameterName(i int, name string, includeTarget bool) string {
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

// kustomizationFileSystem is a wrapper around a real file system that injects a Kustomization at
// a pre-determined location. This has the effect of creating a kustomize root in memory even if
// there is no kustomization.yaml on disk.
type kustomizationFileSystem struct {
	filesys.FileSystem
	KustomizationFileName string
	Kustomization         types.Kustomization
}

func (fs *kustomizationFileSystem) ReadFile(path string) ([]byte, error) {
	if path == fs.KustomizationFileName {
		return yaml.Marshal(fs.Kustomization)
	}
	return fs.FileSystem.ReadFile(path)
}
