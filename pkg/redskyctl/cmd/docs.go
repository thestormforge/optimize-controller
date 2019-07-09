package cmd

import (
	"fmt"
	"os"

	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// TODO Add support for fetching Red Sky OpenAPI specification
// TODO Add support for generating OpenAPI specification based on Kube API (including validation schema)

const (
	docsLong    = ``
	docsExample = ``
)

type DocsOptions struct {
	Directory string
	DocType   string

	root *cobra.Command

	cmdutil.IOStreams
}

func NewDocsOptions(ioStreams cmdutil.IOStreams) *DocsOptions {
	return &DocsOptions{
		IOStreams: ioStreams,
	}
}

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

	cmd.Flags().StringVarP(&o.Directory, "directory", "d", "./", "Directory where documentation is written")

	return cmd
}

func (o *DocsOptions) Complete(cmd *cobra.Command) error {
	if err := os.MkdirAll(o.Directory, 0777); err != nil {
		return err
	}

	o.root = cmd.Root()

	return nil
}

func (o *DocsOptions) Run() error {
	switch o.DocType {
	case "markdown", "md", "":
		return doc.GenMarkdownTree(o.root, o.Directory)
	case "man":
		return doc.GenManTree(o.root, &doc.GenManHeader{Title: "RED SKY", Section: "1"}, o.Directory)
	default:
		return fmt.Errorf("unknown documentation type: %s", o.DocType)
	}
}
