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

package server

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/trial"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Finalizer is used to ensure synchronization with the server
	Finalizer = "serverFinalizer.redskyops.dev"
)

// TODO Split this into trial.go and experiment.go ?

// FromCluster converts cluster state to API state
func FromCluster(in *redskyv1beta1.Experiment) (redskyapi.ExperimentName, *redskyapi.Experiment) {
	out := &redskyapi.Experiment{}
	out.ExperimentMeta.LastModified = in.CreationTimestamp.Time
	out.ExperimentMeta.SelfURL = in.Annotations[redskyv1beta1.AnnotationExperimentURL]
	out.ExperimentMeta.NextTrialURL = in.Annotations[redskyv1beta1.AnnotationNextTrialURL]

	out.Optimization = nil
	for _, o := range in.Spec.Optimization {
		out.Optimization = append(out.Optimization, redskyapi.Optimization{
			Name:  o.Name,
			Value: o.Value,
		})
	}

	out.Parameters = nil
	for _, p := range in.Spec.Parameters {
		// This is a special case to omit parameters client side
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

	out.Constraints = nil
	for _, c := range in.Spec.Constraints {
		switch {
		case c.Order != nil:
			out.Constraints = append(out.Constraints, redskyapi.Constraint{
				Name:           c.Name,
				ConstraintType: redskyapi.ConstraintOrder,
				OrderConstraint: redskyapi.OrderConstraint{
					LowerParameter: c.Order.LowerParameter,
					UpperParameter: c.Order.UpperParameter,
				},
			})
		case c.Sum != nil:
			sc := redskyapi.SumConstraint{
				IsUpperBound: c.Sum.IsUpperBound,
				Bound:        float64(c.Sum.Bound.MilliValue()) / 1000,
			}
			for _, p := range c.Sum.Parameters {
				// This is a special case to omit parameters client side
				if p.Weight.IsZero() {
					continue
				}

				sc.Parameters = append(sc.Parameters, redskyapi.SumConstraintParameter{
					Name:   p.Name,
					Weight: float64(p.Weight.MilliValue()) / 1000,
				})
			}

			out.Constraints = append(out.Constraints, redskyapi.Constraint{
				Name:           c.Name,
				ConstraintType: redskyapi.ConstraintSum,
				SumConstraint:  sc,
			})
		}
	}

	out.Metrics = nil
	for _, m := range in.Spec.Metrics {
		out.Metrics = append(out.Metrics, redskyapi.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
		})
	}

	n := redskyapi.NewExperimentName(in.Name)
	return n, out
}

// ToCluster converts API state to cluster state
func ToCluster(exp *redskyv1beta1.Experiment, ee *redskyapi.Experiment) {
	if exp.GetAnnotations() == nil {
		exp.SetAnnotations(make(map[string]string))
	}

	exp.GetAnnotations()[redskyv1beta1.AnnotationExperimentURL] = ee.SelfURL
	exp.GetAnnotations()[redskyv1beta1.AnnotationNextTrialURL] = ee.NextTrialURL

	exp.Spec.Optimization = nil
	for i := range ee.Optimization {
		exp.Spec.Optimization = append(exp.Spec.Optimization, redskyv1beta1.Optimization{
			Name:  ee.Optimization[i].Name,
			Value: ee.Optimization[i].Value,
		})
	}

	controllerutil.AddFinalizer(exp, Finalizer)
}

// ToClusterTrial converts API state to cluster state
func ToClusterTrial(t *redskyv1beta1.Trial, suggestion *redskyapi.TrialAssignments) {
	if t.GetAnnotations() != nil {
		t.GetAnnotations()[redskyv1beta1.AnnotationReportTrialURL] = suggestion.SelfURL
	}

	// Try to make the cluster trial names match what is on the server
	if t.Name == "" && t.GenerateName != "" && suggestion.SelfURL != "" {
		name := path.Base(suggestion.SelfURL)
		if num, err := strconv.ParseInt(name, 10, 64); err == nil {
			t.Name = fmt.Sprintf("%s%03d", t.GenerateName, num)
		} else {
			t.Name = t.GenerateName + name
		}
	}

	for _, a := range suggestion.Assignments {
		if v, err := a.Value.Int64(); err == nil {
			t.Spec.Assignments = append(t.Spec.Assignments, redskyv1beta1.Assignment{
				Name:  a.ParameterName,
				Value: v,
			})
		}
	}

	trial.UpdateStatus(t)

	controllerutil.AddFinalizer(t, Finalizer)
}

// FromClusterTrial converts cluster state to API state
func FromClusterTrial(in *redskyv1beta1.Trial) *redskyapi.TrialValues {
	out := &redskyapi.TrialValues{}

	// Check to see if the trial failed
	for _, c := range in.Status.Conditions {
		if c.Type == redskyv1beta1.TrialFailed && c.Status == corev1.ConditionTrue {
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

	return out
}

// StopExperiment updates the experiment in the event that it should be paused or halted
func StopExperiment(exp *redskyv1beta1.Experiment, err error) bool {
	if rse, ok := err.(*redskyapi.Error); ok && rse.Type == redskyapi.ErrExperimentStopped {
		exp.SetReplicas(0)
		delete(exp.GetAnnotations(), redskyv1beta1.AnnotationNextTrialURL)
		return true
	}
	return false
}
