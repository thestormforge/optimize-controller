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

package config

import (
	"os"
	"path/filepath"

	client "github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	configLong    = `Modify or view the Red Sky Ops configuration file`
	configExample = ``
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
		Short:   "Work with the configuration file",
		Long:    configLong,
		Example: configExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.AddCommand(NewViewCommand(f, ioStreams))
	cmd.AddCommand(NewSetCommand(f, ioStreams))
	cmd.AddCommand(NewFixCommand(f, ioStreams))
	cmd.AddCommand(NewEnvCommand(f, ioStreams))

	return cmd
}

func (o *ConfigOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if cfg, err := f.ToClientConfig(false); err != nil {
		return err
	} else {
		o.Config = cfg
	}
	return nil
}
