package config

import (
	"fmt"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	viewLong    = `TODO`
	viewExample = `TODO`
)

func NewViewCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runView

	cmd := &cobra.Command{
		Use:     "view",
		Short:   "TODO",
		Long:    viewLong,
		Example: viewExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigOptions) runView() error {
	output, err := yaml.Marshal(o.Config)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(o.Out, string(output))
	return err
}
