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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
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

// YAMLReader returns a resource node reader for the named file.
func (s *IOStreams) YAMLReader(filename string) kio.Reader {
	r := &kio.ByteReader{
		Reader: &fileReader{Filename: filename},
		SetAnnotations: map[string]string{
			kioutil.PathAnnotation: filename,
		},
	}

	// Handle the process relative "default" stream
	if filename == "-" {
		r.Reader = s.In

		delete(r.SetAnnotations, kioutil.PathAnnotation)
		if path, err := filepath.Abs("stdin"); err == nil {
			r.SetAnnotations[kioutil.PathAnnotation] = path
		}
	}

	return r
}

// fileReader is a reader the lazily opens a file for reading and automatically
// closes it when it hits EOF.
type fileReader struct {
	sync.Once
	io.ReadCloser
	Filename string
}

func (r *fileReader) Read(p []byte) (n int, err error) {
	r.Once.Do(func() {
		r.ReadCloser, err = os.Open(r.Filename)
	})
	if err != nil {
		return
	}

	n, err = r.ReadCloser.Read(p)
	if err != io.EOF {
		return
	}
	if closeErr := r.ReadCloser.Close(); closeErr != nil {
		err = closeErr
	}
	return
}

// YAMLWriter returns a resource node writer for the current output stream. The writer
// is configured to strip common annotations used during pipeline processing.
func (s *IOStreams) YAMLWriter() kio.Writer {
	return &kio.ByteWriter{
		Writer: s.Out,
		ClearAnnotations: []string{
			kioutil.PathAnnotation,
			filters.FmtAnnotation,
		},
	}
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
func SetExperimentsAPI(expAPI *experimentsv1alpha1.API, cfg *config.OptimizeConfig, cmd *cobra.Command) error {
	ctx := cmd.Context()
	srv, err := config.CurrentServer(cfg.Reader())
	if err != nil {
		return err
	}

	// NOTE: We should use `srv.Identifier` but technically this version of the configuration
	// exposes this double counted "/v1/experiments/" endpoint
	address := strings.TrimSuffix(srv.API.ExperimentsEndpoint, "/v1/experiments/")

	// Reuse the OAuth2 base transport for the API calls
	t, err := cfg.Authorize(ctx, oauth2.NewClient(ctx, nil).Transport)
	if err != nil {
		return err
	}

	c, err := api.NewClient(address, t)
	if err != nil {
		return err
	}

	*expAPI = experimentsv1alpha1.NewAPI(c)
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
	_ = optimizev1beta2.AddToScheme(kp.scheme)
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
func ConfigGlobals(cfg *config.OptimizeConfig, cmd *cobra.Command) {
	// Make sure we get the root to make these globals
	root := cmd.Root()

	// Create the configuration options on top of environment variable overrides
	root.PersistentFlags().StringVar(&cfg.Filename, "stormforgeconfig", cfg.Filename, "path to the stormforgeconfig `file` to use")
	root.PersistentFlags().StringVar(&cfg.Overrides.Context, "context", "", "the `name` of the stormforgeconfig context to use, NOT THE KUBE CONTEXT")
	root.PersistentFlags().StringVar(&cfg.Overrides.KubeConfig, "kubeconfig", "", "path to the kubeconfig `file` to use for CLI requests")
	root.PersistentFlags().StringVarP(&cfg.Overrides.Namespace, "namespace", "n", "", "the Kubernetes namespace scope for this CLI request")

	_ = root.MarkFlagFilename("stormforgeconfig")
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

// SetFlagValues updates the named flag usage and completion to include possible choices.
func SetFlagValues(cmd *cobra.Command, flagName string, values ...string) {
	f := cmd.Flag(flagName)
	if f == nil {
		return
	}

	// Remove blank values
	tmp := values[:0]
	for _, v := range values {
		if v != "" {
			tmp = append(tmp, v)
		}
	}
	values = tmp

	f.Usage = fmt.Sprintf("%s; one of: %s", f.Usage, strings.Join(values, "|"))
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c := make([]string, 0, len(values))
		for _, v := range values {
			if strings.HasPrefix(v, toComplete) {
				c = append(c, v)
			}
		}
		return c, cobra.ShellCompDirectiveNoFileComp
	})
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
