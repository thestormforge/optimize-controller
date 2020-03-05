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

package deletion

import (
	"context"
	"fmt"

	"github.com/redskyops/redskyops-controller/internal/controller"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/spf13/cobra"
)

const (
	// TypeExperiment is the type argument to use to delete experiments
	TypeExperiment = "experiment"
)

// Options includes the configuration for the subcommands
type Options struct {
	// Config is the Red Sky Configuration
	Config config.Config
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Type of resource type to delete
	Type string
	// Names of the resources to delete
	Names []string
	// IgnoreNotFound treats missing resources as successful deletes
	IgnoreNotFound bool
}

// NewCommand creates a new deletion command
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete TYPE NAME...",
		Short: "Delete a Red Sky resource",
		Long:  "Delete Red Sky resources from the remote server",

		ValidArgs: []string{TypeExperiment},
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(2)(cmd, args); err != nil {
				return err
			}
			return cobra.OnlyValidArgs(cmd, args[:1])
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			o.Type = args[0]
			o.Names = args[1:]
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: commander.WithContextE(o.delete),
	}

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) delete(ctx context.Context) error {
	for _, name := range o.Names {
		var err error
		switch o.Type {
		case TypeExperiment, "exp":
			err = o.deleteExperiment(ctx, name)
		default:
			return fmt.Errorf("unknown resource type: %s", o.Type)
		}

		if err != nil {
			if o.IgnoreNotFound && controller.IgnoreNotFound(err) == nil {
				continue
			}
			return err
		}
		_, _ = fmt.Fprintf(o.Out, "%s \"%s\" deleted\n", o.Type, name)
	}
	return nil
}

func (o *Options) deleteExperiment(ctx context.Context, name string) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, experimentsv1alpha1.NewExperimentName(name))
	if err != nil {
		return err
	}
	return o.ExperimentsAPI.DeleteExperiment(ctx, exp.Self)
}
