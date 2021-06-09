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
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	"k8s.io/client-go/util/jsonpath"
)

type resourceType string

const (
	// typeExperiment is the type argument to use for experiments
	typeExperiment resourceType = "experiment"
	// typeTrial is the type argument to use for trials
	typeTrial resourceType = "trial"
)

// normalizeType returns a consistent value based on a user entered type. The returned plural type only
// preserves the plural form of the input.
func normalizeType(t string) (normalType resourceType, pluralType string, err error) {
	// NOTE: We always return one of the `resourceType` constants, even if the input is plural
	switch strings.ToLower(t) {
	case "experiment", "exp":
		return typeExperiment, string(typeExperiment), nil
	case "experiments":
		return typeExperiment, string(typeExperiment) + "s", nil
	case "trial", "tr":
		return typeTrial, string(typeTrial), nil
	case "trials":
		return typeTrial, string(typeTrial) + "s", nil
	}
	return "", "", fmt.Errorf("unknown resource type \"%s\"", t)
}

// Options are the common options for interacting with the Optimize Experiments API
type Options struct {
	// Config is the Optimize Configuration
	Config *config.OptimizeConfig
	// ExperimentsAPI is used to interact with the Optimize Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// Printer is the resource printer used to render objects from the Optimize Experiments API
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Names of the resources to work with
	Names []name
}

func (o *Options) validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// The experiment API will not be set when we are getting completions
	// NOTE: The context is not set on the `cmd` (see Cobra #1263), use the parent as a workaround
	if err := commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd.Parent()); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return Completion(cmd.Parent().Context(), o.ExperimentsAPI, args, toComplete)
}

func (o *Options) setNames(args []string) error {
	var err error
	o.Names, err = parseNames(args)
	return err
}

// hasTrialNumber checks to see if a trail's number is in the supplied list
func hasTrialNumber(t *experimentsv1alpha1.TrialItem, nums []int64) bool {
	for _, n := range nums {
		if t.Number == n || n < 0 {
			return true
		}
	}
	return false
}

// name is construct for identifying an object in the Experiments API
type name struct {
	// Type is the normalized type name being named
	Type resourceType
	// Name is the actual name (minus the type and number if present)
	Name string
	// Number is the number suffix found on the name
	Number int64
}

// experimentName returns the name as a typed experiment name
func (n *name) experimentName() experimentsv1alpha1.ExperimentName {
	return experimentsv1alpha1.NewExperimentName(n.Name)
}

// trialNumber returns the trial number extracted from the name, values less then zero indicate no
// number is present.
func (n *name) trialNumber() int64 {
	return n.Number
}

// String returns the joined name and number.
func (n *name) String() string {
	if n.Name == "" || n.Type == typeExperiment || n.Number < 0 {
		return n.Name
	}
	return fmt.Sprintf("%s/%d", n.Name, n.Number)
}

// parseNames parses a list of arguments into structured names
func parseNames(args []string) ([]name, error) {
	names := make([]name, 0, len(args))

	var defaultType string
	for _, arg := range args {
		// Split the argument into type/name/number
		argType, argName, argNumber := splitArg(arg, defaultType)
		if argType == "" && argName == "" && argNumber == "" {
			return nil, fmt.Errorf("invalid name: %s", arg)
		}
		argIsTypeOnly := argName == "" && argNumber == ""

		// Normalize the type (note: the plural type only preserves plurality)
		normalType, pluralType, err := normalizeType(argType)
		if err != nil {
			return nil, err
		}
		typeIsPlural := string(normalType) != pluralType

		// Set the default type if it hasn't been set and no name was supplied
		if defaultType == "" && argIsTypeOnly {
			defaultType = pluralType
			continue
		}

		// Create a new name
		n := name{Type: normalType, Name: argName, Number: -1}

		// Special case where trial can alternatively end with "-<NUM>" instead of "/<NUM>"
		if n.Type == typeTrial && argNumber == "" && !typeIsPlural {
			if pos := strings.LastIndex(argName, "-"); pos > 0 {
				if _, err := strconv.ParseInt(argName[pos+1:], 10, 64); err == nil {
					n.Name, argNumber = argName[0:pos], argName[pos+1:]
				}
			}
		}

		// Parse the number, if present
		if argNumber != "" {
			if n.Type != typeTrial {
				return nil, fmt.Errorf("%s name cannot include a number: %s", n.Type, arg)
			}
			n.Number, err = strconv.ParseInt(argNumber, 10, 64)
			if err != nil {
				return nil, err
			}
		}

		names = append(names, n)
	}

	// If no names were generated we can just use the default type
	if len(names) == 0 {
		if defaultType == "" {
			return nil, fmt.Errorf("required resource not specified")
		}
		normalType, _, err := normalizeType(defaultType)
		if err != nil {
			return nil, err
		}
		names = append(names, name{Type: normalType, Number: -1})
	}

	return names, nil
}

func splitArg(arg, defType string) (parsedType string, parsedName string, parsedNumber string) {
	p := strings.Split(arg, "/")
	switch len(p) {
	case 1:
		// type | name
		if defType != "" {
			return defType, p[0], ""
		}
		return p[0], "", ""
	case 2:
		// type/name | name/number
		if _, err := strconv.ParseInt(p[1], 10, 64); err != nil {
			return p[0], p[1], ""
		}
		return defType, p[0], p[1]
	case 3:
		// type/name/number
		if p[2] == "" {
			p[2] = "-1"
		}
		return p[0], p[1], p[2]
	default:
		return "", "", ""
	}
}

// verbPrinter
type verbPrinter struct {
	verb string
}

func (v *verbPrinter) PrintObj(obj interface{}, w io.Writer) error {
	switch o := obj.(type) {
	case *experimentsv1alpha1.Experiment:
		_, _ = fmt.Fprintf(w, "experiment \"%s\" %s\n", o.DisplayName, v.verb)
	case *experimentsv1alpha1.TrialItem:
		_, _ = fmt.Fprintf(w, "trial \"%s-%03d\" %s\n", o.Experiment.DisplayName, o.Number, v.verb)
	default:
		return fmt.Errorf("could not print \"%s\" for: %T", v.verb, obj)
	}
	return nil
}

// experimentsMeta is the metadata extraction necessary for printing Optimize Experiments API objects
type experimentsMeta struct{}

// ExtractList returns the items from an API list object
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

// Columns returns the column names to use
func (m *experimentsMeta) Columns(obj interface{}, outputFormat string, showLabels bool) []string {
	// Special case for trial list CSV to include everything as columns
	if tl, ok := obj.(*experimentsv1alpha1.TrialList); ok && outputFormat == "csv" {
		columns := []string{"experiment", "number", "status"}

		// CSV column names should correspond to the parameter and metric names
		if tl.Experiment != nil {
			for i := range tl.Experiment.Parameters {
				columns = append(columns, "parameter_"+tl.Experiment.Parameters[i].Name)
			}
			for i := range tl.Experiment.Metrics {
				columns = append(columns, "metric_"+tl.Experiment.Metrics[i].Name)
			}
		}

		// Add the failure reason and message
		columns = append(columns, "failureReason", "failureMessage")

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

		return columns
	}

	// Columns are less complex in other cases
	columns := []string{"name"}
	switch obj.(type) {

	case *experimentsv1alpha1.TrialList, *experimentsv1alpha1.TrialItem:
		columns = append(columns, "Status") // Title case the value

	case *experimentsv1alpha1.ExperimentList, *experimentsv1alpha1.ExperimentItem:
		if outputFormat == "wide" {
			columns = append(columns, "observations")
		}
	}

	if showLabels {
		columns = append(columns, "labels")
	}

	return columns
}

// ExtractValue returns a cell value
func (m *experimentsMeta) ExtractValue(obj interface{}, column string) (string, error) {
	switch o := obj.(type) {
	case *experimentsv1alpha1.ExperimentItem:
		switch column {
		case "name":
			return o.Name(), nil
		case "Name":
			return o.DisplayName, nil
		case "observations":
			return strconv.FormatInt(o.Observations, 10), nil
		case "labels":
			var labels []string
			for k, v := range o.Labels {
				labels = append(labels, fmt.Sprintf("%s=%s", k, v))
			}
			return strings.Join(labels, ","), nil
		}
	case *experimentsv1alpha1.TrialItem:
		switch column {
		case "experiment":
			if o.Experiment != nil {
				return o.Experiment.DisplayName, nil
			}
			return "", nil
		case "name":
			if o.Experiment != nil {
				return fmt.Sprintf("%s-%03d", o.Experiment.DisplayName, o.Number), nil
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
		case "failureReason":
			return o.FailureReason, nil
		case "failureMessage":
			return o.FailureMessage, nil
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

// Header returns the header name to use for a column
func (m *experimentsMeta) Header(outputFormat string, column string) string {
	if strings.ToLower(outputFormat) == "csv" {
		return column
	}
	column = regexp.MustCompile("(.)([A-Z])").ReplaceAllString(column, "$1 $2")
	return strings.ToUpper(column)
}

// sortByField sorts using a JSONPath expression
func sortByField(sortBy string, item func(int) interface{}) func(int, int) bool {
	// TODO We always wrap the items in maps now, can we simplify?
	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	if err := parser.Parse(relaxedJSONPathExpression(sortBy)); err != nil {
		return func(i int, j int) bool { return i < j }
	}

	return func(i, j int) bool {
		ir, ierr := parser.FindResults(item(i))
		iok := ierr == nil && len(ir) > 0 && len(ir[0]) > 0 && ir[0][0].CanInterface()

		jr, jerr := parser.FindResults(item(j))
		jok := jerr == nil && len(jr) > 0 && len(jr[0]) > 0 && jr[0][0].CanInterface()

		if iok && jok && ir[0][0].Kind() == jr[0][0].Kind() {
			jv := jr[0][0].Interface()
			switch iv := ir[0][0].Interface().(type) {
			case int64:
				return iv < jv.(int64)
			case float64:
				return iv < jv.(float64)
			case string:
				return iv < jv.(string) // TODO Improve the sort order
			}
		}

		return i < j
	}
}

func relaxedJSONPathExpression(expr string) string {
	// Roughly the same as RelaxedJSONPathExpression in kubectl
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimPrefix(strings.TrimSuffix(expr, "}"), "{")
	}
	expr = strings.TrimPrefix(expr, ".")
	if expr == "" {
		return "{$}"
	}
	return fmt.Sprintf("{.%s}", expr)
}
