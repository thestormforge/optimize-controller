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
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Scanner looks for resources that can be patched and adds them to an experiment.
type Scanner struct {
	// FileSystem to use when looking for resources, generally a pass through to the OS file system.
	FileSystem filesys.FileSystem
	// Resources representing the application to scan.
	Resources []string
	// ContainerResourcesSelector are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelector []ContainerResourcesSelector
}

// ScanInto scans the specified resource references and adds the necessary patches and parameter
// definitions to the supplied experiment.
func (s *Scanner) ScanInto(exp *redskyv1beta1.Experiment) error {
	// Load all of the resource references
	rm, err := s.load(s.Resources)
	if err != nil {
		return err
	}

	// We need to aggregate them all the container resources we can apply them
	crs, err := s.scanForContainerResources(rm)
	if err != nil {
		return err
	}

	// Apply the accumulated results to the experiment
	if err := s.applyContainerResources(crs, exp); err != nil {
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

func (s *Scanner) scanForContainerResources(rm resmap.ResMap) ([]*containerResources, error) {
	result := make([]*containerResources, 0, rm.Size())
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
			cr := &containerResources{}
			if err := cr.SaveTargetReference(node); err != nil {
				return nil, err
			}
			if err := cr.SaveResourcesPaths(node, sel); err != nil {
				// TODO Ignore errors if the resource doesn't have a matching resources path
				return nil, err
			}
			if cr.Empty() {
				continue
			}

			// Make sure we only get the newly discovered parts
			result = mergeOrAppend(result, cr)
		}
	}
	return result, nil
}

func (s *Scanner) applyContainerResources(crs []*containerResources, exp *redskyv1beta1.Experiment) error {
	// TODO We can probably be smarter determining if a prefix is necessary
	needsPrefix := len(crs) > 1

	for _, cr := range crs {
		patch, err := cr.ResourcesPatch(needsPrefix)
		if err != nil {
			return err
		}
		exp.Spec.Patches = append(exp.Spec.Patches, *patch)
		exp.Spec.Parameters = append(exp.Spec.Parameters, cr.ResourcesParameters(needsPrefix)...)
	}

	return nil
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
