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
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/internal/sfio"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
)

type Options struct {
	commander.IOStreams
	Filenames []string
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Fix manifests",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.Fix),
	}

	cmd.Flags().StringArrayVarP(&o.Filenames, "filename", "f", nil, "manifest `file` to fix")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func (o *Options) Fix() error {
	p := kio.Pipeline{
		Filters: []kio.Filter{
			kio.FilterAll(&sfio.ExperimentMigrationFilter{}),
			filters.FormatFilter{},
		},
		Outputs: []kio.Writer{o.YAMLWriter()},
	}

	for _, filename := range o.Filenames {
		p.Inputs = append(p.Inputs, o.YAMLReader(filename))
	}

	return p.Execute()
}
