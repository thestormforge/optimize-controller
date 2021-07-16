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

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/controller"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
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
		Short: "Delete an Optimize resource",
		Long:  "Delete StormForge Optimize resources from the remote server",

		ValidArgsFunction: o.validArgs,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			if err := commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd); err != nil {
				return err
			}
			return o.setNames(args)
		},
		RunE: commander.WithContextE(o.delete),
	}

	o.Printer = &verbPrinter{verb: "deleted"}

	return cmd
}

func (o *DeleteOptions) delete(ctx context.Context) error {
	for _, n := range o.Names {
		if n.Name == "" {
			return fmt.Errorf("name is required for delete")
		}

		switch n.Type {
		case typeExperiment:
			if err := o.deleteExperiment(ctx, n.experimentName()); o.ignoreDeleteError(err) != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot delete \"%s\"", n.Type)
		}
	}
	return nil
}

// ignoreDeleteError is a helper for ignoring errors that occur during deletion
func (o *DeleteOptions) ignoreDeleteError(err error) error {
	if o.IgnoreNotFound && controller.IgnoreNotFound(err) == nil {
		return nil
	}
	return err
}

// deleteExperiment deletes an individual experiment by name
//noinspection GoNilness
func (o *DeleteOptions) deleteExperiment(ctx context.Context, name experimentsv1alpha1.ExperimentName) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, name)
	if err != nil {
		return err
	}
	selfURL := exp.Link(api.RelationSelf)
	if selfURL == "" {
		return nil
	}

	if err := o.ExperimentsAPI.DeleteExperiment(ctx, selfURL); err != nil {
		return err
	}

	return o.Printer.PrintObj(&exp, o.Out)
}
