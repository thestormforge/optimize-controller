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
	"strings"

	"github.com/spf13/cobra"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/experiments"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-controller/v2/internal/setup"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1/numstr"
	batchv1 "k8s.io/api/batch/v1"
)

type TrialOptions struct {
	experiments.SuggestOptions

	Filename       string
	Job            string
	JobTrialNumber int
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
	cmd.Flags().StringVar(&o.Job, "job", "", "generate the specified trial job; one of: trial|create|delete")
	cmd.Flags().IntVar(&o.JobTrialNumber, "job-trial-number", 0, "explicitly set the trial number when generating jobs")
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
		experiments.DefaultBaseline, // This option is unique to the Kube based implementation
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
	exp := &optimizev1beta2.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, exp); err != nil {
		return err
	}

	if len(exp.Spec.Parameters) == 0 {
		return fmt.Errorf("experiment must contain at least one parameter")
	}

	// Convert the experiment so we can use it to collect the suggested assignments
	_, serverExperiment, baselines, err := server.FromCluster(exp)
	if err != nil {
		return err
	}
	if baselines != nil {
		o.Baselines = make(map[string]*numstr.NumberOrString)
		for _, a := range baselines.Assignments {
			o.Baselines[a.ParameterName] = &a.Value
		}
	}
	ta := experimentsv1alpha1.TrialAssignments{}
	if err := o.SuggestAssignments(serverExperiment, &ta); err != nil {
		return err
	}
	if err := o.AddLabels(&ta); err != nil {
		return err
	}

	// Build the trial
	t := &optimizev1beta2.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t)
	server.ToClusterTrial(t, &ta)

	// NOTE: Leaving the trial name empty and generateName non-empty means that you MUST use `kubectl create` and not `apply`

	// Clear out some values we do not need
	t.Finalizers = nil
	t.Annotations = nil

	// Print the trial directly if no job conversion was requested
	if o.Job == "" {
		return o.Printer.PrintObj(t, o.Out)
	}

	// Convert the trial into a job
	job, err := newJob(t, o.Job, o.JobTrialNumber)
	if err != nil {
		return err
	}

	return o.Printer.PrintObj(job, o.Out)
}

func newJob(t *optimizev1beta2.Trial, mode string, trialNumber int) (*batchv1.Job, error) {
	// Make sure the trial has a name when generating the jobs or we produce invalid output
	if t.Name == "" {
		t.Name = fmt.Sprintf("%s%d", t.GenerateName, trialNumber)
	}

	// If the mode is "trial" generate the actual trial job instead of a setup job
	if strings.EqualFold(mode, "trial") {
		return trial.NewJob(t), nil
	}

	// Create the setup job
	job, err := setup.NewJob(t, mode)
	if err != nil {
		return nil, err
	}

	// Instead of checking ahead of time for setup tasks, check the number of containers
	// on the job. This will better account for things like the "skip" settings.
	if len(job.Spec.Template.Spec.Containers) == 0 {
		return nil, nil
	}

	return job, nil
}
