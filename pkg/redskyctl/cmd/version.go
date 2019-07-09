package cmd

import (
	"fmt"

	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/gramLabs/redsky/pkg/version"
	"github.com/spf13/cobra"
)

// TODO Add support for getting Red Sky server version
// TODO Add support for getting manager version in cluster

type VersionOptions struct {
	root *cobra.Command

	cmdutil.IOStreams
}

func NewVersionOptions(ioStreams cmdutil.IOStreams) *VersionOptions {
	return &VersionOptions{
		IOStreams: ioStreams,
	}
}

func NewVersionCommand(ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewVersionOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Show the version information for Red Sky Control.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *VersionOptions) Complete(cmd *cobra.Command) error {
	o.root = cmd.Root()

	return nil
}

func (o *VersionOptions) Run() error {
	_, err := fmt.Fprintf(o.Out, "%s version: %s\n", o.root.Name(), version.GetVersion())
	return err
}
