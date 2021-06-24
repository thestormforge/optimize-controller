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
	"sort"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetOptions includes the configuration for getting experiment API objects
type GetOptions struct {
	Options

	ChunkSize int
	SortBy    string
	Selector  string
	All       bool
}

// NewGetCommand creates a new get command
func NewGetCommand(o *GetOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get (TYPE NAME | TYPE/NAME ...)",
		Short: "Display an Optimize resource",
		Long:  "Get StormForge Optimize resources from the remote server",

		ValidArgsFunction: o.validArgs,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			if err := commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd); err != nil {
				return err
			}
			return o.setNames(args)
		},
		RunE: commander.WithContextE(o.get),
	}

	cmd.Flags().IntVar(&o.ChunkSize, "chunk-size", o.ChunkSize, "fetch large lists in chunks rather then all at once")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "selector (label `query`) to filter on")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.SortBy, "sort list types using this JSONPath `expression`")
	cmd.Flags().BoolVarP(&o.All, "all", "A", false, "include all resources")

	commander.SetPrinter(&experimentsMeta{}, &o.Printer, cmd, nil)

	return cmd
}

func (o *GetOptions) get(ctx context.Context) error {
	e := make([]experimentsv1alpha1.ExperimentName, 0, len(o.Names))
	t := make(map[experimentsv1alpha1.ExperimentName][]int64)

	for _, n := range o.Names {
		switch n.Type {

		case typeExperiment:
			if n.Name == "" {
				q := experimentsv1alpha1.ExperimentListQuery{}
				q.SetLimit(o.ChunkSize)
				return o.getExperimentList(ctx, q)
			}
			e = append(e, n.experimentName())

		case typeTrial:
			if n.trialNumber() < 0 {
				return o.getTrialList(ctx, n.experimentName(), o.trialListQuery())
			}
			key := n.experimentName()
			t[key] = append(t[key], n.trialNumber())

		default:
			return fmt.Errorf("cannot get %s", n.Type)
		}
	}

	if len(e) > 0 {
		return o.getExperiments(ctx, e)
	}

	if len(t) > 0 {
		return o.getTrials(ctx, t)
	}

	return nil
}

func (o *GetOptions) trialListQuery() experimentsv1alpha1.TrialListQuery {
	q := experimentsv1alpha1.TrialListQuery{}
	q.SetStatus(experimentsv1alpha1.TrialActive, experimentsv1alpha1.TrialCompleted, experimentsv1alpha1.TrialFailed)
	if o.All {
		q.AddStatus(experimentsv1alpha1.TrialStaged)
	}
	return q
}

func (o *GetOptions) getExperiments(ctx context.Context, names []experimentsv1alpha1.ExperimentName) error {
	// Create a list to hold the experiments
	l := &experimentsv1alpha1.ExperimentList{}
	for _, n := range names {
		exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, n)
		if err != nil {
			return err
		}
		l.Experiments = append(l.Experiments, experimentsv1alpha1.ExperimentItem{Experiment: exp})
	}

	// If this was a request for a single object, just print it out (e.g. don't produce a JSON list for a single element)
	if len(names) == 1 && len(l.Experiments) == 1 {
		return o.Printer.PrintObj(&l.Experiments[0], o.Out)
	}

	if err := o.filterAndSortExperiments(l); err != nil {
		return err
	}

	return o.Printer.PrintObj(l, o.Out)
}

func (o *GetOptions) getExperimentList(ctx context.Context, q experimentsv1alpha1.ExperimentListQuery) error {
	// Get all the experiments one page at a time
	l, err := o.ExperimentsAPI.GetAllExperiments(ctx, q)
	if err != nil {
		return err
	}

	next := l.Link(api.RelationNext)
	for next != "" {
		n, err := o.ExperimentsAPI.GetAllExperimentsByPage(ctx, next)
		if err != nil {
			return err
		}
		next = n.Link(api.RelationNext)
		l.Experiments = append(l.Experiments, n.Experiments...)
	}

	if err := o.filterAndSortExperiments(&l); err != nil {
		return err
	}

	return o.Printer.PrintObj(&l, o.Out)
}

func (o *GetOptions) getTrials(ctx context.Context, numbers map[experimentsv1alpha1.ExperimentName][]int64) error {
	l := &experimentsv1alpha1.TrialList{}

	for n, nums := range numbers {
		// Get the experiment
		exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, n)
		if err != nil {
			return err
		}

		// Get the trials
		tl, err := o.ExperimentsAPI.GetAllTrials(ctx, exp.Link(api.RelationTrials), o.trialListQuery())
		if err != nil {
			return err
		}

		for i := range tl.Trials {
			if hasTrialNumber(&tl.Trials[i], nums) {
				t := tl.Trials[i]
				t.Experiment = &exp
				l.Trials = append(l.Trials, t)
			}
		}
	}

	// If this was a request for a single object, just print it out (e.g. don't produce a JSON list for a single element)
	if len(numbers) == 1 && len(l.Trials) == 1 { // TODO Also should check the length of the map value...
		return o.Printer.PrintObj(&l.Trials[0], o.Out)
	}

	if err := o.filterAndSortTrials(l); err != nil {
		return err
	}

	return o.Printer.PrintObj(l, o.Out)
}

func (o *GetOptions) getTrialList(ctx context.Context, name experimentsv1alpha1.ExperimentName, q experimentsv1alpha1.TrialListQuery) error {
	// Get the experiment
	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, name)
	if err != nil {
		return err
	}

	// Fetch the trial data
	var l experimentsv1alpha1.TrialList
	if trialsURL := exp.Link(api.RelationTrials); trialsURL != "" {
		l, err = o.ExperimentsAPI.GetAllTrials(ctx, trialsURL, q)
		if err != nil {
			return err
		}

		// Store a back reference to the experiment on the list and every item in it
		l.Experiment = &exp
		for i := range l.Trials {
			l.Trials[i].Experiment = &exp
		}
	}

	if err := o.filterAndSortTrials(&l); err != nil {
		return err
	}

	return o.Printer.PrintObj(&l, o.Out)
}

func (o *GetOptions) filterAndSortExperiments(l *experimentsv1alpha1.ExperimentList) error {
	// Experiments do not have labels so anything but the empty selector will just nil out the list
	if sel, err := labels.Parse(o.Selector); err != nil {
		return err
	} else if !sel.Empty() {
		l.Experiments = nil
	}

	// If sorting was requested, sort using maps with all the sortable keys
	if o.SortBy != "" {
		sort.Slice(l.Experiments, sortByField(o.SortBy, func(i int) interface{} { return sortableExperimentData(&l.Experiments[i]) }))
	}

	return nil
}

// sortableExperimentData slightly modifies the schema of the experiment item to make it easier to specify sort orders
func sortableExperimentData(item *experimentsv1alpha1.ExperimentItem) map[string]interface{} {
	d := make(map[string]interface{}, 2)
	d["name"] = item.DisplayName
	d["observations"] = item.Observations
	return d
}

func (o *GetOptions) filterAndSortTrials(l *experimentsv1alpha1.TrialList) error {
	// Filter the trial list using Kubernetes label selectors
	if sel, err := labels.Parse(o.Selector); err != nil {
		return err
	} else if !sel.Empty() {
		var filtered []experimentsv1alpha1.TrialItem
		for i := range l.Trials {
			// TODO Add status into the label map?
			if sel.Matches(labels.Set(l.Trials[i].Labels)) {
				filtered = append(filtered, l.Trials[i])
			}
		}
		l.Trials = filtered
	}

	// If sorting was requested, sort using maps with all the sortable keys
	if o.SortBy != "" {
		sort.Slice(l.Trials, sortByField(o.SortBy, func(i int) interface{} { return sortableTrialData(&l.Trials[i]) }))
	}

	return nil
}

// sortableTrialData slightly modifies the schema of the trial item to make it easier to specify sort orders
func sortableTrialData(item *experimentsv1alpha1.TrialItem) map[string]interface{} {
	assignments := make(map[string]interface{}, len(item.Assignments))
	for i := range item.Assignments {
		if v := item.Assignments[i].Value; v.IsString {
			assignments[item.Assignments[i].ParameterName] = v.String()
		} else {
			assignments[item.Assignments[i].ParameterName] = v.Int64Value()
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
