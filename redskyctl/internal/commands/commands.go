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
	"fmt"
	"os"
	"strings"

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
	"github.com/redskyops/redskyops-go/pkg/config"
	experimentsv1alpha1 "github.com/redskyops/redskyops-go/pkg/redskyapi/experiments/v1alpha1"
	"github.com/spf13/cobra"
)

// NewRedskyctlCommand creates a new top-level redskyctl command
func NewRedskyctlCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "redskyctl",
		Short:             "Kubernetes Exploration",
		DisableAutoGenTag: true,
		SilenceUsage:      true,
	}

	// Create a global configuration
	cfg := &config.RedSkyConfig{}
	commander.ConfigGlobals(cfg, rootCmd)

	// Establish OAuth client identity
	cfg.ClientIdentity = authorizationIdentity

	// Kubernetes Commands
	rootCmd.AddCommand(initialize.NewCommand(&initialize.Options{GeneratorOptions: initialize.GeneratorOptions{Config: cfg, IncludeBootstrapRole: true}}))
	rootCmd.AddCommand(reset.NewCommand(&reset.Options{Config: cfg}))
	rootCmd.AddCommand(grant_permissions.NewCommand(&grant_permissions.Options{GeneratorOptions: grant_permissions.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(authorize_cluster.NewCommand(&authorize_cluster.Options{GeneratorOptions: authorize_cluster.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(generate.NewCommand(&generate.Options{Config: cfg}))

	// Remove Server Commands
	rootCmd.AddCommand(experiments.NewDeleteCommand(&experiments.DeleteOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewGetCommand(&experiments.GetOptions{Options: experiments.Options{Config: cfg}, ChunkSize: 500}))
	rootCmd.AddCommand(experiments.NewLabelCommand(&experiments.LabelOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewSuggestCommand(&experiments.SuggestOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(results.NewCommand(&results.Options{Config: cfg}))

	// Administrative Commands
	rootCmd.AddCommand(login.NewCommand(&login.Options{Config: cfg}))
	rootCmd.AddCommand(revoke.NewCommand(&revoke.Options{Config: cfg}))
	rootCmd.AddCommand(configure.NewCommand(&configure.Options{Config: cfg}))
	rootCmd.AddCommand(check.NewCommand(&check.Options{Config: cfg}))
	rootCmd.AddCommand(completion.NewCommand(&completion.Options{}))
	rootCmd.AddCommand(kustomize.NewCommand())
	rootCmd.AddCommand(version.NewCommand(&version.Options{Config: cfg}))
	rootCmd.AddCommand(docs.NewCommand(&docs.Options{}))

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Add a "trial cleanup" command to run setup tasks (perhaps remove labels from standard setupJob)
	// TODO Some kind of debug tool to evaluate metric queries
	// TODO The "get" functionality needs to support templating so you can extract assignments for downstream use

	commander.MapErrors(rootCmd, mapError)
	return rootCmd
}

// mapError intercepts errors returned by commands before they are reported.
func mapError(err error) error {
	if experimentsv1alpha1.IsUnauthorized(err) {
		// Trust the error message we get from the experiments API
		if _, ok := err.(*experimentsv1alpha1.Error); ok {
			return fmt.Errorf("%w, try running 'redskyctl login'", err)
		}
		return fmt.Errorf("unauthorized, try running 'redskyctl login'")
	}

	return err
}

// authorizationIdentity returns the client identifier to use for a given authorization server (identified by it's issuer URI)
func authorizationIdentity(issuer string) string {
	switch issuer {
	case "https://auth.carbonrelay.io/", "https://carbonrelay.auth0.com/":
		return "pE3kMKdrMTdW4DOxQHesyAuFGNOWaEke"
	case "https://auth.carbonrelay.dev/", "https://carbonrelay-dev.auth0.com/":
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
