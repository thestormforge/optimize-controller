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

package kustomize

import (
	"path/filepath"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Kustomize provides the ability to manage Kubernetes resources through
// kubernetes-sigs/Kustomize.
type Kustomize struct {
	Base string

	kustomize *types.Kustomization
	fs        filesys.FileSystem

	*krusty.Kustomizer
}

// NewKustomization instantiates a new kustomize workflow.
// Options are used to control setting specific parameters.
func NewKustomization(setters ...Option) (k *Kustomize, err error) {
	k = defaultOptions()

	// Update settings for kustomization
	for _, setter := range setters {
		if err = setter(k); err != nil {
			return k, err
		}
	}

	// Create Kustomization
	kustomizeYaml, err := yaml.Marshal(k.kustomize)
	if err != nil {
		return k, err
	}

	if err = k.fs.WriteFile(filepath.Join(k.Base, konfig.DefaultKustomizationFileName()), kustomizeYaml); err != nil {
		return k, err
	}

	return k, err
}

// Yamls is a convenience function to run through the kustomize workflow and
// return the generated yaml bytes from `kustomize build`.
func Yamls(setters ...Option) ([]byte, error) {
	kustom, err := NewKustomization(setters...)
	if err != nil {
		return nil, err
	}

	resources, err := kustom.Run(kustom.Base)
	if err != nil {
		return nil, err
	}

	return resources.AsYaml()
}
