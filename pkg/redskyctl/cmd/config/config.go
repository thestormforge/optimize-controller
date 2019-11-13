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
	"fmt"
	"os"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	configLong    = `Modify or view the Red Sky Ops configuration file`
	configExample = ``
)

// TODO managerEnvVar?
type ManagerEnvVar struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

func NewConfigCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Work with the configuration file",
		Long:    configLong,
		Example: configExample,
	}

	cmd.AddCommand(NewConfigViewCommand(f, ioStreams))
	cmd.AddCommand(NewConfigEnvCommand(f, ioStreams))
	cmd.AddCommand(NewConfigSetCommand(f, ioStreams))

	// By default, we want to run a view command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		cmdutil.CheckErr(NewConfigViewOptions(ioStreams).Run())
	}

	// Add the config file name to the help output
	cmd.Long = fmt.Sprintf("%s ('%s')", cmd.Long, os.ExpandEnv("$HOME/.redsky"))

	return cmd
}
