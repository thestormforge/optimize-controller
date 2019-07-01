package cmd

import (
	"io"
	"os"

	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/init"
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/reset"
	"github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/gramLabs/redsky/pkg/version"
	"github.com/spf13/cobra"
)

func NewDefaultRedskyctlCommand() *cobra.Command {
	return NewKubectlCommand(os.Stdin, os.Stdout, os.Stderr)
}

func NewKubectlCommand(in io.Reader, out, err io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "redskyctl",
		Version: version.GetVersion(),
	}
	rootCmd.Run = rootCmd.HelpFunc()

	flags := rootCmd.PersistentFlags()

	kubeConfigFlags := util.NewConfigFlags()
	kubeConfigFlags.AddFlags(flags)

	redskyConfigFlags := util.NewServerFlags()
	redskyConfigFlags.AddFlags(flags)

	f := util.NewFactory(kubeConfigFlags, redskyConfigFlags)

	ioStreams := util.IOStreams{In: in, Out: out, ErrOut: err}

	rootCmd.AddCommand(init.NewInitCommand(f, ioStreams))
	rootCmd.AddCommand(reset.NewResetCommand(f, ioStreams))

	// TODO Add a 'suggest' command to generate suggestions (either remotely or in cluster)
	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO Add API client commands for interacting with a remote server
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Should the "create experiment" wizard be a Kustomize plugin?

	return nil
}
