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

package generate

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thestormforge/konjure/pkg/konjure"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	manifests "github.com/thestormforge/optimize-controller/v2/config"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type SampleOptions struct {
	Filter konjure.Filter
	Out    konjure.Writer
}

func NewSampleCommand(o *SampleOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "sample",
		Short:  "Generate samples",
		Long:   "Generate sample manifests for the custom resources",
		Hidden: true,

		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Out.Writer = cmd.OutOrStdout()
			o.Filter.DefaultReader = cmd.InOrStdin()
			o.Filter.WorkingDirectory, err = os.Getwd()
			return
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVar(&o.Filter.Kind, "kind", "", "resource `kind` of samples to emit")
	cmd.Flags().StringVar(&o.Filter.Name, "name", "", "restrict to `name`d samples")
	cmd.Flags().StringVarP(&o.Out.Format, "output", "o", "yaml", "output `format`; one of: yaml|json")

	return cmd
}

func (o *SampleOptions) generate() error {
	inputs, err := samples()
	if err != nil {
		return err
	}

	return kio.Pipeline{
		Inputs:  inputs,
		Filters: []kio.Filter{&o.Filter},
		Outputs: []kio.Writer{&o.Out},
	}.Execute()
}

// samples returns a KIO resource reader over the embedded sample resources.
func samples() ([]kio.Reader, error) {
	var samples []kio.Reader
	return samples, fs.WalkDir(manifests.Content, "samples", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}

		f, err := manifests.Content.Open(path)
		if err != nil {
			return err
		}

		samples = append(samples, &kio.ByteReader{Reader: f})
		return nil
	})
}
