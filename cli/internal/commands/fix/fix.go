/*
Copyright 2021 GramLabs, Inc.

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

package fix

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Options struct {
	commander.IOStreams
	Filenames []string
	InPlace   bool
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Fix manifests",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.Fix),
	}

	cmd.Flags().StringArrayVarP(&o.Filenames, "filename", "f", nil, "manifest `file` to fix")
	cmd.Flags().BoolVarP(&o.InPlace, "in-place", "i", false, "overwrite input files WITHOUT BACKUPS")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func (o *Options) Fix() error {
	p := kio.Pipeline{
		Filters: []kio.Filter{
			kio.FilterAll(&sfio.ExperimentMigrationFilter{}),
			kio.FilterAll(&sfio.MetadataMigrationFilter{}),
			filters.FormatFilter{},
		},
	}

	for _, filename := range o.Filenames {
		p.Inputs = append(p.Inputs, o.YAMLReader(filename))
	}

	if o.InPlace {
		p.Outputs = append(p.Outputs, kio.WriterFunc(o.writeBackToPathAnnotation))
	} else {
		p.Outputs = append(p.Outputs, o.YAMLWriter())
	}

	return p.Execute()
}

func (o *Options) writeBackToPathAnnotation(nodes []*yaml.RNode) error {
	// Note: we cannot use the kio.LocalPackageWriter because it assumes a common base directory

	if err := kioutil.DefaultPathAndIndexAnnotation("", nodes); err != nil {
		return err
	}

	pathIndex := make(map[string][]*yaml.RNode, len(nodes))
	for _, n := range nodes {
		if path, err := n.Pipe(yaml.GetAnnotation(kioutil.PathAnnotation)); err == nil {
			pathIndex[path.YNode().Value] = append(pathIndex[path.YNode().Value], n)
		}
	}
	for k := range pathIndex {
		_ = kioutil.SortNodes(pathIndex[k])
	}

	for k, v := range pathIndex {
		if err := o.writeToPath(k, v); err != nil {
			return err
		}
	}

	return nil
}

func (o *Options) writeToPath(path string, nodes []*yaml.RNode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := kio.ByteWriter{
		Writer:           f,
		ClearAnnotations: []string{kioutil.PathAnnotation},
	}
	return w.Write(nodes)
}
