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
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	resetLong    = `Uninstall Red Sky Ops from a cluster`
	resetExample = ``
)

// ResetOptions is the configuration for suggesting assignments
type ResetOptions struct {
	Kubectl   *cmdutil.Kubectl
	Namespace string

	cmdutil.IOStreams
}

// NewResetOptions returns a new reset options struct
func NewResetOptions(ioStreams cmdutil.IOStreams) *ResetOptions {
	return &ResetOptions{
		Kubectl:   cmdutil.NewKubectl(),
		Namespace: "redsky-system",
		IOStreams: ioStreams,
	}
}

func NewResetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewResetOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Uninstall from a cluster",
		Long:    resetLong,
		Example: resetExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Override default namespace.")

	return cmd
}

func (o *ResetOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *ResetOptions) Run() error {
	if err := o.crd(); err != nil {
		return err
	}

	if err := o.bootstrapRole(); err != nil {
		return err
	}

	if err := o.install(); err != nil {
		return err
	}

	return nil
}

func (o *ResetOptions) install() error {
	deleteCmd := o.Kubectl.NewCmd("delete", "--ignore-not-found", "-f", "-")
	deleteCmd.Stdout = o.Out
	deleteCmd.Stderr = o.ErrOut
	return install(o.Kubectl, o.Namespace, deleteCmd)
}

func (o *ResetOptions) bootstrapRole() error {
	deleteCmd := o.Kubectl.NewCmd("delete", "--ignore-not-found", "-f", "-")
	deleteCmd.Stdout = o.Out
	deleteCmd.Stderr = o.ErrOut
	return bootstrapRole(deleteCmd, false)
}

func (o *ResetOptions) crd() error {
	deleteCmd := o.Kubectl.NewCmd("delete", "--ignore-not-found", "crd",
		"trials.redskyops.dev",
		"experiments.redskyops.dev",
	)
	return deleteCmd.Run()
}
