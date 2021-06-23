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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
)

// Completion implements argument completion for the type/name arguments.
func Completion(ctx context.Context, api experimentsv1alpha1.API, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Start by suggesting a type
	if len(args) == 0 {
		var s []string
		if t := string(typeExperiment); strings.HasPrefix(t, toComplete) {
			s = append(s, t)
		}
		if t := string(typeTrial); strings.HasPrefix(t, toComplete) {
			s = append(s, t)
		}
		return s, cobra.ShellCompDirectiveNoFileComp
	}

	// Assume we can parse names now
	argsToParse := make([]string, len(args), len(args)+1)
	copy(argsToParse, args)
	if toComplete != "" {
		argsToParse = append(argsToParse, strings.TrimRight(toComplete, "/-"))
	}
	names, err := parseNames(argsToParse)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Use the API to get the suggestions based on the type of the last entry
	switch names[len(names)-1].Type {
	case typeExperiment:
		return experimentNames(ctx, api, names)
	case typeTrial:
		return trialNames(ctx, api, names)
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

func experimentNames(ctx context.Context, expAPI experimentsv1alpha1.API, ns names) (completions []string, directive cobra.ShellCompDirective) {
	directive = cobra.ShellCompDirectiveNoFileComp
	addPage := func(list *experimentsv1alpha1.ExperimentList) {
		for i := range list.Experiments {
			n := list.Experiments[i].Name()
			if ns.suggest(n) {
				completions = append(completions, n)
			}
		}
	}

	// Get the first page of experiments
	l, err := expAPI.GetAllExperiments(ctx, experimentsv1alpha1.ExperimentListQuery{})
	if err != nil {
		return
	}
	addPage(&l)
	next := l.Link(api.RelationNext)

	// Get the rest of the experiments
	for next != "" {
		n, err := expAPI.GetAllExperimentsByPage(ctx, next)
		if err != nil {
			break
		}
		addPage(&n)
		next = n.Link(api.RelationNext)
	}

	return
}

func trialNames(ctx context.Context, expAPI experimentsv1alpha1.API, ns names) ([]string, cobra.ShellCompDirective) {
	name := ns[len(ns)-1].experimentName()
	trials, err := getAllTrials(ctx, expAPI, name)

	// When the experiment name is invalid, assume we need to suggest experiments names with trailing "/"
	var eerr *api.Error
	if errors.As(err, &eerr) && eerr.Type == experimentsv1alpha1.ErrExperimentNotFound {
		completions, directive := experimentNames(ctx, expAPI, ns)
		for i := range completions {
			completions[i] = completions[i] + "/"
		}
		return completions, directive | cobra.ShellCompDirectiveNoSpace
	}

	completions := make([]string, 0, len(trials.Trials))
	for i := range trials.Trials {
		n := fmt.Sprintf("%s/%d", name.Name(), trials.Trials[i].Number)
		if ns.suggest(n) {
			completions = append(completions, n)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

type names []name

func (ns names) suggest(name string) bool {
	for i := range ns {
		if i == len(ns)-1 {
			return strings.HasPrefix(name, ns[i].String())
		} else if name == ns[i].String() {
			return false
		}
	}

	return false
}

func getAllTrials(ctx context.Context, expAPI experimentsv1alpha1.API, name experimentsv1alpha1.ExperimentName) (*experimentsv1alpha1.TrialList, error) {
	// Check for an empty name as this does not happen automatically
	if name.Name() == "" {
		return nil, &api.Error{Type: experimentsv1alpha1.ErrExperimentNotFound}
	}

	// Return if we can't get the experiment or if it does not have a trial list
	exp, err := expAPI.GetExperimentByName(ctx, name)
	if err != nil {
		return nil, err
	}
	trialsURL := exp.Link(api.RelationTrials)
	if trialsURL == "" {
		return nil, nil
	}

	// Get all the trials and sort them by number
	result, err := expAPI.GetAllTrials(ctx, trialsURL, experimentsv1alpha1.TrialListQuery{})
	if err != nil {
		return nil, err
	}

	sort.Slice(result.Trials, func(i, j int) bool {
		return result.Trials[i].Number < result.Trials[j].Number
	})

	return &result, nil
}
