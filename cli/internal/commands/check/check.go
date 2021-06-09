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

package check

import (
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// Options includes the configuration for the subcommands
type Options struct {
	// Config is the Optimize Configuration
	Config *config.OptimizeConfig
}

// NewCommand creates a new command for checking components
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run a consistency check",
		Long:  "Run a consistency check on StormForge Optimize components",
	}

	cmd.AddCommand(NewConfigCommand(&ConfigOptions{Config: o.Config}))
	cmd.AddCommand(NewExperimentCommand(&ExperimentOptions{}))
	cmd.AddCommand(NewVersionCommand(&VersionOptions{}))
	cmd.AddCommand(NewControllerCommand(&ControllerOptions{Config: o.Config}))

	return cmd
}
