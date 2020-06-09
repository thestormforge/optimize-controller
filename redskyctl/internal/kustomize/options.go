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
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
)

type Option func(*Kustomize) error

const (
	defaultNamespace = "redsky-system"
	defaultImage     = "controller:latest"
)

// This will get overridden at build time with the appropriate version image.
var BuildImage = defaultImage

func defaultOptions() *Kustomize {
	fs := filesys.MakeFsInMemory()

	return &Kustomize{
		Base:       "/app/base",
		fs:         fs,
		Kustomizer: krusty.MakeKustomizer(fs, krusty.MakeDefaultOptions()),
		kustomize: &types.Kustomization{
			Namespace: defaultNamespace,
			Images: []types.Image{
				{
					Name:    defaultImage,
					NewName: strings.Split(BuildImage, ":")[0],
					NewTag:  strings.Split(BuildImage, ":")[1],
				},
			},
		},
	}
}

// WithNamespace sets the namespace attribute for the kustomization.
func WithNamespace(n string) Option {
	return func(k *Kustomize) error {
		k.kustomize.Namespace = n
		return nil
	}
}

// WithImage sets the image attribute for the kustomiztion.
func WithImage(i string) Option {
	return func(k *Kustomize) error {
		imageParts := strings.Split(i, ":")
		if len(imageParts) != 2 {
			return fmt.Errorf("invalid image specified %s", i)
		}

		k.kustomize.Images = append(k.kustomize.Images, types.Image{
			Name:    BuildImage,
			NewName: imageParts[0],
			NewTag:  imageParts[1],
		})
		return nil
	}
}

// WithLabels sets the common labels attribute for the kustomization.
func WithLabels(l map[string]string) Option {
	return func(k *Kustomize) error {
		// The schema for plugins are loosely defined, so we need to use a template
		labelTransformer := `
apiVersion: builtin
kind: LabelTransformer
metadata:
  name: metadata_labels
labels:
{{ range $label, $value := . }}
  {{ $label }}: {{ $value }}
{{ end }}
fieldSpecs:
  - kind: Deployment
    path: spec/template/metadata/labels
    create: true
  - path: metadata/labels
    create: true`

		t := template.Must(template.New("labelTransformer").Parse(labelTransformer))

		// Execute the template for each recipient.
		var b bytes.Buffer
		if err := t.Execute(&b, l); err != nil {
			return err
		}

		if err := k.fs.WriteFile(filepath.Join(k.Base, "labelTransformer.yaml"), b.Bytes()); err != nil {
			return err
		}

		k.kustomize.Transformers = append(k.kustomize.Transformers, "labelTransformer.yaml")

		return nil
	}
}
