/*
Copyright 2020 GramLabs, Inc.

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

package configure

import (
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// Options includes the configuration for the subcommands
type Options struct {
	// Config is the Optimize Configuration
	Config *config.OptimizeConfig
}

// NewCommand creates a new configuration command
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Work with the configuration file",
		Long:  "Modify or view the StormForge Optimize Configuration file",
	}

	cmd.AddCommand(NewCurrentContextCommand(&CurrentContextOptions{Config: o.Config}))
	cmd.AddCommand(NewViewCommand(&ViewOptions{Config: o.Config}))
	cmd.AddCommand(NewSetCommand(&SetOptions{Config: o.Config}))
	cmd.AddCommand(NewEnvCommand(&EnvOptions{Config: o.Config}))

	// TODO It might be nice to have a "create-context" command for adding a context to the config file

	return cmd
}
