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

package initialize

import (
	"bytes"
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
)

// Options is the configuration for initialization
type Options struct {
	GeneratorOptions

	Wait bool
}

// NewCommand creates a command for performing an initialization
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install to a cluster",
		Long:  "Install StormForge Optimize to a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.Initialize),
	}

	cmd.Flags().BoolVar(&o.Wait, "wait", o.Wait, "wait for resources to be established before returning")

	o.addFlags(cmd)

	return cmd
}

func (o *Options) Initialize(ctx context.Context) error {
	install, err := o.generateInstall()
	if err != nil {
		return err
	}

	// Run `kubectl apply` to install the product
	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=optimize,app.kubernetes.io/managed-by=%s"
	kubectlApply, err := o.Config.Kubectl(ctx, "apply", "-f", "-")
	if err != nil {
		return err
	}
	kubectlApply.Stdout = o.Out
	kubectlApply.Stderr = o.ErrOut
	kubectlApply.Stdin = install
	if err := kubectlApply.Run(); err != nil {
		return err
	}

	// Run `kubectl wait` to ensure the CRD is installed
	if o.Wait {
		kubectlWait, err := o.Config.Kubectl(ctx, "wait", "crd/experiments.optimize.stormforge.io", "crd/trials.optimize.stormforge.io", "--for", "condition=Established")
		if err != nil {
			return err
		}
		if err := kubectlWait.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (o *Options) generateInstall() (io.Reader, error) {
	var buf bytes.Buffer

	opts := o.GeneratorOptions // Make a copy so we can overwrite the IOStreams without impacting the init command
	opts.labels = map[string]string{"app.kubernetes.io/managed-by": "stormforge"}
	opts.IOStreams = commander.IOStreams{Out: &buf}
	if err := opts.generate(); err != nil {
		return nil, err
	}
	return &buf, nil
}
