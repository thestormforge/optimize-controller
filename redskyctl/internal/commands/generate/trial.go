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

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/experiment"
	"github.com/redskyops/redskyops-controller/internal/server"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/util"
	"github.com/spf13/cobra"
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

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "File that contains the experiment to generate trials for.")

	cmd.Flags().StringToStringVarP(&o.Assignments, "assign", "A", nil, "Assign an explicit value to a parameter.")
	cmd.Flags().BoolVar(&o.AllowInteractive, "interactive", o.AllowInteractive, "Allow interactive prompts for unspecified parameter assignments.")
	cmd.Flags().StringVar(&o.DefaultBehavior, "default", "", "Select the behavior for default values; one of: none|min|max|rand.")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	commander.SetKubePrinter(&o.Printer, cmd)
	commander.ExitOnError(cmd)
	return cmd
}

func (o *TrialOptions) generate() error {
	// Read the experiments
	exp, err := util.ReadExperiment(o.Filename, o.In)
	if err != nil {
		return err
	}

	if len(exp.Spec.Parameters) == 0 {
		return fmt.Errorf("experiment must contain at least one parameter")
	}

	// Convert the experiment so we can use it to collect the suggested assignments
	_, serverExperiment := server.FromCluster(exp)
	sug, err := o.SuggestAssignments(serverExperiment)
	if err != nil {
		return err
	}

	// Build the trial
	t := &redskyv1beta1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t)
	server.ToClusterTrial(t, sug)

	// NOTE: Leaving the trial name empty and generateName non-empty means that you MUST use `kubectl create` and not `apply`

	// Clear out some values we do not need
	t.Finalizers = nil
	t.Annotations = nil

	return o.Printer.PrintObj(t, o.Out)
}
