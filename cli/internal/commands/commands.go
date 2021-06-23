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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/authorize_cluster"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/check"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/completion"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/configure"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/debug"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/docs"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/experiments"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/export"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/fix"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/generate"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/grant_permissions"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/initialize"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/login"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/ping"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/reset"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/revoke"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/version"
	"github.com/thestormforge/optimize-go/pkg/api"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// NewRootCommand creates a new top-level command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "stormforge",
		Short:             "Release with Confidence",
		DisableAutoGenTag: true,
		SilenceUsage:      true,
	}

	// Create a global configuration
	cfg := &config.OptimizeConfig{}
	commander.ConfigGlobals(cfg, rootCmd)

	// Establish OAuth client identity
	cfg.ClientIdentity = authorizationIdentity
	cfg.AuthorizationParameters = map[string][]string{
		"audience": {"https://api.carbonrelay.io/v1/"},
	}

	// Kubernetes Commands
	rootCmd.AddCommand(initialize.NewCommand(&initialize.Options{GeneratorOptions: initialize.GeneratorOptions{Config: cfg, IncludeBootstrapRole: true}}))
	rootCmd.AddCommand(reset.NewCommand(&reset.Options{Config: cfg}))
	rootCmd.AddCommand(grant_permissions.NewCommand(&grant_permissions.Options{GeneratorOptions: grant_permissions.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(authorize_cluster.NewCommand(&authorize_cluster.Options{GeneratorOptions: authorize_cluster.GeneratorOptions{Config: cfg}}))
	rootCmd.AddCommand(generate.NewCommand(&generate.Options{Config: cfg}))
	rootCmd.AddCommand(fix.NewCommand(&fix.Options{}))
	rootCmd.AddCommand(export.NewCommand(&export.Options{Config: cfg}))
	rootCmd.AddCommand(run.NewCommand(&run.Options{Config: cfg}))

	// Remote Server Commands
	rootCmd.AddCommand(experiments.NewDeleteCommand(&experiments.DeleteOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewGetCommand(&experiments.GetOptions{Options: experiments.Options{Config: cfg}, ChunkSize: 500}))
	rootCmd.AddCommand(experiments.NewLabelCommand(&experiments.LabelOptions{Options: experiments.Options{Config: cfg}}))
	rootCmd.AddCommand(experiments.NewSuggestCommand(&experiments.SuggestOptions{Options: experiments.Options{Config: cfg}}))

	// Administrative Commands
	rootCmd.AddCommand(login.NewCommand(&login.Options{Config: cfg}))
	rootCmd.AddCommand(revoke.NewCommand(&revoke.Options{Config: cfg}))
	rootCmd.AddCommand(ping.NewCommand(&ping.Options{Config: cfg}))
	rootCmd.AddCommand(configure.NewCommand(&configure.Options{Config: cfg}))
	rootCmd.AddCommand(check.NewCommand(&check.Options{Config: cfg}))
	rootCmd.AddCommand(completion.NewCommand(&completion.Options{}))
	rootCmd.AddCommand(version.NewCommand(&version.Options{Config: cfg}))
	rootCmd.AddCommand(docs.NewCommand(&docs.Options{}))
	rootCmd.AddCommand(debug.NewCommand(&debug.Options{Config: cfg}))

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier
	// TODO Add a "trial cleanup" command to run setup tasks (perhaps remove labels from standard setupJob)
	// TODO The "get" functionality needs to support templating so you can extract assignments for downstream use

	commander.MapErrors(rootCmd, mapError)
	return rootCmd
}

// mapError intercepts errors returned by commands before they are reported.
func mapError(err error) error {
	if api.IsUnauthorized(err) {
		// Trust the error message we get from the experiments API
		if _, ok := err.(*api.Error); ok {
			return fmt.Errorf("%w, try running 'stormforge login'", err)
		}
		return fmt.Errorf("unauthorized, try running 'stormforge login'")
	}

	// It's really annoying to just get an "exit status was one" message.
	var e *exec.ExitError
	if errors.As(err, &e) && !e.Success() && len(e.Stderr) > 0 {
		return fmt.Errorf("%w\n%s", err, string(e.Stderr))
	}

	return err
}

// authorizationIdentity returns the client identifier to use for a given authorization server (identified by it's issuer URI)
func authorizationIdentity(issuer string) string {
	switch issuer {
	case "https://auth.stormforge.io/", "https://auth.carbonrelay.io/", "https://carbonrelay.auth0.com/":
		return "pE3kMKdrMTdW4DOxQHesyAuFGNOWaEke"
	case "https://auth.stormforge.dev/", "https://auth.carbonrelay.dev/", "https://carbonrelay-dev.auth0.com/":
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
