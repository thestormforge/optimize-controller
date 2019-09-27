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

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// TODO Add support for fetching Red Sky OpenAPI specification

const (
	docsLong    = `Generate documentation for Red Sky Ops`
	docsExample = ``
)

// DocsOptions is the configuration for generating documentation
type DocsOptions struct {
	Directory  string
	DocType    string
	SourcePath string

	root *cobra.Command

	cmdutil.IOStreams
}

// NewDocsOptions returns a new documentation options struct
func NewDocsOptions(ioStreams cmdutil.IOStreams) *DocsOptions {
	return &DocsOptions{
		IOStreams: ioStreams,
	}
}

// NewDocsCommand returns a new documentation command
func NewDocsCommand(ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewDocsOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "docs",
		Short:   "Generate documentation",
		Long:    docsLong,
		Example: docsExample,
		Hidden:  true,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Directory, "directory", "d", "./", "Directory where documentation is written.")
	cmd.Flags().StringVar(&o.DocType, "doc-type", "markdown", "Documentation type to write, one of: markdown|man|api.")
	cmd.Flags().StringVar(&o.SourcePath, "source", "pkg/apis/redsky/v1alpha1", "Source path used to find API types.")

	return cmd
}

// Complete the documentation options
func (o *DocsOptions) Complete(cmd *cobra.Command) error {
	if err := os.MkdirAll(o.Directory, 0777); err != nil {
		return err
	}

	o.root = cmd.Root()
	o.root.DisableAutoGenTag = true

	return nil
}

// Run the documentation options
func (o *DocsOptions) Run() error {
	switch o.DocType {
	case "markdown", "md", "":
		return doc.GenMarkdownTree(o.root, o.Directory)
	case "man":
		return doc.GenManTree(o.root, &doc.GenManHeader{Title: "RED SKY", Section: "1"}, o.Directory)
	case "api":
		return GenAPIDocs(o.SourcePath, o.Directory)
	default:
		return fmt.Errorf("unknown documentation type: %s", o.DocType)
	}
}

// GenAPIDocs reads the trial and experiment source code from source path and outputs Markdown to the output directory
func GenAPIDocs(sourcePath, dir string) error {
	if err := genAPIDoc(dir, "trial.md", filepath.Join(sourcePath, "trial_types.go")); err != nil {
		return err
	}
	if err := genAPIDoc(dir, "experiment.md", filepath.Join(sourcePath, "experiment_types.go")); err != nil {
		return err
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
