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

	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/generate"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	resetLong    = `Uninstall Red Sky Ops from a cluster`
	resetExample = ``
)

// ResetOptions is the configuration for suggesting assignments
type ResetOptions struct {
	Kubectl

	cmdutil.IOStreams
}

// NewResetOptions returns a new reset options struct
func NewResetOptions(ioStreams cmdutil.IOStreams) *ResetOptions {
	return &ResetOptions{
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
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ResetOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *ResetOptions) Run() error {
	// Delete the bootstrap role if it exists
	if err := o.bootstrapRole(); err != nil {
		return err
	}

	// Delete the created manifests
	cmd := o.Kubectl.GenerateRedSkyOpsManifests(nil)
	deleteCmd := o.Kubectl.Delete()
	deleteCmd.Stdout = o.Out
	return RunPiped(cmd, deleteCmd)
}

func (o *ResetOptions) bootstrapRole() error {
	opts := generate.NewGenerateRBACOptions(cmdutil.IOStreams{Out: o.Out})
	opts.Bootstrap = true
	if err := opts.Complete(); err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	opts.IOStreams.Out = buf
	if err := opts.Run(); err != nil {
		return err
	}

	deleteCmd := o.Kubectl.Delete()
	deleteCmd.Stdout = o.Out
	deleteCmd.Stdin = buf
	return deleteCmd.Run()
}
