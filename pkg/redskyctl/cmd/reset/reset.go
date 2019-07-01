package reset

import (
	"github.com/gramLabs/redsky/pkg/redskyctl/cmd/init"
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

type ResetOptions struct {
	// Really we just hide an init call in here
	initOptions *init.InitOptions
}

func NewResetOptions(ioStreams cmdutil.IOStreams) *ResetOptions {
	return &ResetOptions{
		initOptions: init.NewInitOptions(ioStreams),
	}
}

func NewResetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewResetOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall Red Sky from a cluster",
		Long:  "The reset command will uninstall the Red Sky manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ResetOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	return o.initOptions.Complete(f, cmd)
}

func (o *ResetOptions) Run() error {
	return o.initOptions.Run()
}
