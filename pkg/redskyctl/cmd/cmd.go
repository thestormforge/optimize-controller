package cmd

import (
	"io"
	"os"

	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/check"
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/generate"
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/get"
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/setup"
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/suggest"
	"github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

func NewDefaultRedskyctlCommand() *cobra.Command {
	return NewRedskyctlCommand(os.Stdin, os.Stdout, os.Stderr)
}

func NewRedskyctlCommand(in io.Reader, out, err io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "redskyctl",
	}
	rootCmd.Run = rootCmd.HelpFunc()

	flags := rootCmd.PersistentFlags()

	kubeConfigFlags := util.NewConfigFlags()
	kubeConfigFlags.AddFlags(flags)

	redskyConfigFlags := util.NewServerFlags()
	redskyConfigFlags.AddFlags(flags)

	f := util.NewFactory(kubeConfigFlags, redskyConfigFlags)

	ioStreams := util.IOStreams{In: in, Out: out, ErrOut: err}

	rootCmd.AddCommand(NewDocsCommand(ioStreams))
	rootCmd.AddCommand(NewVersionCommand(ioStreams))

	rootCmd.AddCommand(setup.NewInitCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewResetCommand(f, ioStreams))
	rootCmd.AddCommand(check.NewCheckCommand(f, ioStreams))
	rootCmd.AddCommand(suggest.NewSuggestCommand(f, ioStreams))
	rootCmd.AddCommand(generate.NewGenerateCommand(f, ioStreams))
	rootCmd.AddCommand(get.NewGetCommand(f, ioStreams))

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO Add API client commands for interacting with a remote server
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Should the "create experiment" wizard be a Kustomize plugin?

	return rootCmd
}

// NewDefaultCommand is used for creating commands from the standard NewXCommand functions
func NewDefaultCommand(cmd func(f util.Factory, ioStreams util.IOStreams) *cobra.Command) *cobra.Command {
	ioStreams := util.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	// TODO Make a dummy implementation that returns reasonable errors
	var f util.Factory
	return cmd(f, ioStreams)
}
