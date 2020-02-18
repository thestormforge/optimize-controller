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
	"strings"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/check"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/generate"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/get"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/kustomize"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/setup"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/cmd/suggest"
	"github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/configuration"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/deletion"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/docs"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/login"
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

	// Establish OAuth client identity
	cfg.ClientIdentity = authorizationIdentity

	// Add the sub-commands
	rootCmd.AddCommand(configuration.NewCommand(&configuration.Options{Config: cfg}))
	rootCmd.AddCommand(deletion.NewCommand(&deletion.Options{Config: cfg}))
	rootCmd.AddCommand(docs.NewCommand(&docs.Options{}))
	rootCmd.AddCommand(login.NewCommand(&login.Options{Config: cfg}))
	rootCmd.AddCommand(results.NewCommand(&results.Options{Config: cfg}))
	rootCmd.AddCommand(revoke.NewCommand(&revoke.Options{Config: cfg}))
	rootCmd.AddCommand(version.NewCommand(&version.Options{Config: cfg}))

	// Compatibility mode: these commands need to be migrated to use the new style
	addUnmigratedCommands(rootCmd, cfg)

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Add a "trial cleanup" command to run setup tasks (perhaps remove labels from standard setupJob)
	// TODO Some kind of debug tool to evaluate metric queries
	// TODO The "get" functionality needs to support templating so you can extract assignments for downstream use

	return rootCmd
}

func addUnmigratedCommands(rootCmd *cobra.Command, cfg *config.RedSkyConfig) {
	flags := rootCmd.PersistentFlags()
	configFlags := util.NewConfigFlags(cfg)
	configFlags.AddFlags(flags)
	f := util.NewFactory(configFlags)
	ioStreams := util.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	rootCmd.AddCommand(setup.NewInitCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewResetCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewAuthorizeCommand(f, ioStreams))
	rootCmd.AddCommand(kustomize.NewKustomizeCommand(f, ioStreams))
	rootCmd.AddCommand(check.NewCheckCommand(f, ioStreams))
	rootCmd.AddCommand(suggest.NewSuggestCommand(f, ioStreams))
	rootCmd.AddCommand(generate.NewGenerateCommand(f, ioStreams))
	rootCmd.AddCommand(get.NewGetCommand(f, ioStreams))
}

// authorizationIdentity returns the client identifier to use for a given authorization server (identified by it's issuer URI)
func authorizationIdentity(issuer string) string {
	switch issuer {
	case "https://auth.carbonrelay.io/":
		return "pE3kMKdrMTdW4DOxQHesyAuFGNOWaEke"
	case "https://carbonrelay-dev.auth0.com/":
		return "fmbRPm2zoQJ64hb37CUJDJVmRLHhE04Y"
	default:
		// OAuth specifications warning against mix-ups, instead of using a fixed environment variable name, the name
		// should be derived from the issuer: this helps ensure we do not send the client identifier to the wrong server.

		// PRECONDITION: issuer identifiers must be https:// URIs with no query or fragment
		prefix := strings.ReplaceAll(strings.TrimPrefix(issuer, "https://"), "//", "/")
		prefix = strings.ReplaceAll(strings.TrimRight(prefix, "/"), "/", "//") + "/"
		prefix = strings.Map(func(r rune) rune {
			switch {
			case r >= 'A' && r <= 'Z':
				return r
			case r == '.' || r == '/':
				return '_'
			}
			return -1
		}, strings.ToUpper(prefix))

		return os.Getenv(prefix + "CLIENT_ID")
	}
}
