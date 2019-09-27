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
package experiment

import (
	"encoding/json"
	"path"
	"strconv"

	redskyapi "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// NOTE: The convert list methods DO NOT nil the target slice to allow for converting paged result sets

// ConvertExperiment copies a Kubernetes experiment into a Red Sky API experiment
func ConvertExperiment(in *redskyv1alpha1.Experiment, out *redskyapi.Experiment) error {
	out.ExperimentMeta.LastModified = in.CreationTimestamp.Time
	out.ExperimentMeta.Self = in.Annotations[redskyv1alpha1.AnnotationExperimentURL]
	out.ExperimentMeta.NextTrial = in.Annotations[redskyv1alpha1.AnnotationNextTrialURL]

	out.Optimization = redskyapi.Optimization{}
	if in.Spec.Parallelism != nil {
		out.Optimization.ParallelTrials = *in.Spec.Parallelism
	} else {
		out.Optimization.ParallelTrials = in.GetReplicas()
	}
	if in.Spec.BurnIn != nil {
		out.Optimization.BurnIn = *in.Spec.BurnIn
	}
	if in.Spec.Budget != nil {
		out.Optimization.ExperimentBudget = *in.Spec.Budget
	}

	out.Parameters = nil
	for _, p := range in.Spec.Parameters {
		// This is a special case for testing
		if p.Min == p.Max {
			continue
		}

		out.Parameters = append(out.Parameters, redskyapi.Parameter{
			Type: redskyapi.ParameterTypeInteger,
			Name: p.Name,
			Bounds: redskyapi.Bounds{
				Min: json.Number(strconv.FormatInt(p.Min, 10)),
				Max: json.Number(strconv.FormatInt(p.Max, 10)),
			},
		})
	}

	out.Metrics = nil
	for _, m := range in.Spec.Metrics {
		out.Metrics = append(out.Metrics, redskyapi.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
		})
	}

	return nil
}

// ConvertExperimentList copies a Kubernetes experiment list into a Red Sky API experiment list.
// Note that the output slice is not cleared, allowing this method to be used with paged results.
func ConvertExperimentList(in *redskyv1alpha1.ExperimentList, out *redskyapi.ExperimentList) error {
	for i := range in.Items {
		e := redskyapi.ExperimentItem{}
		if err := ConvertExperiment(&in.Items[i], &e.Experiment); err != nil {
			return err
		}

		// TODO This is here as a hack for `redskyctl get experiments`
		if e.Self == "" {
			e.DisplayName = in.Items[i].Name
			e.Self = path.Join(".", e.DisplayName)
		}

		out.Experiments = append(out.Experiments, e)
	}

	return nil
}

// ConvertTrialAssignments copies the assignments of a Kubernetes trial into a Red Sky API trial assignments
func ConvertTrialAssignements(in *redskyv1alpha1.Trial, out *redskyapi.TrialAssignments) error {
	out.Assignments = nil
	for i := range in.Spec.Assignments {
		out.Assignments = append(out.Assignments, redskyapi.Assignment{
			ParameterName: in.Spec.Assignments[i].Name,
			Value:         json.Number(strconv.FormatInt(in.Spec.Assignments[i].Value, 10)),
		})
	}

	return nil
}

// ConvertTiralValues copies the values of a Kubernetes trial into a Red Sky API trial values
func ConvertTrialValues(in *redskyv1alpha1.Trial, out *redskyapi.TrialValues) error {
	// Check to see if the trial failed
	for _, c := range in.Status.Conditions {
		if c.Type == redskyv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
			out.Failed = true
		}
	}

	// Record the values only if we didn't fail
	out.Values = nil
	if !out.Failed {
		for _, v := range in.Spec.Values {
			if fv, err := strconv.ParseFloat(v.Value, 64); err == nil {
				value := redskyapi.Value{
					MetricName: v.Name,
					Value:      fv,
				}
				if ev, err := strconv.ParseFloat(v.Error, 64); err == nil {
					value.Error = ev
				}
				out.Values = append(out.Values, value)
			}
		}
	}

	return nil
}

// ConvertTrialList copies a Kubernetes trial into a Red Sky API trial list.
// Note that the output slice is not cleared, allowing this method to be used with paged results.
func ConvertTrialList(in *redskyv1alpha1.TrialList, out *redskyapi.TrialList) error {
	for i := range in.Items {
		t := redskyapi.TrialItem{}
		if err := ConvertTrialAssignements(&in.Items[i], &t.TrialAssignments); err != nil {
			return err
		}
		if err := ConvertTrialValues(&in.Items[i], &t.TrialValues); err != nil {
			return err
		}
		// TODO Copy labels over? Status?
		out.Trials = append(out.Trials, t)
	}

	return nil
}
