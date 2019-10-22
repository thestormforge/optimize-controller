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

package delete

import (
	"context"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/pkg/util"
	"github.com/spf13/cobra"
)

const (
	deleteLong    = `Delete Red Sky experiments from the remote server`
	deleteExample = ``
)

type DeleteOptions struct {
	Names []string

	RedSkyAPI redsky.API
	cmdutil.IOStreams
}

func NewDeleteOptions(ioStreams cmdutil.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams: ioStreams,
	}
}

func NewDeleteCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewDeleteOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "delete [experiment...]",
		Short:   "Delete a Red Sky experiment",
		Long:    deleteLong,
		Example: deleteExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *DeleteOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Names = args
	if o.RedSkyAPI, err = f.RedSkyAPI(); err != nil {
		return err
	}
	return nil
}

func (o *DeleteOptions) Run() error {
	ctx := context.Background()
	for _, name := range o.Names {
		// Get the experiment
		exp, err := o.RedSkyAPI.GetExperimentByName(ctx, redsky.NewExperimentName(name))
		if err != nil {
			if util.IgnoreNotFound(err) == nil {
				continue
			}
			return err
		}

		// Delete the experiment
		if err := o.RedSkyAPI.DeleteExperiment(ctx, exp.Self); err != nil {
			return err
		}
	}
	return nil
}
