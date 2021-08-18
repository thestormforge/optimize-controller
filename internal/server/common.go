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
	"strconv"
	"strings"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func labels(exp *optimizev1beta2.Experiment) map[string]string {
	if len(exp.ObjectMeta.Labels) == 0 {
		return nil
	}

	labels := make(map[string]string, len(exp.ObjectMeta.Labels))

	for k, v := range exp.ObjectMeta.Labels {
		k = strings.TrimPrefix(k, "stormforge.io/")
		labels[k] = v
	}

	return labels
}

func parameters(exp *optimizev1beta2.Experiment) []experimentsv1alpha1.Parameter {
	if len(exp.Spec.Parameters) == 0 {
		return nil
	}

	params := make([]experimentsv1alpha1.Parameter, 0, len(exp.Spec.Parameters))

	for _, p := range exp.Spec.Parameters {
		// This is a special case to omit parameters client side
		if p.Min == p.Max && len(p.Values) == 0 {
			continue
		}

		if len(p.Values) > 0 {
			params = append(params, experimentsv1alpha1.Parameter{
				Type:   experimentsv1alpha1.ParameterTypeCategorical,
				Name:   p.Name,
				Values: p.Values,
			})
		} else {
			params = append(params, experimentsv1alpha1.Parameter{
				Type: experimentsv1alpha1.ParameterTypeInteger,
				Name: p.Name,
				Bounds: &experimentsv1alpha1.Bounds{
					Min: json.Number(strconv.FormatInt(int64(p.Min), 10)),
					Max: json.Number(strconv.FormatInt(int64(p.Max), 10)),
				},
			})
		}
	}

	return params
}

func baselines(exp *optimizev1beta2.Experiment) ([]experimentsv1alpha1.Assignment, error) {
	if len(exp.Spec.Parameters) == 0 {
		return nil, nil
	}

	baselineAssignments := make([]experimentsv1alpha1.Assignment, 0, len(exp.Spec.Parameters))

	for _, p := range exp.Spec.Parameters {
		// This is a special case to omit parameters client side
		if p.Min == p.Max && len(p.Values) == 0 {
			continue
		}

		if p.Baseline != nil {
			var v api.NumberOrString
			if p.Baseline.Type == intstr.String {
				vs := p.Baseline.StrVal
				if !stringSliceContains(p.Values, vs) {
					return nil, fmt.Errorf("baseline out of range for parameter '%s'", p.Name)
				}

				v = api.FromString(vs)
			} else {
				vi := p.Baseline.IntVal
				if vi < p.Min || vi > p.Max {
					return nil, fmt.Errorf("baseline out of range for parameter '%s'", p.Name)
				}

				v = api.FromInt64(int64(vi))
			}

			baselineAssignments = append(baselineAssignments, experimentsv1alpha1.Assignment{
				ParameterName: p.Name,
				Value:         v,
			})
		}
	}

	return baselineAssignments, nil
}

func constraints(exp *optimizev1beta2.Experiment) []experimentsv1alpha1.Constraint {
	if len(exp.Spec.Constraints) == 0 {
		return nil
	}

	constraints := make([]experimentsv1alpha1.Constraint, 0, len(exp.Spec.Constraints))

	for _, c := range exp.Spec.Constraints {
		switch {
		case c.Order != nil:
			constraints = append(constraints, experimentsv1alpha1.Constraint{
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

			constraints = append(constraints, experimentsv1alpha1.Constraint{
				Name:           c.Name,
				ConstraintType: experimentsv1alpha1.ConstraintSum,
				SumConstraint:  sc,
			})
		}
	}

	return constraints
}

func metrics(exp *optimizev1beta2.Experiment) []experimentsv1alpha1.Metric {
	if len(exp.Spec.Metrics) == 0 {
		return nil
	}

	metrics := make([]experimentsv1alpha1.Metric, 0, len(exp.Spec.Metrics))

	for _, m := range exp.Spec.Metrics {
		metrics = append(metrics, experimentsv1alpha1.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
			Optimize: m.Optimize,
		})
	}

	return metrics
}
