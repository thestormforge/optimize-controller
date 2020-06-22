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
	"fmt"
	"io"
	"os"
	"os/exec"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	internalconfig "github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyapi"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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

// NewExperimentsAPI creates a new experiments API interface from the supplied configuration
func NewExperimentsAPI(ctx context.Context, cfg config.Config) (experimentsv1alpha1.API, error) {
	// Reuse the OAuth2 base transport for the API calls
	t := oauth2.NewClient(ctx, nil).Transport
	c, err := redskyapi.NewClient(ctx, cfg, t)
	if err != nil {
		return nil, err
	}

	return experimentsv1alpha1.NewAPI(c), nil
}

// SetPrinter assigns the resource printer during the pre-run of the supplied command
func SetPrinter(meta TableMeta, printer *ResourcePrinter, cmd *cobra.Command) {
	pf := newPrintFlags(meta, cmd.Annotations)
	pf.addFlags(cmd)
	AddPreRunE(cmd, func(*cobra.Command, []string) error {
		return pf.toPrinter(printer)
	})
}

// SetKubePrinter assigns a client-go enabled resource printer during the pre-run of the supplied command
func SetKubePrinter(printer *ResourcePrinter, cmd *cobra.Command) {
	kp := &kubePrinter{scheme: runtime.NewScheme()}
	_ = clientgoscheme.AddToScheme(kp.scheme)
	_ = redskyv1alpha1.AddToScheme(kp.scheme)
	_ = redskyv1beta1.AddToScheme(kp.scheme)
	pf := newPrintFlags(kp, cmd.Annotations)
	pf.addFlags(cmd)
	AddPreRunE(cmd, func(*cobra.Command, []string) error {
		if err := pf.toPrinter(&kp.printer); err != nil {
			return err
		}
		*printer = kp
		return nil
	})
}

// ConfigGlobals sets up persistent globals for the supplied configuration
func ConfigGlobals(cfg *internalconfig.RedSkyConfig, cmd *cobra.Command) {
	// Make sure we get the root to make these globals
	root := cmd.Root()

	// Create the configuration options on top of environment variable overrides
	root.PersistentFlags().StringVar(&cfg.Filename, "redskyconfig", cfg.Filename, "Path to the redskyconfig file to use.")
	root.PersistentFlags().StringVar(&cfg.Overrides.Context, "context", "", "The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.")
	root.PersistentFlags().StringVar(&cfg.Overrides.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
	root.PersistentFlags().StringVarP(&cfg.Overrides.Namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request.")

	_ = root.MarkFlagFilename("redskyconfig")
	_ = root.MarkFlagFilename("kubeconfig")

	// Set the persistent pre-run on the root, individual commands can bypass this by supplying their own persistent pre-run
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error { return cfg.Load() }
}

// WithContextE wraps a function that accepts a context in one that accepts a command and argument slice
func WithContextE(runE func(context.Context) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error { return runE(cmd.Context()) }
}

// WithoutArgsE wraps a no-argument function in one that accepts a command and argument slice
func WithoutArgsE(runE func() error) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error { return runE() }
}

// AddPreRunE adds an error returning pre-run function to the supplied command, existing pre-run actions will run AFTER
// the supplied function, and only if the supplied pre-run function does not return an error
func AddPreRunE(cmd *cobra.Command, preRunE func(*cobra.Command, []string) error) {
	// Nothing set yet, just add it
	if cmd.PreRunE == nil && cmd.PreRun == nil {
		cmd.PreRunE = preRunE
		return
	}

	// Capture the existing function
	oldPreRunE := cmd.PreRunE
	oldPreRun := cmd.PreRun

	// Redefine the pre-run
	cmd.PreRun = nil
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if err := preRunE(cmd, args); err != nil {
			return err
		}
		if oldPreRunE != nil {
			return oldPreRunE(cmd, args)
		}
		if oldPreRun != nil {
			oldPreRun(cmd, args)
		}
		return nil
	}
}

// ExitOnError converts all the error returning run functions to non-error implementations that immediately exit
func ExitOnError(cmd *cobra.Command) {
	// Convert a RunE to a Run
	wrapE := func(runE func(*cobra.Command, []string) error) func(*cobra.Command, []string) {
		return func(cmd *cobra.Command, args []string) {
			err := runE(cmd, args)
			if err == nil {
				return
			}

			// Handle forked process errors by propagating the exit status
			if eerr, ok := err.(*exec.ExitError); ok && !eerr.Success() {
				os.Exit(eerr.ExitCode())
			}

			// Handle unauthorized errors by suggesting `login`
			if experimentsv1alpha1.IsUnauthorized(err) {
				msg := "unauthorized"
				if _, ok := err.(*experimentsv1alpha1.Error); ok {
					msg = err.Error()
				}
				err = fmt.Errorf("%s, try running 'redskyctl login'", msg)
			}

			// TODO With the exception of silence usage behavior and stdout vs. stderr, this is basically what Cobra already does with a RunE...
			cmd.PrintErr("Error: ", err.Error(), "\n")
			os.Exit(1)
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
