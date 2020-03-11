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

package initialize

import (
	"context"
	"fmt"
	"io"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/spf13/cobra"
)

// TODO We should be passing in labels/annotations to apply from here (e.g. managed-by redskyctl comes from here)

// Options is the configuration for initialization
type Options struct {
	GeneratorOptions

	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool
}

// NewCommand creates a command for performing an initialization
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install to a cluster",
		Long:  "Install Red Sky Ops to a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.initialize),
	}

	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "Create the bootstrap role (if it does not exist).")
	cmd.Flags().BoolVar(&o.IncludeExtraPermissions, "extra-permissions", o.IncludeExtraPermissions, "Generate permissions required for features like namespace creation")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) initialize(ctx context.Context) error {
	// Fork `kubectl apply` and get a pipe to write manifests to
	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s"
	kubectlApply, err := o.Config.Kubectl(ctx, "apply", "-f", "-")
	if err != nil {
		return err
	}
	kubectlApply.Stdout = o.Out
	kubectlApply.Stderr = o.ErrOut
	w, err := kubectlApply.StdinPipe()
	if err != nil {
		return err
	}
	if err := kubectlApply.Start(); err != nil {
		return err
	}

	// Generate all of the manifests
	// TODO How should we synchronize this? How should we check the close error?
	errChan := make(chan error)
	go func() {
		err := o.generateManifests(w)
		if err != nil {
			errChan <- err
		}

		// Close the stream (tells kubectl there are no more resources to apply) and the error channel (so we can wait on kubectl)
		_ = w.Close()
		close(errChan)
	}()
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Wait for everything to be applied
	return kubectlApply.Wait()
}

// generateManifests writes all of the initialization manifests to the supplied writer
func (o *Options) generateManifests(out io.Writer) error {
	if err := o.generateInstall(out); err != nil {
		return err
	}
	if err := o.generateBootstrapRole(out); err != nil {
		return err
	}
	if err := o.generateSecret(out); err != nil {
		return err
	}
	return nil
}

func (o *Options) generateInstall(out io.Writer) error {
	opts := &o.GeneratorOptions
	cmd := NewGeneratorCommand(opts)
	return o.executeCommand(cmd, out)
}

func (o *Options) generateBootstrapRole(out io.Writer) error {
	opts := &grant_permissions.GeneratorOptions{
		Config:                o.Config,
		SkipDefault:           !o.IncludeBootstrapRole,
		CreateTrialNamespaces: o.IncludeExtraPermissions,
	}
	cmd := grant_permissions.NewGeneratorCommand(opts)
	return o.executeCommand(cmd, out)
}

func (o *Options) generateSecret(out io.Writer) error {
	opts := &authorize_cluster.GeneratorOptions{
		Config: o.Config,
	}
	cmd := authorize_cluster.NewGeneratorCommand(opts)
	// TODO Ignore errors if we aren't logged in yet? Or just skip execution all together?
	return o.executeCommand(cmd, out)
}

// executeCommand runs the supplied command, send output to the writer
func (o *Options) executeCommand(cmd *cobra.Command, out io.Writer) error {
	// Prepare the command and execute it
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	if err := cmd.Execute(); err != nil {
		return err
	}

	// Since we are dumping the output of multiple generators into a single stream, insert a YAML document separator
	_, _ = fmt.Fprintln(out, "---")
	return nil
}
