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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"k8s.io/client-go/util/jsonpath"
)

type resourceType string

const (
	// typeExperiment is the type argument to use for experiments
	typeExperiment resourceType = "experiment"
	// typeTrial is the type argument to use for trials
	typeTrial resourceType = "trial"
)

// validTypes returns the supported object types as strings
func validTypes() []string {
	return []string{string(typeExperiment), string(typeTrial)}
}

// normalizeType returns a consistent value based on a user entered type
func normalizeType(t string) (resourceType, error) {
	switch strings.ToLower(t) {
	case "experiment", "experiments", "exp":
		return typeExperiment, nil
	case "trial", "trials", "tr":
		return typeTrial, nil
	}
	return "", fmt.Errorf("unknown resource type \"%s\"", t)
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

	// Names of the resources to work with
	Names []Identifier
}

func (o *Options) setNames(args []string) error {
	var err error
	o.Names, err = ParseNames(args)
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
type Identifier struct {
	// Type is the normalized type name being named
	Type resourceType
	// Name is the actual name (minus the type and number if present)
	Name string
	// Number is the number suffix found on the name
	Number int64
}

// ExperimentName returns the name as a typed experiment name
func (id *Identifier) ExperimentName() experimentsv1alpha1.ExperimentName {
	return experimentsv1alpha1.NewExperimentName(id.Name)
}

// numberSuffixPattern matches the trailing digits, for example the number on the end of a trial name
var numberSuffixPattern = regexp.MustCompile(`(.*?)(?:[/\-]([[:digit:]]+))?$`)

// parseNames parses a list of arguments into structured names
func ParseNames(args []string) ([]Identifier, error) {
	var t resourceType
	ids := make([]Identifier, 0, len(args))

	for _, arg := range args {
		id := Identifier{Type: t, Name: arg, Number: -1}

		if sm := numberSuffixPattern.FindStringSubmatch(id.Name); sm != nil && sm[2] != "" {
			id.Number, _ = strconv.ParseInt(sm[2], 10, 64)
			id.Name = sm[1]
		}

		p := strings.SplitN(id.Name, "/", 2)
		if len(p) > 1 || t == "" {
			nt, err := normalizeType(p[0])
			if err != nil {
				return nil, err
			}
			if len(p) > 1 {
				id.Type = nt
				id.Name = p[1]
			} else if t == "" {
				t = nt
				continue
			}
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		if t == "" {
			return nil, fmt.Errorf("required resource not specified")
		}
		ids = append(ids, Identifier{Type: t, Number: -1})
	}

	return ids, nil
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

// experimentsMeta is the metadata extraction necessary for printing Red Sky Experiments API objects
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
