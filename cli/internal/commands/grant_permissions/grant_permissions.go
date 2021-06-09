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

package grant_permissions

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
)

// Options are the configuration options for granting controller permissions
type Options struct {
	GeneratorOptions
}

// NewCommand creates a new command for granting controller permissions
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grant-permissions",
		Short: "Grant permissions",
		Long:  "Grant the StormForge Optimize Controller permissions",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.grantPermissions),
	}

	o.addFlags(cmd)

	return cmd
}

func (o *Options) grantPermissions(ctx context.Context) error {
	// Fork `kubectl apply` and get a pipe to write manifests to
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

	// Generate all of the manifests
	go func() {
		defer func() { _ = w.Close() }()
		if err := o.generateBootstrapRole(w); err != nil {
			return
		}
	}()

	// Wait for everything to be applied
	return kubectlApply.Run()
}

func (o *Options) generateBootstrapRole(out io.Writer) error {
	opts := o.GeneratorOptions
	cmd := NewGeneratorCommand(&opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	return cmd.Execute()
}
