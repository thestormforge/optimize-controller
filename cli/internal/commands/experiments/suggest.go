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
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/validation"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1/numstr"
)

// TODO Accept suggestion inputs from standard input, what formats?

const (
	DefaultNone     = "none"
	DefaultMinimum  = "min"
	DefaultMaximum  = "max"
	DefaultRandom   = "rand"
	DefaultBaseline = "base"
)

// SuggestOptions includes the configuration for suggesting experiment trials
type SuggestOptions struct {
	Options

	Assignments      map[string]string
	AllowInteractive bool
	DefaultBehavior  string
	Labels           string
	Baselines        map[string]*numstr.NumberOrString
}

// NewSuggestCommand creates a new suggestion command
func NewSuggestCommand(o *SuggestOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest NAME",
		Short: "Suggest assignments",
		Long:  "Suggest assignments for a new trial run",

		Args: cobra.ExactArgs(1),

		PreRunE: func(cmd *cobra.Command, args []string) error {
			o.Names = []name{{Type: typeExperiment, Name: args[0]}}
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: commander.WithContextE(o.suggest),
	}

	cmd.Flags().StringToStringVarP(&o.Assignments, "assign", "A", nil, "assign an explicit `key=value` to a parameter")
	cmd.Flags().BoolVar(&o.AllowInteractive, "interactive", false, "allow interactive prompts for unspecified parameter assignments")
	cmd.Flags().StringVar(&o.DefaultBehavior, "default", "", "select the `behavior` for default values")
	cmd.Flags().StringVarP(&o.Labels, "labels", "l", "", "comma separated `key=value` labels to apply to the trial")

	commander.SetFlagValues(cmd, "default", DefaultNone, DefaultMinimum, DefaultMaximum, DefaultRandom)

	return cmd
}

func (o *SuggestOptions) suggest(ctx context.Context) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, o.Names[0].experimentName())
	if err != nil {
		return err
	}

	ta := experimentsv1alpha1.TrialAssignments{}
	if err := o.SuggestAssignments(&exp, &ta); err != nil {
		return err
	}
	if err := o.AddLabels(&ta); err != nil {
		return err
	}

	_, err = o.ExperimentsAPI.CreateTrial(ctx, exp.Link(api.RelationTrials), ta)
	if err != nil {
		return err
	}

	return nil
}

// SuggestAssignments creates new assignments object based on the parameters of the supplied experiment
func (o *SuggestOptions) SuggestAssignments(exp *experimentsv1alpha1.Experiment, ta *experimentsv1alpha1.TrialAssignments) error {
	for i := range exp.Parameters {
		p := &exp.Parameters[i]
		v, err := o.assign(p)
		if err != nil {
			return err
		}
		ta.Assignments = append(ta.Assignments, experimentsv1alpha1.Assignment{
			ParameterName: p.Name,
			Value:         *v,
		})
	}

	if err := validation.CheckConstraints(exp.Constraints, ta.Assignments); err != nil {
		return err
	}

	return nil
}

func (o *SuggestOptions) assign(p *experimentsv1alpha1.Parameter) (*numstr.NumberOrString, error) {
	// Look for explicit assignments
	if a, ok := o.Assignments[p.Name]; ok {
		return checkValue(p, a)
	}

	// Compute a default value (may be needed for interactive prompt)
	def, err := o.defaultValue(p)
	if err != nil {
		return nil, err
	}

	// Collect the value interactively
	if o.AllowInteractive {
		return o.assignInteractive(p, def)
	}

	// Use the default
	if def != nil {
		return def, nil
	}

	return nil, fmt.Errorf("no assignment for parameter: %s", p.Name)
}

func (o *SuggestOptions) AddLabels(ta *experimentsv1alpha1.TrialAssignments) error {
	if o.Labels == "" {
		return nil
	}

	ta.Labels = make(map[string]string)
	for _, l := range strings.Split(o.Labels, ",") {
		if p := strings.SplitN(l, "=", 2); len(p) == 2 {
			ta.Labels[p[0]] = p[1]
		} else if strings.HasSuffix(l, "-") && strings.Trim(l, "-") != "" {
			ta.Labels[strings.TrimSuffix(l, "-")] = ""
		}
	}
	return nil
}

func (o *SuggestOptions) defaultValue(p *experimentsv1alpha1.Parameter) (*numstr.NumberOrString, error) {
	switch o.DefaultBehavior {
	case DefaultNone, "":
		return nil, nil
	case DefaultMinimum, "minimum":
		return p.LowerBound()
	case DefaultMaximum, "maximum":
		return p.UpperBound()
	case DefaultRandom, "random":
		return randomValue(p)
	case DefaultBaseline, "baseline":
		return o.Baselines[p.Name], nil
	default:
		return nil, fmt.Errorf("unknown default behavior: %q", o.DefaultBehavior)
	}
}

func (o *SuggestOptions) assignInteractive(p *experimentsv1alpha1.Parameter, def *numstr.NumberOrString) (*numstr.NumberOrString, error) {
	_, _ = fmt.Fprint(o.ErrOut, prompt(p, def))
	s := bufio.NewScanner(o.In)
	for attempts := 0; attempts < 3; attempts++ {
		if attempts > 0 {
			_, _ = fmt.Fprintf(o.Out, "Invalid assignment, try again: ")
		}
		if !s.Scan() {
			break
		}
		text := s.Text()
		if text == "" && def != nil {
			return def, nil
		}
		result, err := checkValue(p, text)
		if err != nil {
			continue
		}
		return result, nil
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("no assignment for parameter: %s", p.Name)
}

func prompt(p *experimentsv1alpha1.Parameter, def *numstr.NumberOrString) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Assignment for %v parameter '%s'", p.Type, p.Name))

	// Add the bounds
	if p.Type == experimentsv1alpha1.ParameterTypeCategorical {
		b.WriteString(fmt.Sprintf(" [%s]", strings.Join(p.Values, ", ")))
	} else if p.Bounds != nil {
		b.WriteString(fmt.Sprintf(" [%v,%v]", p.Bounds.Min, p.Bounds.Max))
	}

	// Add the default
	if def != nil {
		b.WriteString(fmt.Sprintf(" (%s)", def.String()))
	}

	b.WriteString(": ")
	return b.String()
}

func checkValue(p *experimentsv1alpha1.Parameter, s string) (*numstr.NumberOrString, error) {
	v, err := p.ParseValue(s)
	if err != nil {
		return nil, err
	}
	err = experimentsv1alpha1.CheckParameterValue(p, v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func randomValue(p *experimentsv1alpha1.Parameter) (*numstr.NumberOrString, error) {
	switch p.Type {
	case experimentsv1alpha1.ParameterTypeInteger:
		min, max, err := intBounds(p.Bounds)
		if err != nil {
			return nil, err
		}
		r := numstr.FromInt64(rand.Int63n(max-min) + min)
		return &r, nil
	case experimentsv1alpha1.ParameterTypeDouble:
		min, max, err := floatBounds(p.Bounds)
		if err != nil {
			return nil, err
		}
		r := numstr.FromFloat64(rand.Float64()*max + min)
		return &r, nil
	case experimentsv1alpha1.ParameterTypeCategorical:
		r := numstr.FromString(p.Values[rand.Intn(len(p.Values))])
		return &r, nil
	}
	return nil, fmt.Errorf("unable to produce random %v", p.Type)
}

func intBounds(b *experimentsv1alpha1.Bounds) (int64, int64, error) {
	min, err := b.Min.Int64()
	if err != nil {
		return 0, 0, err
	}
	max, err := b.Max.Int64()
	if err != nil {
		return 0, 0, err
	}
	return min, max, nil
}

func floatBounds(b *experimentsv1alpha1.Bounds) (float64, float64, error) {
	min, err := b.Min.Float64()
	if err != nil {
		return 0, 0, err
	}
	max, err := b.Max.Float64()
	if err != nil {
		return 0, 0, err
	}
	return min, max, err
}
