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
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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

	// Assume we can parse now
	argsToParse := make([]string, len(args), len(args)+1)
	copy(argsToParse, args)
	if toComplete != "" {
		argsToParse = append(argsToParse, strings.TrimRight(toComplete, "/-"))
	}
	names, err := parseNames(argsToParse)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Use the API to get the names
	n := names[len(names)-1]
	switch n.Type {
	case typeExperiment:
		return experimentNames(ctx, api, n.Name)
	case typeTrial:
		return trialNames(ctx, api, n.Name, n.Number)
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

func experimentNames(ctx context.Context, api experimentsv1alpha1.API, prefix string) ([]string, cobra.ShellCompDirective) {
	l, err := api.GetAllExperiments(ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(l.Experiments))
	addPage := func(list *experimentsv1alpha1.ExperimentList) {
		for i := range l.Experiments {
			n := l.Experiments[i].Name()
			if strings.HasPrefix(n, prefix) {
				names = append(names, n)
			}
		}
	}

	addPage(&l)

	// Get the rest of the experiments
	next := l.Next
	for next != "" {
		n, err := api.GetAllExperimentsByPage(ctx, next)
		if err != nil {
			break
		}
		addPage(&n)
		next = n.Next
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func trialNames(ctx context.Context, api experimentsv1alpha1.API, prefix string, prefixNumber int64) ([]string, cobra.ShellCompDirective) {
	names, err := trialNamesForExperiment(ctx, api, prefix, prefixNumber)

	var eerr *experimentsv1alpha1.Error
	if errors.As(err, &eerr) && eerr.Type == experimentsv1alpha1.ErrExperimentNotFound {
		names, _ = experimentNames(ctx, api, prefix)
		for i := range names {
			names[i] = names[i] + "/"
		}
		return names, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func trialNamesForExperiment(ctx context.Context, api experimentsv1alpha1.API, prefix string, prefixNumber int64) ([]string, error) {
	// This is not implicit
	if prefix == "" {
		return nil, &experimentsv1alpha1.Error{Type: experimentsv1alpha1.ErrExperimentNotFound}
	}

	exp, err := api.GetExperimentByName(ctx, experimentsv1alpha1.NewExperimentName(prefix))
	if err != nil || exp.TrialsURL == "" {
		return nil, err
	}

	l, err := api.GetAllTrials(ctx, exp.TrialsURL, nil)
	if err != nil {
		return nil, err
	}

	sort.Slice(l.Trials, func(i, j int) bool { return l.Trials[i].Number < l.Trials[j].Number })

	// Match trials by the formatted name
	trialPrefix := prefix + "/"
	if prefixNumber >= 0 {
		trialPrefix += strconv.FormatInt(prefixNumber, 10)
	}

	names := make([]string, 0, len(l.Trials))
	for i := range l.Trials {
		n := fmt.Sprintf("%s/%d", prefix, l.Trials[i].Number)
		if strings.HasPrefix(n, trialPrefix) {
			names = append(names, n)
		}
	}
	return names, nil
}
