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

package generate

import (
	"fmt"

	"github.com/spf13/cobra"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/experiment"
	"github.com/thestormforge/optimize-controller/internal/server"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/experiments"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
)

type TrialOptions struct {
	experiments.SuggestOptions

	Filename string
}

func NewTrialCommand(o *TrialOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trial",
		Short: "Generate experiment trials",
		Long:  "Generate a trial from an experiment manifest",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
			commander.PrinterHideStatus:     "true",
		},

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "file that contains the experiment to generate trials for")
	cmd.Flags().StringVarP(&o.Labels, "labels", "l", "", "comma separated `key=value` labels to apply to the trial")

	cmd.Flags().StringToStringVarP(&o.Assignments, "assign", "A", nil, "assign an explicit `key=value` to a parameter")
	cmd.Flags().BoolVar(&o.AllowInteractive, "interactive", o.AllowInteractive, "allow interactive prompts for unspecified parameter assignments")
	cmd.Flags().StringVar(&o.DefaultBehavior, "default", "", "select the `behavior` for default values")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	commander.SetFlagValues(cmd, "default",
		experiments.DefaultNone,
		experiments.DefaultMinimum,
		experiments.DefaultMaximum,
		experiments.DefaultRandom,
	)

	commander.SetKubePrinter(&o.Printer, cmd, nil)

	return cmd
}

func (o *TrialOptions) generate() error {
	r, err := o.IOStreams.OpenFile(o.Filename)
	if err != nil {
		return err
	}

	// Read the experiment
	exp := &redskyv1beta1.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, exp); err != nil {
		return err
	}

	if len(exp.Spec.Parameters) == 0 {
		return fmt.Errorf("experiment must contain at least one parameter")
	}

	// Convert the experiment so we can use it to collect the suggested assignments
	_, serverExperiment, _, err := server.FromCluster(exp)
	if err != nil {
		return err
	}
	ta := experimentsv1alpha1.TrialAssignments{}
	if err := o.SuggestAssignments(serverExperiment, &ta); err != nil {
		return err
	}
	if err := o.AddLabels(&ta); err != nil {
		return err
	}

	// Build the trial
	t := &redskyv1beta1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t)
	server.ToClusterTrial(t, &ta)

	// NOTE: Leaving the trial name empty and generateName non-empty means that you MUST use `kubectl create` and not `apply`

	// Clear out some values we do not need
	t.Finalizers = nil
	t.Annotations = nil

	return o.Printer.PrintObj(t, o.Out)
}
