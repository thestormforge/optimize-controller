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

package validation

import (
	"fmt"
	"math"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
)

// CheckDefinition will make sure the cluster and API experiment definitions are compatible
func CheckDefinition(exp *optimizev1beta2.Experiment, ee *experimentsv1alpha1.Experiment) error {
	if len(exp.Spec.Parameters) == len(ee.Parameters) {
		parameters := make(map[string]bool, len(exp.Spec.Parameters))
		for i := range exp.Spec.Parameters {
			parameters[exp.Spec.Parameters[i].Name] = true
		}
		for i := range ee.Parameters {
			delete(parameters, ee.Parameters[i].Name)
		}
		if len(parameters) > 0 {
			return fmt.Errorf("server and cluster have incompatible parameter definitions")
		}
	} else {
		return fmt.Errorf("server and cluster have incompatible parameter definitions")
	}

	if len(exp.Spec.Metrics) == len(ee.Metrics) {
		metrics := make(map[string]bool, len(exp.Spec.Metrics))
		for i := range exp.Spec.Metrics {
			metrics[exp.Spec.Metrics[i].Name] = true
		}
		for i := range ee.Metrics {
			delete(metrics, ee.Metrics[i].Name)
		}
		if len(metrics) > 0 {
			return fmt.Errorf("server and cluster have incompatible metric definitions")
		}
	} else {
		return fmt.Errorf("server and cluster have incompatible metric definitions")
	}

	return nil
}

// CheckConstraints ensures the supplied baseline assignments are valid for a set
// of constraints.
func CheckConstraints(constraints []experimentsv1alpha1.Constraint, baselines []experimentsv1alpha1.Assignment) error {
	// Nothing to check
	if len(constraints) == 0 || len(baselines) == 0 {
		return nil
	}

	// Index numeric assignments and expose a helper for validating them
	values := make(map[string]float64, len(baselines))
	for _, b := range baselines {
		if b.Value.IsString {
			values[b.ParameterName] = math.NaN()
		} else {
			values[b.ParameterName] = b.Value.Float64Value()
		}
	}
	getValue := func(constraintName, parameterName string) (float64, error) {
		value, ok := values[parameterName]
		switch {
		case !ok:
			return 0, fmt.Errorf("constraint %q references missing parameter %q", constraintName, parameterName)
		case math.IsNaN(value):
			return 0, fmt.Errorf("non-numeric baseline for parameter %q cannot be used to satisfy constraint %q", parameterName, constraintName)
		default:
			return value, nil
		}
	}

	// Make sure all constraints pass
	for _, c := range constraints {
		switch c.ConstraintType {
		case experimentsv1alpha1.ConstraintOrder:
			lower, err := getValue(c.Name, c.OrderConstraint.LowerParameter)
			if err != nil {
				return err
			}

			upper, err := getValue(c.Name, c.OrderConstraint.UpperParameter)
			if err != nil {
				return err
			}

			if lower > upper {
				return fmt.Errorf("baseline does not satisfy constraint %q", c.Name)
			}

		case experimentsv1alpha1.ConstraintSum:
			var sum float64
			for _, p := range c.SumConstraint.Parameters {
				value, err := getValue(c.Name, p.ParameterName)
				if err != nil {
					return err
				}

				sum += value * p.Weight
			}

			if (c.IsUpperBound && sum > c.Bound) || (!c.IsUpperBound && sum < c.Bound) {
				return fmt.Errorf("baseline does not satisfy constraint %q", c.Name)
			}
		}
	}

	return nil
}
