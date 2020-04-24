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

package server

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFromCluster(t *testing.T) {
	now := time.Now()
	cases := []struct {
		desc string
		in   *redskyv1alpha1.Experiment
		out  *redskyapi.Experiment
		name redskyapi.ExperimentName
	}{
		{
			desc: "basic",
			in: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "basic",
					CreationTimestamp: metav1.Time{
						Time: now,
					},
					Annotations: map[string]string{
						redskyv1alpha1.AnnotationExperimentURL: "self_111",
						redskyv1alpha1.AnnotationNextTrialURL:  "next_trial_111",
					},
				},
			},
			out: &redskyapi.Experiment{
				ExperimentMeta: redskyapi.ExperimentMeta{
					LastModified: now,
					Self:         "self_111",
					NextTrial:    "next_trial_111",
				},
			},
		},
		{
			desc: "optimization",
			in: &redskyv1alpha1.Experiment{
				Spec: redskyv1alpha1.ExperimentSpec{
					Optimization: []redskyv1alpha1.Optimization{
						{Name: "one", Value: "111"},
						{Name: "two", Value: "222"},
						{Name: "three", Value: "333"},
					},
				},
			},
			out: &redskyapi.Experiment{
				Optimization: []redskyapi.Optimization{
					{Name: "one", Value: "111"},
					{Name: "two", Value: "222"},
					{Name: "three", Value: "333"},
				},
			},
		},
		{
			desc: "parameters",
			in: &redskyv1alpha1.Experiment{
				Spec: redskyv1alpha1.ExperimentSpec{
					Parameters: []redskyv1alpha1.Parameter{
						{Name: "one", Min: 111, Max: 222},
						{Name: "two", Min: 1111, Max: 2222},
						{Name: "three", Min: 11111, Max: 22222},
						{Name: "test_case", Min: 1, Max: 1},
					},
				},
			},
			out: &redskyapi.Experiment{
				Parameters: []redskyapi.Parameter{
					{
						Type: redskyapi.ParameterTypeInteger,
						Name: "one",
						Bounds: redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(111, 10)),
							Max: json.Number(strconv.FormatInt(222, 10)),
						},
					},
					{
						Type: redskyapi.ParameterTypeInteger,
						Name: "two",
						Bounds: redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(1111, 10)),
							Max: json.Number(strconv.FormatInt(2222, 10)),
						},
					},
					{
						Type: redskyapi.ParameterTypeInteger,
						Name: "three",
						Bounds: redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(11111, 10)),
							Max: json.Number(strconv.FormatInt(22222, 10)),
						},
					},
				},
			},
		},
		{
			desc: "orderConstraints",
			in: &redskyv1alpha1.Experiment{
				Spec: redskyv1alpha1.ExperimentSpec{
					Constraints: []redskyv1alpha1.Constraint{
						{
							Name: "one-two",
							Order: &redskyv1alpha1.OrderConstraint{
								LowerParameter: "one",
								UpperParameter: "two",
							},
						},
					},
				},
			},
			out: &redskyapi.Experiment{
				Constraints: []redskyapi.Constraint{
					{
						ConstraintType:  redskyapi.ConstraintOrder,
						Name:            "one-two",
						OrderConstraint: redskyapi.OrderConstraint{LowerParameter: "one", UpperParameter: "two"},
					},
				},
			},
		},
		{
			desc: "sumConstraints",
			in: &redskyv1alpha1.Experiment{
				Spec: redskyv1alpha1.ExperimentSpec{
					Constraints: []redskyv1alpha1.Constraint{
						{
							Name: "one-two",
							Sum: &redskyv1alpha1.SumConstraint{
								Bound: resource.MustParse("1"),
								Parameters: []redskyv1alpha1.SumConstraintParameter{
									{
										Name:   "one",
										Weight: resource.MustParse("-1.0"),
									},
									{
										Name:   "two",
										Weight: resource.MustParse("1"),
									},
									{
										Name:   "three",
										Weight: resource.MustParse("3.5"),
									},
									{
										Name:   "four",
										Weight: resource.MustParse("5000m"),
									},
								},
							},
						},
					},
				},
			},
			out: &redskyapi.Experiment{
				Constraints: []redskyapi.Constraint{
					{
						Name:           "one-two",
						ConstraintType: redskyapi.ConstraintSum,
						SumConstraint: redskyapi.SumConstraint{
							Bound: 1,
							Parameters: []redskyapi.SumConstraintParameter{
								{Name: "one", Weight: -1.0},
								{Name: "two", Weight: 1.0},
								{Name: "three", Weight: 3.5},
								{Name: "four", Weight: 5.0},
							},
						},
					},
				},
			},
		},
		{
			desc: "metrics",
			in: &redskyv1alpha1.Experiment{
				Spec: redskyv1alpha1.ExperimentSpec{
					Metrics: []redskyv1alpha1.Metric{
						{Name: "one", Minimize: true},
						{Name: "two", Minimize: false},
						{Name: "three", Minimize: true},
					},
				},
			},
			out: &redskyapi.Experiment{
				Metrics: []redskyapi.Metric{
					{Name: "one", Minimize: true},
					{Name: "two", Minimize: false},
					{Name: "three", Minimize: true},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			name, out := FromCluster(c.in)
			assert.Equal(t, c.in.Name, name.Name())
			assert.Equal(t, c.out, out)
		})
	}
}

func TestToCluster(t *testing.T) {
	cases := []struct {
		desc   string
		exp    *redskyv1alpha1.Experiment
		ee     *redskyapi.Experiment
		expOut *redskyv1alpha1.Experiment
	}{
		{
			desc: "basic",
			exp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			ee: &redskyapi.Experiment{
				ExperimentMeta: redskyapi.ExperimentMeta{
					Self:      "self_111",
					NextTrial: "next_trial_111",
				},
				Optimization: []redskyapi.Optimization{
					{Name: "one", Value: "111"},
					{Name: "two", Value: "222"},
					{Name: "three", Value: "333"},
				},
			},
			expOut: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redskyv1alpha1.AnnotationExperimentURL: "self_111",
						redskyv1alpha1.AnnotationNextTrialURL:  "next_trial_111",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Spec: redskyv1alpha1.ExperimentSpec{
					Optimization: []redskyv1alpha1.Optimization{
						{Name: "one", Value: "111"},
						{Name: "two", Value: "222"},
						{Name: "three", Value: "333"},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			ToCluster(c.exp, c.ee)
			assert.Equal(t, c.expOut, c.exp)
		})
	}
}

func TestToClusterTrial(t *testing.T) {
	cases := []struct {
		desc       string
		trial      *redskyv1alpha1.Trial
		suggestion *redskyapi.TrialAssignments
		trialOut   *redskyv1alpha1.Trial
	}{
		{
			desc: "empty name with generate name",
			trial: &redskyv1alpha1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "generate_name",
					Annotations:  map[string]string{},
				},
			},
			suggestion: &redskyapi.TrialAssignments{
				TrialMeta: redskyapi.TrialMeta{
					ReportTrial: "some/path/1",
				},
				Assignments: []redskyapi.Assignment{
					{ParameterName: "one", Value: json.Number("111")},
					{ParameterName: "two", Value: json.Number("222")},
					{ParameterName: "three", Value: json.Number("333")},
				},
			},
			trialOut: &redskyv1alpha1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "generate_name001",
					GenerateName: "generate_name",
					Annotations: map[string]string{
						redskyv1alpha1.AnnotationReportTrialURL: "some/path/1",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Status: redskyv1alpha1.TrialStatus{
					Phase:       "Created",
					Assignments: "one=111, two=222, three=333",
				},
				Spec: redskyv1alpha1.TrialSpec{
					Assignments: []redskyv1alpha1.Assignment{
						{Name: "one", Value: 111},
						{Name: "two", Value: 222},
						{Name: "three", Value: 333},
					},
				},
			},
		},
		{
			desc: "name with generate name",
			trial: &redskyv1alpha1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "generate_name",
					Annotations:  map[string]string{},
				},
			},
			suggestion: &redskyapi.TrialAssignments{
				TrialMeta: redskyapi.TrialMeta{
					ReportTrial: "some/path/one",
				},
				Assignments: []redskyapi.Assignment{
					{ParameterName: "one", Value: json.Number("111")},
					{ParameterName: "two", Value: json.Number("222")},
					{ParameterName: "three", Value: json.Number("333")},
				},
			},
			trialOut: &redskyv1alpha1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "generate_nameone",
					GenerateName: "generate_name",
					Annotations: map[string]string{
						redskyv1alpha1.AnnotationReportTrialURL: "some/path/one",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Status: redskyv1alpha1.TrialStatus{
					Phase:       "Created",
					Assignments: "one=111, two=222, three=333",
				},
				Spec: redskyv1alpha1.TrialSpec{
					Assignments: []redskyv1alpha1.Assignment{
						{Name: "one", Value: 111},
						{Name: "two", Value: 222},
						{Name: "three", Value: 333},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			ToClusterTrial(c.trial, c.suggestion)
			assert.Equal(t, c.trialOut, c.trial)
		})
	}
}

func TestFromClusterTrial(t *testing.T) {
	cases := []struct {
		desc        string
		in          *redskyv1alpha1.Trial
		expectedOut *redskyapi.TrialValues
	}{
		{
			desc: "no conditions",
			in: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					Conditions: []redskyv1alpha1.TrialCondition{},
				},
			},
			expectedOut: &redskyapi.TrialValues{},
		},
		{
			desc: "not failed",
			in: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					Conditions: []redskyv1alpha1.TrialCondition{
						{Type: redskyv1alpha1.TrialComplete, Status: v1.ConditionTrue},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{},
		},
		{
			desc: "failed",
			in: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					Conditions: []redskyv1alpha1.TrialCondition{
						{Type: redskyv1alpha1.TrialFailed, Status: v1.ConditionTrue},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{
				Failed: true,
			},
		},
		{
			desc: "conditions not failed",
			in: &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					Conditions: []redskyv1alpha1.TrialCondition{
						{Type: redskyv1alpha1.TrialComplete, Status: v1.ConditionTrue},
					},
				},
				Spec: redskyv1alpha1.TrialSpec{
					Values: []redskyv1alpha1.Value{
						{Name: "one", Value: "111.111", Error: "1111.1111"},
						{Name: "two", Value: "222.222", Error: "2222.2222"},
						{Name: "three", Value: "333.333", Error: "3333.3333"},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{
				Values: []v1alpha1.Value{
					{MetricName: "one", Value: 111.111, Error: 1111.1111},
					{MetricName: "two", Value: 222.222, Error: 2222.2222},
					{MetricName: "three", Value: 333.333, Error: 3333.3333},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			out := FromClusterTrial(c.in)
			assert.Equal(t, c.expectedOut, out)
		})
	}
}

func TestStopExperiment(t *testing.T) {
	cases := []struct {
		desc        string
		exp         *redskyv1alpha1.Experiment
		err         error
		expectedOut bool
		expectedExp *redskyv1alpha1.Experiment
	}{
		{
			desc: "no error",
			exp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err:         nil,
			expectedOut: false,
			expectedExp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error wrong type",
			exp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err: &redskyapi.Error{
				Type: redskyapi.ErrExperimentNameInvalid,
			},
			expectedOut: false,
			expectedExp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error",
			exp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redskyv1alpha1.AnnotationNextTrialURL: "111",
					},
				},
			},
			err: &redskyapi.Error{
				Type: redskyapi.ErrExperimentStopped,
			},
			expectedOut: true,
			expectedExp: &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			out := StopExperiment(c.exp, c.err)
			assert.Equal(t, c.expectedOut, out)
			assert.Equal(t, c.expectedExp.GetAnnotations(), c.exp.GetAnnotations())
		})
	}
}
