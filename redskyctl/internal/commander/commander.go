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
	"io"
	"os"
	"os/exec"

	internalconfig "github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/pkg/version"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/spf13/cobra"
)

// TODO Terminal type for commands whose produce character level interactions (as opposed to byte level implied by the direct streams)
// TODO Run helper that supplies a context for command execution
// TODO Have the "open browser" functionality somewhere in here

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
	ua := version.UserAgent(cmd.Root().Name(), nil)
	ea, err := experimentsv1alpha1.NewForConfig(cfg, ua)
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

	// Create the configuration options
	cfgOpts := &ConfigOptions{}
	root.PersistentFlags().StringVar(&cfgOpts.RedskyConfig, "redskyconfig", cfg.Filename, "Path to the redskyconfig file to use.")
	root.PersistentFlags().StringVar(&cfgOpts.Context, "context", "", "The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.")

	// Set the persistent pre-run on the root, individual commands can bypass this by supplying their own persistent pre-run
	root.PersistentPreRunE = cfgOpts.load(cfg)
}

// ConfigOptions support overriding configuration settings
type ConfigOptions struct {
	// RedskyConfig overrides the path of the configuration file
	RedskyConfig string
	// Context overrides the current cluster context value
	Context string
	// TODO Namespace
}

func (o *ConfigOptions) load(cfg *internalconfig.RedSkyConfig) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Override the configuration file path if necessary
		if o.RedskyConfig != "" {
			cfg.Filename = o.RedskyConfig
		}

		// Load the configuration
		if err := cfg.Load(); err != nil {
			return err
		}

		// Construct the overrides from the current context, merging will be a no-op unless the overrides are non-zero
		cc := cfg.Minify()
		cc.CurrentContext = o.Context
		cfg.Merge(cc)

		return nil
	}
}

// CheckErr is lazy error handling at it's best
func CheckErr(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	// Handle forked process errors by propagating the exit status
	if eerr, ok := err.(*exec.ExitError); ok && !eerr.Success() {
		os.Exit(eerr.ExitCode())
	}

	// This error handling leaves a lot to be desired...
	cmd.PrintErrln("Failed:", err.Error())
	os.Exit(1)
}

// TODO It might be nicer to have something like `func DoRun(func(context.Context) error) func(*cobra.Command,[]string)`
// TODO Or should we be using RunE and letting errors propagate out?
