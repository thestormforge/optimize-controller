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
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"

	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// TODO Accept suggestion inputs from standard input, what formats?

// SuggestOptions includes the configuration for suggesting experiment trials
type SuggestOptions struct {
	Options

	Assignments      map[string]string
	AllowInteractive bool
	DefaultBehavior  string
}

// NewSuggestCommand creates a new suggestion command
func NewSuggestCommand(o *SuggestOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest NAME",
		Short: "Suggest assignments",
		Long:  "Suggest assignments for a new trial run",

		Args: cobra.ExactArgs(1),

		PreRunE: func(cmd *cobra.Command, args []string) error {
			o.Names = []Identifier{{Type: typeExperiment, Name: args[0]}}

			commander.SetStreams(&o.IOStreams, cmd)

			expAPI, err := commander.NewExperimentsAPI(cmd.Context(), o.Config)
			if err != nil {
				return err
			}

			o.ExperimentsAPI = expAPI

			return nil
		},
		RunE: commander.WithContextE(o.suggest),
	}

	cmd.Flags().StringToStringVarP(&o.Assignments, "assign", "A", nil, "Assign an explicit value to a parameter.")
	cmd.Flags().BoolVar(&o.AllowInteractive, "interactive", false, "Allow interactive prompts for unspecified parameter assignments.")
	cmd.Flags().StringVar(&o.DefaultBehavior, "default", "", "Select the behavior for default values; one of: none|min|max|rand.")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *SuggestOptions) suggest(ctx context.Context) error {
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, o.Names[0].ExperimentName())
	if err != nil {
		return err
	}

	ta, err := o.SuggestAssignments(&exp)
	if err != nil {
		return err
	}

	_, err = o.ExperimentsAPI.CreateTrial(ctx, exp.TrialsURL, *ta)
	return err
}

// SuggestAssignments creates new assignments object based on the parameters of the supplied experiment
func (o *SuggestOptions) SuggestAssignments(exp *experimentsv1alpha1.Experiment) (*experimentsv1alpha1.TrialAssignments, error) {
	ta := &experimentsv1alpha1.TrialAssignments{}
	for i := range exp.Parameters {
		p := &exp.Parameters[i]
		v, err := o.assign(p)
		if err != nil {
			return nil, err
		}
		ta.Assignments = append(ta.Assignments, experimentsv1alpha1.Assignment{ParameterName: p.Name, Value: v})
	}
	return ta, nil
}

func (o *SuggestOptions) assign(p *experimentsv1alpha1.Parameter) (json.Number, error) {
	// Look for explicit assignments
	if a, ok := o.Assignments[p.Name]; ok {
		return checkValue(p, json.Number(a))
	}

	// Compute a default value (may be needed for interactive prompt)
	def, err := o.defaultValue(p)
	if err != nil {
		return "0", err
	}

	// Collect the value interactively
	if o.AllowInteractive {
		return o.assignInteractive(p, def)
	}

	// Use the default
	if def != nil {
		return *def, nil
	}

	return "0", fmt.Errorf("no assignment for parameter: %s", p.Name)
}

func (o *SuggestOptions) defaultValue(p *experimentsv1alpha1.Parameter) (*json.Number, error) {
	switch o.DefaultBehavior {
	case "none":
		return nil, nil
	case "min":
		return &p.Bounds.Min, nil
	case "max":
		return &p.Bounds.Max, nil
	case "rand":
		return randomValue(p)
	}
	// TODO If we ever have a "status quo" or "default" value on the parameter, return it here
	return nil, nil
}

func (o *SuggestOptions) assignInteractive(p *experimentsv1alpha1.Parameter, def *json.Number) (json.Number, error) {
	if def != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "Assignment for %v parameter '%s' [%v,%v] (%v): ", p.Type, p.Name, p.Bounds.Min, p.Bounds.Max, *def)
	} else {
		_, _ = fmt.Fprintf(o.ErrOut, "Assignment for %v parameter '%s' [%v,%v]: ", p.Type, p.Name, p.Bounds.Min, p.Bounds.Max)
	}

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
			return *def, nil
		}
		number, err := checkValue(p, json.Number(text))
		if err != nil {
			continue
		}
		return number, nil
	}

	if err := s.Err(); err != nil {
		return "0", err
	}
	return "0", fmt.Errorf("no assignment for parameter: %s", p.Name)
}

func checkValue(p *experimentsv1alpha1.Parameter, n json.Number) (json.Number, error) {
	switch p.Type {
	case experimentsv1alpha1.ParameterTypeInteger:
		min, max, err := intBounds(&p.Bounds)
		if err != nil {
			return "0", err
		}
		v, err := n.Int64()
		if err != nil {
			return "0", err
		}
		if v < min || v > max {
			return "0", fmt.Errorf("value is not within experiment bounds [%d-%d]: %d", min, max, v)
		}
	case experimentsv1alpha1.ParameterTypeDouble:
		min, max, err := floatBounds(&p.Bounds)
		if err != nil {
			return "0.0", err
		}
		v, err := n.Float64()
		if err != nil {
			return "0.0", err
		}
		if v < min || v > max {
			return "0.0", fmt.Errorf("value is not within experiment bounds [%f-%f]: %f", min, max, v)
		}
	}
	return n, nil
}

func randomValue(p *experimentsv1alpha1.Parameter) (*json.Number, error) {
	switch p.Type {
	case experimentsv1alpha1.ParameterTypeInteger:
		min, max, err := intBounds(&p.Bounds)
		if err != nil {
			return nil, err
		}
		r := json.Number(strconv.FormatInt(rand.Int63n(max-min)+min, 10))
		return &r, nil
	case experimentsv1alpha1.ParameterTypeDouble:
		min, max, err := floatBounds(&p.Bounds)
		if err != nil {
			return nil, err
		}
		r := json.Number(strconv.FormatFloat(rand.Float64()*max+min, 'f', -1, 64))
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
