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
	"strings"

	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// LabelOptions includes the configuration for deleting experiment API objects
type LabelOptions struct {
	Options

	// Labels to apply
	Labels map[string]string
}

// NewLabelCommand creates a new label command
func NewLabelCommand(o *LabelOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label (TYPE NAME | TYPE/NAME ...) KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short: "Label a Red Sky resource",
		Long:  "Label Red Sky resources on the remote server",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)

			expAPI, err := commander.NewExperimentsAPI(cmd.Context(), o.Config)
			if err != nil {
				return err
			}

			o.ExperimentsAPI = expAPI

			return o.setNamesAndLabels(args)
		},
		RunE: commander.WithContextE(o.label),
	}

	_ = cmd.MarkZshCompPositionalArgumentWords(1, validTypes()...)

	o.Printer = &verbPrinter{verb: "labeled"}
	commander.ExitOnError(cmd)
	return cmd
}

func (o *LabelOptions) setNamesAndLabels(args []string) error {
	o.Labels = make(map[string]string, len(args))
	nameArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if p := strings.SplitN(arg, "=", 2); len(p) == 2 {
			o.Labels[p[0]] = p[1]
		} else if strings.HasSuffix(arg, "-") && strings.Trim(arg, "-") != "" {
			o.Labels[strings.TrimSuffix(arg, "-")] = ""
		} else {
			nameArgs = append(nameArgs, arg)
		}
	}
	return o.setNames(nameArgs)
}

func (o *LabelOptions) label(ctx context.Context) error {
	e := make([]experimentsv1alpha1.ExperimentName, 0, len(o.Names))
	t := make(map[experimentsv1alpha1.ExperimentName][]int64)

	for _, n := range o.Names {
		switch n.Type {

		case typeExperiment:
			e = append(e, n.ExperimentName())

		case typeTrial:
			key := n.ExperimentName()
			t[key] = append(t[key], n.Number)

		default:
			return fmt.Errorf("cannot label %s", n.Type)
		}
	}

	if err := o.labelExperiments(ctx, e); err != nil {
		return err
	}

	if err := o.labelTrials(ctx, t); err != nil {
		return err
	}

	return nil
}

func (o *LabelOptions) labelExperiments(ctx context.Context, names []experimentsv1alpha1.ExperimentName) error {
	for _, n := range names {
		exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, n)
		if err != nil {
			return err
		}

		if err := o.ExperimentsAPI.LabelExperiment(ctx, exp.LabelsURL, experimentsv1alpha1.ExperimentLabels{Labels: o.Labels}); err != nil {
			return err
		}

		if err := o.Printer.PrintObj(&exp, o.Out); err != nil {
			return err
		}
	}
	return nil
}

func (o *LabelOptions) labelTrials(ctx context.Context, numbers map[experimentsv1alpha1.ExperimentName][]int64) error {
	for n, nums := range numbers {
		exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, n)
		if err != nil {
			return err
		}

		// Note that you can only label completed trials
		q := &experimentsv1alpha1.TrialListQuery{Status: []experimentsv1alpha1.TrialStatus{experimentsv1alpha1.TrialCompleted}}
		tl, err := o.ExperimentsAPI.GetAllTrials(ctx, exp.TrialsURL, q)
		if err != nil {
			return err
		}

		var labeled int
		for i := range tl.Trials {
			if hasTrialNumber(&tl.Trials[i], nums) {
				t := tl.Trials[i]
				t.Experiment = &exp
				if err := o.ExperimentsAPI.LabelTrial(ctx, t.LabelsURL, experimentsv1alpha1.TrialLabels{Labels: o.Labels}); err != nil {
					return err
				}
				if err := o.Printer.PrintObj(&t, o.Out); err != nil {
					return err
				}
				labeled++
			}
		}

		if len(nums) != labeled {
			return fmt.Errorf("unable to label some trials (only \"completed\" trials can be labeled)")
		}
	}
	return nil
}
