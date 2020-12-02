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
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	redskyv1alpha1 "github.com/thestormforge/optimize-controller/api/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	internalconfig "github.com/thestormforge/optimize-go/pkg/config"
	"github.com/thestormforge/optimize-go/pkg/redskyapi"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/redskyapi/experiments/v1alpha1"
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

// OpenFile returns a read closer for the specified filename. If the filename is logically
// empty (i.e. "-"), the input stream is returned.
func (s *IOStreams) OpenFile(filename string) (io.ReadCloser, error) {
	if filename == "-" {
		return ioutil.NopCloser(s.In), nil
	}
	return os.Open(filename)
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
func SetExperimentsAPI(api *experimentsv1alpha1.API, cfg redskyapi.Config, cmd *cobra.Command) error {
	ctx := cmd.Context()

	// Reuse the OAuth2 base transport for the API calls
	t := oauth2.NewClient(ctx, nil).Transport
	c, err := redskyapi.NewClient(ctx, cfg, t)
	if err != nil {
		return err
	}

	*api = experimentsv1alpha1.NewAPI(c)
	return nil
}

// SetPrinter assigns the resource printer during the pre-run of the supplied command
func SetPrinter(meta TableMeta, printer *ResourcePrinter, cmd *cobra.Command, additionalFormats map[string]AdditionalFormat) {
	pf := newPrintFlags(meta, cmd.Annotations, additionalFormats)
	pf.addFlags(cmd)
	AddPreRunE(cmd, func(*cobra.Command, []string) error {
		return pf.toPrinter(printer)
	})
}

// SetKubePrinter assigns a client-go enabled resource printer during the pre-run of the supplied command
func SetKubePrinter(printer *ResourcePrinter, cmd *cobra.Command, additionalFormats map[string]AdditionalFormat) {
	kp := &kubePrinter{scheme: runtime.NewScheme()}
	_ = clientgoscheme.AddToScheme(kp.scheme)
	_ = redskyv1alpha1.AddToScheme(kp.scheme)
	_ = redskyv1beta1.AddToScheme(kp.scheme)
	pf := newPrintFlags(kp, cmd.Annotations, additionalFormats)
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
	root.PersistentFlags().StringVar(&cfg.Filename, "redskyconfig", cfg.Filename, "path to the redskyconfig `file` to use")
	root.PersistentFlags().StringVar(&cfg.Overrides.Context, "context", "", "the `name` of the redskyconfig context to use, NOT THE KUBE CONTEXT")
	root.PersistentFlags().StringVar(&cfg.Overrides.KubeConfig, "kubeconfig", "", "path to the kubeconfig `file` to use for CLI requests")
	root.PersistentFlags().StringVarP(&cfg.Overrides.Namespace, "namespace", "n", "", "if present, the namespace scope for this CLI request")

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

// MapErrors wraps all of the error returning functions on the supplied command (and it's sub-commands) so that
// they pass any errors through the mapping function.
func MapErrors(cmd *cobra.Command, f func(error) error) {
	// Define a function which passes all errors through the supplied mapping function
	wrapE := func(runE func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
		if runE != nil {
			return func(cmd *cobra.Command, args []string) error {
				return f(runE(cmd, args))
			}
		}
		return nil
	}

	// Wrap all the error returning functions
	cmd.PersistentPreRunE = wrapE(cmd.PersistentPreRunE)
	cmd.PreRunE = wrapE(cmd.PreRunE)
	cmd.RunE = wrapE(cmd.RunE)
	cmd.PostRunE = wrapE(cmd.PostRunE)
	cmd.PersistentPostRunE = wrapE(cmd.PersistentPostRunE)

	// Recurse and wrap errors for all of the sub-commands
	for _, c := range cmd.Commands() {
		MapErrors(c, f)
	}
}
