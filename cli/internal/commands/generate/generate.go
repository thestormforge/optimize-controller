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

package generate

import (
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/authorize_cluster"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/grant_permissions"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/initialize"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// Options includes the configuration for the subcommands
type Options struct {
	// Config is the Optimize Configuration
	Config *config.OptimizeConfig
}

// NewCommand returns a new generate manifests command
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "Generate Optimize resources",
		Long:    "Generate StormForge Optimize resource manifests",
	}

	cmd.AddCommand(initialize.NewGeneratorCommand(&initialize.GeneratorOptions{Config: o.Config}))
	cmd.AddCommand(authorize_cluster.NewGeneratorCommand(&authorize_cluster.GeneratorOptions{Config: o.Config}))
	cmd.AddCommand(grant_permissions.NewGeneratorCommand(&grant_permissions.GeneratorOptions{Config: o.Config}))
	cmd.AddCommand(NewRBACCommand(&RBACOptions{Config: o.Config, ClusterRole: true, ClusterRoleBinding: true}))
	cmd.AddCommand(NewApplicationCommand(&ApplicationOptions{Config: o.Config}))
	cmd.AddCommand(NewExperimentCommand(&ExperimentOptions{Config: o.Config}))
	cmd.AddCommand(NewTrialCommand(&TrialOptions{}))

	return cmd
}
