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

package reset

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/initialize"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// Options is the configuration for suggesting assignments
type Options struct {
	// Config is the Optimize Configuration used to generate the controller manifests for reset
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall from a cluster",
		Long:  "Uninstall StormForge Optimize from a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.reset),
	}

	return cmd
}

func (o *Options) reset(ctx context.Context) error {
	// Delete the CRDs first to avoid issues with the controller being deleted before it can remove the finalizers
	deleteCRD, err := o.Config.Kubectl(ctx, "delete", "--ignore-not-found", "crd", "trials.optimize.stormforge.io", "experiments.optimize.stormforge.io")
	if err != nil {
		return err
	}
	deleteCRD.Stdout = o.Out
	deleteCRD.Stderr = o.ErrOut
	if err := deleteCRD.Run(); err != nil {
		return err
	}

	// Fork `kubectl delete` and get a pipe to write manifests to
	kubectlDelete, err := o.Config.Kubectl(ctx, "delete", "--ignore-not-found", "-f", "-")
	if err != nil {
		return err
	}
	kubectlDelete.Stdout = o.Out
	kubectlDelete.Stderr = o.ErrOut
	w, err := kubectlDelete.StdinPipe()
	if err != nil {
		return err
	}

	// Generate all of the manifests (with YAML document delimiters)
	go func() {
		defer func() { _ = w.Close() }()
		_ = o.generateInstall(w)
	}()

	// Wait for everything to be deleted
	return kubectlDelete.Run()
}

func (o *Options) generateInstall(out io.Writer) error {
	opts := &initialize.GeneratorOptions{
		Config:               o.Config,
		SkipSecret:           true,
		IncludeBootstrapRole: true,
	}
	cmd := initialize.NewGeneratorCommand(opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(o.ErrOut)
	return cmd.Execute()
}
