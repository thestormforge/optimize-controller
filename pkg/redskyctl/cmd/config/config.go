package config

import (
	"os"
	"path/filepath"

	client "github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	configLong    = `TODO`
	configExample = `TODO`
)

type ConfigOptions struct {
	ConfigFile string
	Config     *client.Config

	Source map[string]string

	Run func() error
	cmdutil.IOStreams
}

func NewConfigOptions(ioStreams cmdutil.IOStreams) *ConfigOptions {
	o := &ConfigOptions{IOStreams: ioStreams}
	o.ConfigFile = ".redsky"

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home != "" {
		o.ConfigFile = filepath.Join(home, o.ConfigFile)
	}

	return o
}

func NewConfigCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)

	// By default, perform a "view"
	o.Run = o.runView

	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Work with configuration files",
		Long:    configLong,
		Example: configExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.AddCommand(NewViewCommand(f, ioStreams))
	cmd.AddCommand(NewSetCommand(f, ioStreams))

	// TODO Have a "fix" command to make old configs into current configs

	return cmd
}

func (o *ConfigOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if cfg, err := f.ToClientConfig(false); err != nil {
		return err
	} else {
		o.Config = cfg
	}
	return nil
}
