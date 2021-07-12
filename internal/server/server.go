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
	"math"
	"path"
	"strconv"
	"strings"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/trial"
	"github.com/thestormforge/optimize-controller/v2/internal/validation"
	"github.com/thestormforge/optimize-go/pkg/api"
	"github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Finalizer is used to ensure synchronization with the server
	Finalizer = "serverFinalizer.stormforge.io"
)

// TODO Split this into trial.go and experiment.go ?

// FromCluster converts cluster state to API state
func FromCluster(in *optimizev1beta2.Experiment) (experimentsv1alpha1.ExperimentName, *experimentsv1alpha1.Experiment, *experimentsv1alpha1.TrialAssignments, error) {
	out := &experimentsv1alpha1.Experiment{}

	baseline := &experimentsv1alpha1.TrialAssignments{Labels: map[string]string{"baseline": "true"}}

	if l := len(in.ObjectMeta.Labels); l > 0 {
		out.Labels = make(map[string]string, l)
		for k, v := range in.ObjectMeta.Labels {
			k = strings.TrimPrefix(k, "stormforge.io/")
			out.Labels[k] = v
		}
	}

	out.Optimization = nil
	for _, o := range in.Spec.Optimization {
		out.Optimization = append(out.Optimization, experimentsv1alpha1.Optimization{
			Name:  o.Name,
			Value: o.Value,
		})
	}

	out.Parameters = nil
	for _, p := range in.Spec.Parameters {
		// This is a special case to omit parameters client side
		if p.Min == p.Max && len(p.Values) == 0 {
			continue
		}

		if len(p.Values) > 0 {
			out.Parameters = append(out.Parameters, experimentsv1alpha1.Parameter{
				Type:   experimentsv1alpha1.ParameterTypeCategorical,
				Name:   p.Name,
				Values: p.Values,
			})
		} else {
			out.Parameters = append(out.Parameters, experimentsv1alpha1.Parameter{
				Type: experimentsv1alpha1.ParameterTypeInteger,
				Name: p.Name,
				Bounds: &experimentsv1alpha1.Bounds{
					Min: json.Number(strconv.FormatInt(int64(p.Min), 10)),
					Max: json.Number(strconv.FormatInt(int64(p.Max), 10)),
				},
			})
		}

		if p.Baseline != nil {
			var v api.NumberOrString
			if p.Baseline.Type == intstr.String {
				vs := p.Baseline.StrVal
				if !stringSliceContains(p.Values, vs) {
					return "", nil, nil, fmt.Errorf("baseline out of range for parameter '%s'", p.Name)
				}
				v = api.FromString(vs)
			} else {
				vi := p.Baseline.IntVal
				if vi < p.Min || vi > p.Max {
					return "", nil, nil, fmt.Errorf("baseline out of range for parameter '%s'", p.Name)
				}
				v = api.FromInt64(int64(vi))
			}
			baseline.Assignments = append(baseline.Assignments, experimentsv1alpha1.Assignment{
				ParameterName: p.Name,
				Value:         v,
			})
		}
	}

	out.Constraints = nil
	for _, c := range in.Spec.Constraints {
		switch {
		case c.Order != nil:
			out.Constraints = append(out.Constraints, experimentsv1alpha1.Constraint{
				Name:           c.Name,
				ConstraintType: experimentsv1alpha1.ConstraintOrder,
				OrderConstraint: &experimentsv1alpha1.OrderConstraint{
					LowerParameter: c.Order.LowerParameter,
					UpperParameter: c.Order.UpperParameter,
				},
			})
		case c.Sum != nil:
			sc := &experimentsv1alpha1.SumConstraint{
				IsUpperBound: c.Sum.IsUpperBound,
				Bound:        float64(c.Sum.Bound.MilliValue()) / 1000,
			}
			for _, p := range c.Sum.Parameters {
				// This is a special case to omit parameters client side
				if p.Weight.IsZero() {
					continue
				}

				sc.Parameters = append(sc.Parameters, experimentsv1alpha1.SumConstraintParameter{
					ParameterName: p.Name,
					Weight:        float64(p.Weight.MilliValue()) / 1000,
				})
			}

			out.Constraints = append(out.Constraints, experimentsv1alpha1.Constraint{
				Name:           c.Name,
				ConstraintType: experimentsv1alpha1.ConstraintSum,
				SumConstraint:  sc,
			})
		}
	}

	out.Metrics = nil
	for _, m := range in.Spec.Metrics {
		out.Metrics = append(out.Metrics, experimentsv1alpha1.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
			Optimize: m.Optimize,
		})
	}

	// Check that we have the correct number of assignments on the baseline
	if len(baseline.Assignments) == 0 {
		baseline = nil
	} else if len(baseline.Assignments) != len(out.Parameters) {
		return "", nil, nil, fmt.Errorf("baseline must be specified on all or none of the parameters")
	} else if err := validation.CheckConstraints(out.Constraints, baseline.Assignments); err != nil {
		return "", nil, nil, err
	}

	return v1alpha1.ExperimentName(in.Name), out, baseline, nil
}

// ToCluster converts API state to cluster state
func ToCluster(exp *optimizev1beta2.Experiment, ee *experimentsv1alpha1.Experiment) {
	if exp.GetAnnotations() == nil {
		exp.SetAnnotations(make(map[string]string))
	}

	exp.GetAnnotations()[optimizev1beta2.AnnotationExperimentURL] = ee.Link(api.RelationSelf)
	exp.GetAnnotations()[optimizev1beta2.AnnotationNextTrialURL] = ee.Link(api.RelationNextTrial)

	exp.Spec.Optimization = nil
	for i := range ee.Optimization {
		exp.Spec.Optimization = append(exp.Spec.Optimization, optimizev1beta2.Optimization{
			Name:  ee.Optimization[i].Name,
			Value: ee.Optimization[i].Value,
		})
	}

	controllerutil.AddFinalizer(exp, Finalizer)
}

// ToClusterTrial converts API state to cluster state
func ToClusterTrial(t *optimizev1beta2.Trial, suggestion *experimentsv1alpha1.TrialAssignments) {
	t.GetAnnotations()[optimizev1beta2.AnnotationReportTrialURL] = suggestion.Location()

	// Try to make the cluster trial names match what is on the server
	if t.Name == "" && t.GenerateName != "" && suggestion.Location() != "" {
		name := path.Base(suggestion.Location())
		if num, err := strconv.ParseInt(name, 10, 64); err == nil {
			t.Name = fmt.Sprintf("%s%03d", t.GenerateName, num)
		} else {
			t.Name = t.GenerateName + name
		}
	}

	for _, a := range suggestion.Assignments {
		var v intstr.IntOrString
		if a.Value.IsString {
			v = intstr.FromString(a.Value.StrVal)
		} else {
			// While the server supports 64-bit integers, any parameters used for Kubernetes
			// experiments will have been defined with 32-bit integer bounds.
			val := a.Value.Int64Value()
			switch {
			case val > math.MaxInt32:
				v = intstr.FromInt(math.MaxInt32)
			case val < math.MinInt32:
				v = intstr.FromInt(math.MinInt32)
			default:
				v = intstr.FromInt(int(val))
			}
		}

		t.Spec.Assignments = append(t.Spec.Assignments, optimizev1beta2.Assignment{
			Name:  a.ParameterName,
			Value: v,
		})
	}

	if len(suggestion.Labels) > 0 {
		if t.Labels == nil {
			t.Labels = make(map[string]string, len(suggestion.Labels))
		}
		for k, v := range suggestion.Labels {
			if v != "" {
				t.Labels[k] = v
			} else {
				delete(t.Labels, k)
			}
		}
	}

	trial.UpdateStatus(t)

	controllerutil.AddFinalizer(t, Finalizer)
}

// FromClusterTrial converts cluster state to API state
func FromClusterTrial(t *optimizev1beta2.Trial) *experimentsv1alpha1.TrialValues {
	out := &experimentsv1alpha1.TrialValues{}

	// Set the trial timestamps
	if t.Status.StartTime != nil {
		out.StartTime = &t.Status.StartTime.Time
	}
	if t.Status.CompletionTime != nil {
		out.CompletionTime = &t.Status.CompletionTime.Time
	}

	// Check to see if the trial failed
	for _, c := range t.Status.Conditions {
		if c.Type == optimizev1beta2.TrialFailed && c.Status == corev1.ConditionTrue {
			out.Failed = true
			out.FailureReason = c.Reason
			out.FailureMessage = c.Message
		}
	}

	// Record the values only if we didn't fail
	out.Values = nil
	if !out.Failed {
		for _, v := range t.Spec.Values {
			if fv, err := strconv.ParseFloat(v.Value, 64); err == nil {
				value := experimentsv1alpha1.Value{
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

// IsServerSyncEnabled checks to see if server synchronization is enabled.
func IsServerSyncEnabled(exp *optimizev1beta2.Experiment) bool {
	switch strings.ToLower(exp.GetAnnotations()[optimizev1beta2.AnnotationServerSync]) {
	case "disabled", "false":
		return false
	default:
		return true
	}
}

// DeleteServerExperiment checks to see if the supplied experiment should be
// deleted from the server upon completion. Normally, we do not actually want to
// delete the experiment from the server in order to preserve the data, for
// example, in a multi-cluster experiment we would require that the experiment
// still exist for all the other clusters. We also want to ensure results are
// visible in the UI after the experiment ends in the cluster (deleting the
// server experiments means it won't be available to the UI anymore. We also
// would not want a `reset` (which deletes the CRD) to wipe out all the data on
// the server.
func DeleteServerExperiment(exp *optimizev1beta2.Experiment) bool {
	// As a special case, check to see if synchronization is disabled. This would
	// be the case if someone tried disabling server synchronization mid-run,
	// presumably with the intent of not having the server experiment at the end.
	if !IsServerSyncEnabled(exp) {
		return true
	}

	switch strings.ToLower(exp.GetAnnotations()[optimizev1beta2.AnnotationServerSync]) {
	case "delete-completed", "delete":
		// Allow the server representation of the experiment to be deleted, for
		// example to facilitate debugging or with initial experiment setup.
		return true
	default:
		// DO NOT delete the server experiment
		return false
	}
}

func stringSliceContains(a []string, x string) bool {
	for _, s := range a {
		if s == x {
			return true
		}
	}
	return false
}
