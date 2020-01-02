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
	"os/exec"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	initLong    = `Install Red Sky Ops to a cluster`
	initExample = ``
)

// InitOptions is the configuration for initialization
type InitOptions struct {
	Kubectl                 *cmdutil.Kubectl
	Namespace               string
	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool

	Authorization *AuthorizeOptions

	cmdutil.IOStreams
}

// NewInitOptions returns a new initialization options struct
func NewInitOptions(ioStreams cmdutil.IOStreams) *InitOptions {
	return &InitOptions{
		Kubectl:              cmdutil.NewKubectl(),
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
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Override default namespace.")
	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "Create the bootstrap role (if it does not exist).")
	cmd.Flags().BoolVar(&o.IncludeExtraPermissions, "extra-permissions", false, "Generate permissions required for features like namespace creation")

	return cmd
}

func (o *InitOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if o.Authorization == nil {
		o.Authorization = NewAuthorizeOptions(o.IOStreams)
		o.Authorization.Namespace = o.Namespace
	}

	if err := o.Authorization.Complete(f, cmd, args); err != nil {
		return err
	}

	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) Run() error {
	if err := o.install(); err != nil {
		return err
	}

	if err := o.bootstrapRole(); err != nil {
		return err
	}

	if err := o.Authorization.Run(); err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) install() error {
	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s"
	applyCmd := o.Kubectl.NewCmd("apply", "-f", "-")
	applyCmd.Stdout = o.Out
	applyCmd.Stderr = o.ErrOut
	return install(o.Kubectl, o.Namespace, applyCmd)
}

func (o *InitOptions) bootstrapRole() error {
	if !o.IncludeBootstrapRole {
		return nil
	}

	createCmd := o.Kubectl.NewCmd("create", "-f", "-")
	createCmd.Stdout = o.Out
	if err := bootstrapRole(createCmd, o.IncludeExtraPermissions); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// TODO We expect this to fail when the resource exists, but what about everything else?
			return nil
		}
		return err
	}
	return nil
}
