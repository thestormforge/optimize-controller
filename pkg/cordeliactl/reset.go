package cordeliactl

import (
	cmdutil "github.com/gramLabs/cordelia/pkg/cordeliactl/util"
	"github.com/spf13/cobra"
)

type resetOptions struct {
	kubeconfig string
}

func newResetOptions() *resetOptions {
	o := &resetOptions{}
	return o
}

func newResetCommand() *cobra.Command {
	o := newResetOptions()

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall Cordelia from a cluster",
		Long:  "The reset command will uninstall the Cordelia manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.run())
		},
	}

	cmdutil.KubeConfig(cmd, &o.kubeconfig)

	return cmd
}

func (o *resetOptions) run() error {
	// Just run init with the uninstall
	io := newInitOptions()
	io.kubeconfig = o.kubeconfig
	io.uninstall = true
	return io.run()
}
