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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"
)

const (
	// TypeExperimentList is the type argument to use for lists of experiments
	TypeExperimentList = "experiments"
	// TypeExperiment is the type argument to use for experiments
	TypeExperiment = "experiment"
	// TypeExperimentAliases is a comma separated list of aliases for experiment
	TypeExperimentAliases = "exp"
	// TypeTrialList is the type argument to use for lists of trials
	TypeTrialList = "trials"
	// TypeTrial is the type argument to use for trials
	TypeTrial = "trial"
)

// TypeAndNameArgs binds the "TYPE NAME..." arguments to a command
func TypeAndNameArgs(cmd *cobra.Command, opts *Options) {
	// Change the usage string to indicate what arguments are expected
	cmd.Use = cmd.Use + " TYPE NAME..."

	cmd.ValidArgs = append(cmd.ValidArgs, TypeExperimentList, TypeExperiment)
	cmd.ArgAliases = append(cmd.ArgAliases, strings.Split(TypeExperimentAliases, ",")...)

	cmd.ValidArgs = append(cmd.ValidArgs, TypeTrialList, TypeTrial)

	// Setup argument validation
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		// Require at least one argument (the type)
		if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
			return err
		}

		// First check the aliases, if it's not found, fall back to the standard "only valid args"
		for _, aa := range cmd.ArgAliases {
			if aa == args[0] {
				return nil
			}
		}
		return cobra.OnlyValidArgs(cmd, args[:1])
	}

	// Override the pre-run to capture the arguments into the supplied options instance
	commander.AddPreRunE(cmd, func(_ *cobra.Command, args []string) error {
		opts.Type = args[0]
		opts.Names = args[1:]
		return nil
	})
}

// Options are the common options for interacting with the Red Sky Experiments API
type Options struct {
	// Config is the Red Sky Control Configuration
	Config config.Config
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// Printer is the resource printer used to render objects from the Red Sky Experiments API
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Type of resource to work with
	Type string
	// Names of the resources to work with
	Names []string
}

// GetType returns the type, after expanding any aliases
func (o *Options) GetType() string {
	t := o.Type

	// Expand experiment aliases
	for _, alias := range strings.Split(TypeExperimentAliases, ",") {
		if t == alias {
			t = TypeExperiment
		}
	}

	// Dummy pluralize
	if len(o.Names) == 0 {
		t = strings.TrimSuffix(t, "s") + "s"
	}

	return t
}

// experimentsMeta is the metadata extraction necessary for printing Red Sky Experiments API objects
type experimentsMeta struct {
	base interface{}
}

func (m *experimentsMeta) ExtractList(obj interface{}) ([]interface{}, error) {
	if o, ok := obj.(*experimentsv1alpha1.ExperimentList); ok {
		list := make([]interface{}, len(o.Experiments))
		for i := range o.Experiments {
			list[i] = &o.Experiments[i]
		}
		return list, nil
	}
	if o, ok := obj.(*experimentsv1alpha1.TrialList); ok {
		list := make([]interface{}, len(o.Trials))
		for i := range o.Trials {
			list[i] = &o.Trials[i]
		}
		return list, nil
	}
	if obj != nil {
		return []interface{}{obj}, nil
	}
	return nil, nil
}

func (m *experimentsMeta) Columns(obj interface{}, outputFormat string, showLabels bool) []string {
	columns := []string{"name"}

	if tl, ok := obj.(*experimentsv1alpha1.TrialList); ok {
		if outputFormat == "csv" {
			columns = []string{"experiment", "number", "status"}

			// CSV column names should correspond to the parameter and metric names
			if exp, ok := m.base.(*experimentsv1alpha1.Experiment); ok {
				for i := range exp.Parameters {
					columns = append(columns, "parameter_"+exp.Parameters[i].Name)
				}
				for i := range exp.Metrics {
					columns = append(columns, "metric_"+exp.Metrics[i].Name)
				}
			}

			// CSV labels need to be split out into individual columns
			if showLabels {
				labels := make(map[string]bool)
				for i := range tl.Trials {
					for k := range tl.Trials[i].Labels {
						labels[k] = true
					}
				}
				for k := range labels {
					columns = append(columns, "label_"+k)
				}
			}
		} else {
			columns = append(columns, "Status") // Title case the value
			if showLabels {
				columns = append(columns, "labels")
			}
		}
	}

	return columns
}

func (m *experimentsMeta) ExtractValue(obj interface{}, column string) (string, error) {
	switch o := obj.(type) {
	case *experimentsv1alpha1.ExperimentItem:
		switch column {
		case "name":
			return o.DisplayName, nil
		}
	case *experimentsv1alpha1.TrialItem:
		switch column {
		case "experiment":
			if exp, ok := m.base.(*experimentsv1alpha1.Experiment); ok {
				return exp.DisplayName, nil
			}
			return "", nil
		case "name":
			if exp, ok := m.base.(*experimentsv1alpha1.Experiment); ok {
				return fmt.Sprintf("%s-%03d", exp.DisplayName, o.Number), nil
			}
			return strconv.FormatInt(o.Number, 10), nil
		case "number":
			return strconv.FormatInt(o.Number, 10), nil
		case "status":
			return string(o.Status), nil
		case "Status":
			return strings.Title(string(o.Status)), nil
		case "labels":
			var labels []string
			for k, v := range o.Labels {
				labels = append(labels, fmt.Sprintf("%s=%s", k, v))
			}
			return strings.Join(labels, ","), nil
		default:
			// This could be a name pattern (e.g. parameter assignment, metric value, label)
			if pn := strings.TrimPrefix(column, "parameter_"); pn != column {
				for i := range o.Assignments {
					if pn == o.Assignments[i].ParameterName {
						return o.Assignments[i].Value.String(), nil
					}
				}
			}
			if mn := strings.TrimPrefix(column, "metric_"); mn != column {
				for i := range o.Values {
					if mn == o.Values[i].MetricName {
						return strconv.FormatFloat(o.Values[i].Value, 'f', -1, 64), nil
					}
				}
				if o.Status != experimentsv1alpha1.TrialCompleted {
					return "", nil // Do not fail for missing metrics unless the trial complete
				}
			}
			if ln := strings.TrimPrefix(column, "label_"); ln != column {
				for k, v := range o.Labels {
					if ln == k {
						return v, nil
					}
				}
				return "", nil // Do not fail for missing labels, just leave it blank
			}
		}
	}
	return "", fmt.Errorf("unable to get value for column %s", column)
}

func (m *experimentsMeta) Header(outputFormat string, column string) string {
	if strings.ToLower(outputFormat) == "csv" {
		return column
	}
	return strings.ToUpper(column)
}

// sortByField sorts using a JSONPath expression
func sortByField(sortBy string, item func(int) interface{}) func(int, int) bool {
	// TODO We always wrap the items in maps now, can we simplify?
	field := sortBy // Roughly the same as RelaxedJSONPathExpression
	if strings.HasPrefix(field, "{") && strings.HasSuffix(field, "}") {
		field = strings.TrimPrefix(strings.TrimSuffix(field, "}"), "{")
	}
	field = strings.TrimPrefix(field, ".")
	field = fmt.Sprintf("{.%s}", field)

	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	if err := parser.Parse(field); err != nil {
		return nil
	}

	return func(i, j int) bool {
		ir, err := parser.FindResults(item(i))
		if err != nil || len(ir) == 0 || len(ir[0]) == 0 {
			return true
		}

		jr, err := parser.FindResults(item(j))
		if err != nil || len(jr) == 0 || len(jr[0]) == 0 {
			return false
		}

		less, _ := isLess(ir[0][0], jr[0][0])
		return less
	}
}

// isLess compares values, only int64, float64, and string are allowed
func isLess(i, j reflect.Value) (bool, error) {
	switch i.Kind() {
	case reflect.Int64:
		return i.Int() < j.Int(), nil
	case reflect.Float64:
		return i.Float() < j.Float(), nil
	case reflect.String:
		return i.String() < j.String(), nil // TODO Improve the sort order
	default:
		return false, fmt.Errorf("unsortable type: %v", i.Kind())
	}
}
