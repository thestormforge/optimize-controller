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
	"bufio"
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

	// Generate all of the manifests (with YAML document delimiters)
	go func() {
		defer func() { _ = w.Close() }()
		// Buffer the manifests so we only send the entire group to kubectl; otherwise the time delay generating
		// the secret may result in the controller pods being created before the secret exists.
		buf := bufio.NewWriterSize(w, 2<<18)
		if err := o.generateInstall(buf); err != nil {
			return
		}
		_, _ = fmt.Fprintln(buf, "---")
		if err := o.generateBootstrapRole(buf); err != nil {
			return
		}
		_, _ = fmt.Fprintln(buf, "---")
		if err := o.generateSecret(buf); err != nil {
			return
		}
		_ = buf.Flush()
	}()

	// Wait for everything to be applied
	return kubectlApply.Run()
}

func (o *Options) generateInstall(out io.Writer) error {
	opts := o.GeneratorOptions
	cmd := NewGeneratorCommand(&opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	return cmd.Execute()
}

func (o *Options) generateBootstrapRole(out io.Writer) error {
	opts := &grant_permissions.GeneratorOptions{
		Config:                o.Config,
		SkipDefault:           !o.IncludeBootstrapRole,
		CreateTrialNamespaces: o.IncludeExtraPermissions,
	}
	cmd := grant_permissions.NewGeneratorCommand(opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	return cmd.Execute()
}

func (o *Options) generateSecret(out io.Writer) error {
	opts := &authorize_cluster.GeneratorOptions{
		Config:            o.Config,
		AllowUnauthorized: true,
	}
	cmd := authorize_cluster.NewGeneratorCommand(opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	return cmd.Execute()
}
