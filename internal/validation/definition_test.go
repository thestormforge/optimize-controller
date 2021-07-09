/*
Copyright 2021 GramLabs, Inc.

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
	"testing"

	"github.com/stretchr/testify/assert"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1/numstr"
)

func TestCheckConstraints(t *testing.T) {
	cases := []struct {
		desc        string
		expectErr   bool
		constraints []experimentsv1alpha1.Constraint
		baselines   []experimentsv1alpha1.Assignment
	}{
		{
			desc:      "order-a-less-than-b",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-less-than-b",
					ConstraintType: experimentsv1alpha1.ConstraintOrder,
					OrderConstraint: &experimentsv1alpha1.OrderConstraint{
						LowerParameter: "a",
						UpperParameter: "b",
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(1)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},
		{
			desc:      "order-a-equal-b",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-less-than-b",
					ConstraintType: experimentsv1alpha1.ConstraintOrder,
					OrderConstraint: &experimentsv1alpha1.OrderConstraint{
						LowerParameter: "a",
						UpperParameter: "b",
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(2)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},
		{
			desc:      "order-a-greater-than-b",
			expectErr: true,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-less-than-b",
					ConstraintType: experimentsv1alpha1.ConstraintOrder,
					OrderConstraint: &experimentsv1alpha1.OrderConstraint{
						LowerParameter: "a",
						UpperParameter: "b",
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(3)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},

		{
			desc:      "sum-less-than-upper",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-less-than-5",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound:        5.0,
						IsUpperBound: true,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(2)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},
		{
			desc:      "sum-equal-upper",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-less-than-5",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound:        5.0,
						IsUpperBound: true,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(2)},
				{ParameterName: "b", Value: numstr.FromInt64(3)},
			},
		},
		{
			desc:      "sum-greater-than-upper",
			expectErr: true,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-less-than-5",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound:        5.0,
						IsUpperBound: true,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(3)},
				{ParameterName: "b", Value: numstr.FromInt64(3)},
			},
		},

		{
			desc:      "sum-less-than-lower",
			expectErr: true,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-greater-than-3",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound: 3.0,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(1)},
				{ParameterName: "b", Value: numstr.FromInt64(1)},
			},
		},
		{
			desc:      "sum-equal-lower",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-greater-than-3",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound: 3.0,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(1)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},
		{
			desc:      "sum-greater-than-lower",
			expectErr: false,
			constraints: []experimentsv1alpha1.Constraint{
				{
					Name:           "a-plus-b-greater-than-3",
					ConstraintType: experimentsv1alpha1.ConstraintSum,
					SumConstraint: &experimentsv1alpha1.SumConstraint{
						Bound: 3.0,
						Parameters: []experimentsv1alpha1.SumConstraintParameter{
							{ParameterName: "a", Weight: 1.0},
							{ParameterName: "b", Weight: 1.0},
						},
					},
				},
			},
			baselines: []experimentsv1alpha1.Assignment{
				{ParameterName: "a", Value: numstr.FromInt64(2)},
				{ParameterName: "b", Value: numstr.FromInt64(2)},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := CheckConstraints(c.constraints, c.baselines)
			if c.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
