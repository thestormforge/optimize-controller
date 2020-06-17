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
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/initialize"
	"github.com/spf13/cobra"
)

// Options includes the configuration for the subcommands
type Options struct {
	// Config is the Red Sky Configuration
	Config *config.RedSkyConfig
}

// NewCommand returns a new generate manifests command
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Red Sky Ops objects",
		Long:  "Generate Red Sky Ops object manifests",
	}

	cmd.AddCommand(NewRBACCommand(&RBACOptions{Config: o.Config, ClusterRole: true, ClusterRoleBinding: true}))
	cmd.AddCommand(NewTrialCommand(&TrialOptions{}))

	// Also include plumbing generators used by other commands
	cmd.AddCommand(authorize_cluster.NewGeneratorCommand(&authorize_cluster.GeneratorOptions{Config: o.Config}))
	cmd.AddCommand(grant_permissions.NewGeneratorCommand(&grant_permissions.GeneratorOptions{Config: o.Config}))
	cmd.AddCommand(initialize.NewGeneratorCommand(&initialize.GeneratorOptions{Config: o.Config}))

	return cmd
}
