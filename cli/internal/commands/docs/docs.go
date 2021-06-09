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

package docs

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
)

// TODO Add support for fetching StormForge Optimize API OpenAPI specification

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
		Long:   "Generate documentation for StormForge Optimize",
		Hidden: true,

		RunE: func(cmd *cobra.Command, _ []string) error { return o.docs(cmd) },
	}

	cmd.Flags().StringVarP(&o.Directory, "directory", "d", "./", "directory where documentation is written")
	cmd.Flags().StringVar(&o.DocType, "doc-type", "markdown", "documentation type to write")
	cmd.Flags().StringVar(&o.SourcePath, "source", "", "source path used to find API types")

	_ = cmd.MarkFlagDirname("directory")
	_ = cmd.MarkFlagDirname("source")

	commander.SetFlagValues(cmd, "doc-type", "markdown", "man", "api")

	return cmd
}

func (o *Options) docs(cmd *cobra.Command) error {
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
		if err := doc.GenManTree(cmd.Root(), &doc.GenManHeader{Title: "STORMFORGE OPTIMIZE", Section: "1"}, o.Directory); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown documentation type: %s", o.DocType)
	}

	return nil
}
