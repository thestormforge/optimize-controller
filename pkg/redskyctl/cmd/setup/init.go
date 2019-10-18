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

package setup

import (
	"bytes"
	"fmt"

	"github.com/redskyops/k8s-experiment/pkg/api"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/generate"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	initLong    = `Install Red Sky Ops to a cluster`
	initExample = ``
)

// InitOptions is the configuration for initialization
type InitOptions struct {
	Kubectl
	IncludeBootstrapRole bool
	DryRun               bool

	// TODO Add --env options that are merged with the configuration environment variables

	cmdutil.IOStreams
}

// NewInitOptions returns a new initialization options struct
func NewInitOptions(ioStreams cmdutil.IOStreams) *InitOptions {
	return &InitOptions{
		Kubectl: Kubectl{
			Namespace: "redsky-system",
		},
		IncludeBootstrapRole: true,
		IOStreams:            ioStreams,
	}
}

func NewInitCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewInitOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install to a cluster",
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "--bootstrap-role", true, "Create the bootstrap role (if it does not exist).")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Generate the manifests instead of applying them.")

	return cmd
}

func (o *InitOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) Run() error {
	if err := o.applyConfiguration(); err != nil {
		return err
	}

	if err := o.bootstrapRole(); err != nil {
		return err
	}

	return nil
}

// Kubectl applies the '/config' contents (via the setuptools image)
func (o *InitOptions) applyConfiguration() error {
	// Generate the environment for the manager
	cfg, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	// TODO We need to get more stuff into the environment, e.g. Datadog keys

	// Generate the product manifests
	cmd := o.Kubectl.GenerateRedSkyOpsManifests(cfg.Environment())
	if o.DryRun {
		cmd.Stdout = o.Out
		return cmd.Run()
	}

	applyCmd := o.Kubectl.Apply()
	applyCmd.Stdout = o.Out
	return RunPiped(cmd, applyCmd)
}

// Kubectl creates the `redskyctl generate rbac --bootstrap` output
func (o *InitOptions) bootstrapRole() error {
	if !o.IncludeBootstrapRole {
		return nil
	}

	opts := generate.NewGenerateRBACOptions(cmdutil.IOStreams{Out: o.Out})
	opts.Bootstrap = true
	if err := opts.Complete(); err != nil {
		return err
	}

	if o.DryRun {
		_, _ = fmt.Fprintln(o.Out, "---")
		return opts.Run()
	}

	buf := &bytes.Buffer{}
	opts.IOStreams.Out = buf
	if err := opts.Run(); err != nil {
		return err
	}

	createCmd := o.Kubectl.Create()
	createCmd.Stdout = o.Out
	createCmd.Stdin = buf

	// TODO We expect this to fail when the resource exists, but what about everything else?
	_ = createCmd.Run()

	return nil
}
