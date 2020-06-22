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
		Use:   "delete (TYPE NAME | TYPE/NAME ...)",
		Short: "Delete a Red Sky resource",
		Long:  "Delete Red Sky resources from the remote server",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)

			expAPI, err := commander.NewExperimentsAPI(cmd.Context(), o.Config)
			if err != nil {
				return err
			}

			o.ExperimentsAPI = expAPI

			return o.setNames(args)
		},
		RunE: commander.WithContextE(o.delete),
	}

	_ = cmd.MarkZshCompPositionalArgumentWords(1, validTypes()...)

	o.Printer = &verbPrinter{verb: "deleted"}
	commander.ExitOnError(cmd)
	return cmd
}

func (o *DeleteOptions) delete(ctx context.Context) error {
	for _, n := range o.Names {
		if n.Name == "" {
			return fmt.Errorf("name is required for delete")
		}

		switch n.Type {
		case typeExperiment:
			if err := o.deleteExperiment(ctx, n.ExperimentName()); o.ignoreDeleteError(err) != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot delete \"%s\"", n.Type)
		}
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

func (o *DeleteOptions) deleteExperiment(ctx context.Context, name experimentsv1alpha1.ExperimentName) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, name)
	if err != nil {
		return err
	}

	if err := o.ExperimentsAPI.DeleteExperiment(ctx, exp.SelfURL); err != nil {
		return err
	}

	return o.Printer.PrintObj(&exp, o.Out)
}
