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

package commander

import (
	"context"
	"io"
	"net/http"

	internalconfig "github.com/redskyops/redskyops-controller/internal/config"
	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	"github.com/redskyops/redskyops-controller/pkg/version"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// TODO Terminal type for commands whose produce character level interactions (as opposed to byte level implied by the direct streams)
// TODO Should we add methods like "IOStreams.OutOrStdout()" so we can use the zero IOStreams value?
// TODO Have functionality for setting up IOStreams with os/exec
// TODO Have functionality for setting up IOStreams with stdout, etc.
// TODO Have the "open browser" functionality somewhere in here
// TODO A post run clean up of the command? e.g. set SilenceUsage to true unless there are sub-commands

// IOStreams allows individual commands access to standard process streams (or their overrides).
type IOStreams struct {
	// In is used to access the standard input stream (or it's override)
	In io.Reader
	// Out is used to access the standard output stream (or it's override)
	Out io.Writer
	// ErrOut is used to access the standard error output stream (or it's override)
	ErrOut io.Writer
}

// SetStreams updates the streams using the supplied command
func SetStreams(streams *IOStreams, cmd *cobra.Command) {
	streams.Out = cmd.OutOrStdout()
	streams.ErrOut = cmd.ErrOrStderr()
	streams.In = cmd.InOrStdin()
}

// StreamsPreRun is intended to be used as a pre-run function for commands when no other action is required
func StreamsPreRun(streams *IOStreams) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		SetStreams(streams, cmd)
	}
}

// SetExperimentsAPI creates a new experiments API interface from the supplied configuration
func SetExperimentsAPI(api *experimentsv1alpha1.API, cfg config.Config, cmd *cobra.Command) error {
	// TODO What if cfg == nil? Right now it will panic...
	ea, err := experimentsv1alpha1.NewForConfig(cfg, userAgent(cmd))
	if err != nil {
		return err
	}
	*api = ea
	return nil
}

// ConfigGlobals sets up persistent globals for the supplied configuration
func ConfigGlobals(cfg *internalconfig.RedSkyConfig, cmd *cobra.Command) {
	// Make sure we get the root to make these globals
	root := cmd.Root()

	// Create the configuration options on top of environment variable overrides
	root.PersistentFlags().StringVar(&cfg.Filename, "redskyconfig", cfg.Filename, "Path to the redskyconfig file to use.")
	root.PersistentFlags().StringVar(&cfg.Overrides.Context, "context", "", "The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.")

	// Set the persistent pre-run on the root, individual commands can bypass this by supplying their own persistent pre-run
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error { return cfg.Load() }
}

// WithContextE wraps a function that accepts a context in one that accepts a command and argument slice. The background
// context is used as the root context, however a child context with additional configuration may be supplied to the
// to the wrapped function.
func WithContextE(runE func(context.Context) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		ctx = context.WithValue(ctx, oauth2.HTTPClient, http.Client{Transport: userAgent(cmd)})
		return runE(ctx)
	}
}

// WithoutArgsE wraps a no-argument function in one that accepts a command and argument slice
func WithoutArgsE(runE func() error) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error { return runE() }
}

// ExitOnError converts all the error returning run functions to non-error implementations that immediately exit
func ExitOnError(cmd *cobra.Command) {
	// Convert a RunE to a Run
	wrapE := func(runE func(*cobra.Command, []string) error) func(*cobra.Command, []string) {
		return func(cmd *cobra.Command, args []string) {
			// TODO Move the CheckErr implementation here once everything is migrated over
			cmdutil.CheckErr(cmd, runE(cmd, args))
		}
	}

	// Wrap all of the RunE
	if cmd.PersistentPreRunE != nil {
		cmd.PersistentPreRun = wrapE(cmd.PersistentPreRunE)
		cmd.PersistentPreRunE = nil
	}
	if cmd.PreRunE != nil {
		cmd.PreRun = wrapE(cmd.PreRunE)
		cmd.PreRunE = nil
	}
	if cmd.RunE != nil {
		cmd.Run = wrapE(cmd.RunE)
		cmd.RunE = nil
	}
	if cmd.PostRunE != nil {
		cmd.PostRun = wrapE(cmd.PostRunE)
		cmd.PostRunE = nil
	}
	if cmd.PersistentPostRunE != nil {
		cmd.PersistentPostRun = wrapE(cmd.PersistentPostRunE)
		cmd.PersistentPostRunE = nil
	}
}

func userAgent(cmd *cobra.Command) http.RoundTripper {
	// TODO Get version number from cmd?
	// TODO Include OS, etc. in comment?
	return version.UserAgent(cmd.Root().Name(), "", nil)
}
