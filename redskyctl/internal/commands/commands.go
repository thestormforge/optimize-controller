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

package commands

import (
	"os"

	configuration "github.com/redskyops/k8s-experiment/internal/config"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/check"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/config"
	deleteCmd "github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/delete"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/generate"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/get"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/kustomize"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/results"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/setup"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/suggest"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commander"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commands/docs"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commands/login"
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
	cfg := &configuration.RedSkyConfig{}
	commander.ConfigGlobals(cfg, rootCmd)

	// Add the sub-commands
	rootCmd.AddCommand(docs.NewCommand(&docs.Options{}))
	rootCmd.AddCommand(login.NewCommand(&login.Options{Config: cfg}))

	// Compatibility mode: these commands need to be migrated to use the new style
	addUnmigratedCommands(rootCmd)

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Add a "trial cleanup" command to run setup tasks (perhaps remove labels from standard setupJob)
	// TODO Some kind of debug tool to evaluate metric queries
	// TODO The "get" functionality needs to support templating so you can extract assignments for downstream use

	return rootCmd
}

func addUnmigratedCommands(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	kubeConfigFlags := util.NewConfigFlags()
	kubeConfigFlags.AddFlags(flags)
	redskyConfigFlags := util.NewServerFlags()
	redskyConfigFlags.AddFlags(flags)
	f := util.NewFactory(kubeConfigFlags, redskyConfigFlags)
	ioStreams := util.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	rootCmd.AddCommand(cmd.NewVersionCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewInitCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewResetCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewAuthorizeCommand(f, ioStreams))
	rootCmd.AddCommand(kustomize.NewKustomizeCommand(f, ioStreams))
	rootCmd.AddCommand(config.NewConfigCommand(f, ioStreams))
	rootCmd.AddCommand(check.NewCheckCommand(f, ioStreams))
	rootCmd.AddCommand(suggest.NewSuggestCommand(f, ioStreams))
	rootCmd.AddCommand(generate.NewGenerateCommand(f, ioStreams))
	rootCmd.AddCommand(get.NewGetCommand(f, ioStreams))
	rootCmd.AddCommand(deleteCmd.NewDeleteCommand(f, ioStreams))
	rootCmd.AddCommand(results.NewResultsCommand(f, ioStreams))
}
