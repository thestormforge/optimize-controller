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

package experiments

import (
	"context"
	"fmt"

	"github.com/redskyops/redskyops-controller/internal/controller"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// DeleteOptions includes the configuration for deleting experiment API objects
type DeleteOptions struct {
	Options

	// IgnoreNotFound treats missing resources as successful deletes
	IgnoreNotFound bool
}

// NewDeleteCommand creates a new deletion command
func NewDeleteCommand(o *DeleteOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a Red Sky resource",
		Long:  "Delete Red Sky resources from the remote server",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: commander.WithContextE(o.delete),
	}

	TypeAndNameArgs(cmd, &o.Options)
	o.Printer = &verbPrinter{verb: "deleted"}
	commander.ExitOnError(cmd)
	return cmd
}

func (o *DeleteOptions) delete(ctx context.Context) error {
	switch o.GetType() {
	case TypeExperiment:
		for _, name := range o.Names {
			if err := o.deleteExperiment(ctx, name); o.ignoreDeleteError(err) != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cannot delete %s", o.GetType())
	}
	return nil
}

// ignoreDelete error is helper for ignoring errors that occur during deletion
func (o *DeleteOptions) ignoreDeleteError(err error) error {
	if o.IgnoreNotFound && controller.IgnoreNotFound(err) == nil {
		return nil
	}
	return err
}

func (o *DeleteOptions) deleteExperiment(ctx context.Context, name string) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, experimentsv1alpha1.NewExperimentName(name))
	if err != nil {
		return err
	}

	if err := o.ExperimentsAPI.DeleteExperiment(ctx, exp.Self); err != nil {
		return err
	}

	return o.Printer.PrintObj(&exp, o.Out)
}
