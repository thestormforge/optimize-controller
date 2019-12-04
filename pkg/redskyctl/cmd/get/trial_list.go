/*
Copyright 2019 GramLabs, Inc.

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

package get

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	meta2 "github.com/redskyops/k8s-experiment/internal/meta"
	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/experiment"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	getTrialListLong    = `Prints a list of trials for an experiment using a tabular format by default`
	getTrialListExample = ``
)

// NewGetTrialListCommand returns a new get trial list command
func NewGetTrialListCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGetOptions(ioStreams)

	// We need to modify the table metadata during `Run` (i.e. once we have fetched the experiment and know the parameters and metrics)
	meta := &trialTableMeta{}
	printFlags := cmdutil.NewPrintFlags(meta)

	cmd := &cobra.Command{
		Use:     "trials NAME",
		Short:   "Display a list of trial for an experiment",
		Long:    getTrialListLong,
		Example: getTrialListExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args, printFlags))
			cmdutil.CheckErr(RunGetTrialList(o, meta))
		},
	}

	o.AddFlags(cmd)
	printFlags.AddFlags(cmd)

	return cmd
}

// RunGetTrialList gets a trial list for the given get options
func RunGetTrialList(o *GetOptions, meta *trialTableMeta) error {
	if o.RedSkyAPI != nil {
		if err := o.printIf(getRedSkyAPITrialList(o, meta)); err != nil {
			return err
		}
	}

	if o.RedSkyClientSet != nil {
		if err := o.printIf(getKubernetesTrialList(o, meta)); err != nil {
			return err
		}
	}

	return nil
}

func getRedSkyAPITrialList(o *GetOptions, meta *trialTableMeta) (*redsky.TrialList, error) {
	api := *o.RedSkyAPI

	// Get the experiment
	exp, err := api.GetExperimentByName(context.TODO(), redsky.NewExperimentName(o.Name))
	if err != nil {
		return nil, err
	}

	// Collect the parameter and metric names from the experiment
	meta.name = o.Name
	for i := range exp.Parameters {
		meta.parameters = append(meta.parameters, exp.Parameters[i].Name)
	}
	for i := range exp.Metrics {
		meta.metrics = append(meta.metrics, exp.Metrics[i].Name)
	}

	// Fetch the trial data
	tq := &redsky.TrialListQuery{Status: []redsky.TrialStatus{redsky.TrialActive, redsky.TrialCompleted, redsky.TrialFailed}}
	if exp.Trials == "" {
		return &redsky.TrialList{}, nil
	} else if tl, err := api.GetAllTrials(context.TODO(), exp.Trials, tq); err != nil {
		return nil, err
	} else {
		return filterAndSortTrials(&tl, o.Selector, o.SortBy)
	}
}

func getKubernetesTrialList(o *GetOptions, meta *trialTableMeta) (*redsky.TrialList, error) {
	clientset := o.RedSkyClientSet

	// Get the experiment
	exp, err := clientset.RedskyopsV1alpha1().Experiments(o.Namespace).Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Collect the parameter and metric names from the experiment
	meta.name = o.Name
	for i := range exp.Spec.Parameters {
		meta.parameters = append(meta.parameters, exp.Spec.Parameters[i].Name)
	}
	for i := range exp.Spec.Metrics {
		meta.metrics = append(meta.metrics, exp.Spec.Metrics[i].Name)
	}

	// Fetch the trial data
	list := &redsky.TrialList{}
	opts := metav1.ListOptions{}
	if sel, err := meta2.MatchingSelector(exp.TrialSelector()); err != nil {
		return nil, err
	} else {
		sel.ApplyToListOptions(&opts)
	}
	if tl, err := clientset.RedskyopsV1alpha1().Trials("").List(opts); err != nil {
		return nil, err
	} else if err := experiment.ConvertTrialList(tl, list); err != nil {
		return nil, err
	}
	return filterAndSortTrials(list, o.Selector, o.SortBy)
}

func filterAndSortTrials(tl *redsky.TrialList, selector, sortBy string) (*redsky.TrialList, error) {
	sel, err := labels.Parse(selector)
	if err != nil {
		return nil, err
	}

	if !sel.Empty() {
		var filtered []redsky.TrialItem
		for i := range tl.Trials {
			// TODO Add status into the label map?
			if sel.Matches(labels.Set(tl.Trials[i].Labels)) {
				filtered = append(filtered, tl.Trials[i])
			}
		}
		tl.Trials = filtered
	}

	if sortBy != "" {
		sort.Slice(tl.Trials, sortByField(sortBy, func(i int) interface{} { return sortableTrialData(&tl.Trials[i]) }))
	}

	return tl, nil
}

// Slightly modifies the schema of the trial item to make it easier to specify sort orders
func sortableTrialData(item *redsky.TrialItem) map[string]interface{} {
	assignments := make(map[string]int64, len(item.Assignments))
	for i := range item.Assignments {
		if a, err := item.Assignments[i].Value.Int64(); err == nil {
			assignments[item.Assignments[i].ParameterName] = a
		}
	}

	values := make(map[string]interface{}, len(item.Values))
	for i := range item.Values {
		v := make(map[string]float64, 2)
		v["value"] = item.Values[i].Value
		v["error"] = item.Values[i].Error
		values[item.Values[i].MetricName] = v
	}

	d := make(map[string]interface{}, 5)
	d["assignments"] = assignments
	d["labels"] = item.Labels
	d["number"] = item.Number
	d["status"] = item.Status
	d["values"] = values
	return d
}

type trialTableMeta struct {
	name       string
	parameters []string
	metrics    []string
}

func (*trialTableMeta) IsListType(obj interface{}) bool {
	if _, ok := obj.(*redsky.TrialList); ok {
		return true
	}
	return false
}

func (*trialTableMeta) ExtractList(obj interface{}) ([]interface{}, error) {
	switch o := obj.(type) {
	case *redsky.TrialList:
		list := make([]interface{}, len(o.Trials))
		for i := range o.Trials {
			list[i] = &o.Trials[i]
		}
		return list, nil
	default:
		return []interface{}{obj}, nil
	}
}

func (t *trialTableMeta) ExtractValue(obj interface{}, column string) (string, error) {
	switch o := obj.(type) {
	case *redsky.TrialItem:
		if strings.HasPrefix(column, "parameter_") {
			column = strings.TrimPrefix(column, "parameter_")
			for i := range o.Assignments {
				if o.Assignments[i].ParameterName == column {
					return o.Assignments[i].Value.String(), nil
				}
			}
		} else if strings.HasPrefix(column, "metric_") {
			column = strings.TrimPrefix(column, "metric_")
			for i := range o.Values {
				if o.Values[i].MetricName == column {
					return strconv.FormatFloat(o.Values[i].Value, 'f', -1, 64), nil
				}
			}
		} else {
			switch column {
			case "name":
				return fmt.Sprintf("%s-%d", t.name, o.Number), nil
			case "status":
				return string(o.Status), nil
			case "labels":
				var l []string
				for k, v := range o.Labels {
					l = append(l, fmt.Sprintf("%s=%s", k, v))
				}
				return strings.Join(l, ","), nil
			}
		}
	}
	return "", nil
}

func (*trialTableMeta) Allow(outputFormat string) bool {
	return outputFormat == "" || strings.ToLower(outputFormat) == "name" || strings.ToLower(outputFormat) == "csv"
}

func (t *trialTableMeta) Columns(outputFormat string) []string {
	var columns []string
	switch strings.ToLower(outputFormat) {
	case "csv":
		for _, p := range t.parameters {
			columns = append(columns, "parameter_"+p)
		}
		for _, m := range t.metrics {
			columns = append(columns, "metric_"+m)
		}
	default:
		columns = append(columns, "name", "status")
	}
	return columns
}

func (*trialTableMeta) Header(outputFormat string, column string) string {
	switch strings.ToLower(outputFormat) {
	case "csv":
		return column
	default:
		return strings.ToUpper(column)
	}
}
