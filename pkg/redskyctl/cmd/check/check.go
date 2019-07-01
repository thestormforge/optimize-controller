package check

import (
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

func NewCheckCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run a consistency check",
		Long:  "The check command provides the ability to run self check diagnostics.",
	}
	cmd.Run = cmd.HelpFunc()

	cmd.AddCommand(NewServerCheckCommand(f, ioStreams))

	// TODO Add local file based checks for validating experiment manifests?

	return cmd
}
