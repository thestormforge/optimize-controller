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

package commands

import (
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/check"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/completion"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/configure"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/docs"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/initialize"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/kustomize"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/login"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/reset"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/results"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/revoke"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/version"
	"github.com/spf13/cobra"
)

// NewRedskyctlCommand creates a new top-level redskyctl command
func NewRedskyctlCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "redskyctl",
		Short:             "Kubernetes Exploration",
		DisableAutoGenTag: true,
	}

	// By default just run the help
	rootCmd.Run = rootCmd.HelpFunc()

	// Create a global configuration
	cfg := &config.RedSkyConfig{}
	commander.ConfigGlobals(cfg, rootCmd)

	// Add the sub-commands
	rootCmd.AddCommand(authorize_cluster.NewCommand(&authorize_cluster.Options{GeneratorOptions: authorize_cluster.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(check.NewCommand(&check.Options{Config: cfg}))
	rootCmd.AddCommand(completion.NewCommand(&completion.Options{}))
	rootCmd.AddCommand(configure.NewCommand(&configure.Options{Config: cfg}))
	rootCmd.AddCommand(docs.NewCommand(&docs.Options{}))
	rootCmd.AddCommand(experiments.NewDeleteCommand(&experiments.DeleteOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewGetCommand(&experiments.GetOptions{Options: experiments.Options{Config: cfg}, ChunkSize: 500}))
	rootCmd.AddCommand(experiments.NewLabelCommand(&experiments.LabelOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewSuggestCommand(&experiments.SuggestOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(generate.NewCommand(&generate.Options{Config: cfg}))
	rootCmd.AddCommand(grant_permissions.NewCommand(&grant_permissions.Options{GeneratorOptions: grant_permissions.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(initialize.NewCommand(&initialize.Options{GeneratorOptions: initialize.GeneratorOptions{Config: cfg}, IncludeBootstrapRole: true}))
	rootCmd.AddCommand(kustomize.NewCommand())
	rootCmd.AddCommand(login.NewCommand(&login.Options{Config: cfg}))
	rootCmd.AddCommand(reset.NewCommand(&reset.Options{Config: cfg}))
	rootCmd.AddCommand(results.NewCommand(&results.Options{Config: cfg}))
	rootCmd.AddCommand(revoke.NewCommand(&revoke.Options{Config: cfg}))
	rootCmd.AddCommand(version.NewCommand(&version.Options{Config: cfg}))

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Add a "trial cleanup" command to run setup tasks (perhaps remove labels from standard setupJob)
	// TODO Some kind of debug tool to evaluate metric queries
	// TODO The "get" functionality needs to support templating so you can extract assignments for downstream use

	return rootCmd
}
