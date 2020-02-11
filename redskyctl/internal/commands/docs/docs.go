/*
Copyright 2019 GramLabs, Inc.

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

package docs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// TODO Add support for fetching Red Sky OpenAPI specification

// Options is the configuration for generating documentation
type Options struct {
	// Directory is the output directory for generated documentation
	Directory string
	// DocType is type of documentation to generate
	DocType string
	// SourcePath is the path to Kubernetes API sources
	SourcePath string
}

// NewCommand returns a new documentation command
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation",
		Long:   "Generate documentation for Red Sky Ops",
		Hidden: true,

		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run(cmd)
			commander.CheckErr(cmd, err)
		},
	}

	cmd.Flags().StringVarP(&o.Directory, "directory", "d", "./", "Directory where documentation is written.")
	cmd.Flags().StringVar(&o.DocType, "doc-type", "markdown", "Documentation type to write, one of: markdown|man|api.")
	cmd.Flags().StringVar(&o.SourcePath, "source", "pkg/apis/redsky/v1alpha1", "Source path used to find API types.")

	return cmd
}

// Run executes the documentation generation
func (o *Options) Run(cmd *cobra.Command) error {
	// Create the directory to write documentation into
	if err := os.MkdirAll(o.Directory, 0777); err != nil {
		return err
	}

	// Generate the requested type of documentation
	switch o.DocType {

	case "markdown", "md", "":
		if err := doc.GenMarkdownTree(cmd.Root(), o.Directory); err != nil {
			return err
		}

	case "man":
		if err := doc.GenManTree(cmd.Root(), &doc.GenManHeader{Title: "RED SKY", Section: "1"}, o.Directory); err != nil {
			return err
		}

	case "api":
		if err := genAPIDoc(o.Directory, "trial.md", filepath.Join(o.SourcePath, "trial_types.go")); err != nil {
			return err
		}
		if err := genAPIDoc(o.Directory, "experiment.md", filepath.Join(o.SourcePath, "experiment_types.go")); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown documentation type: %s", o.DocType)
	}

	return nil
}

func genAPIDoc(dir, basename, path string) error {
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	printAPIDocs(f, path)
	return nil
}
