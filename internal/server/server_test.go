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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	redskyapi "github.com/thestormforge/optimize-go/pkg/redskyapi/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/redskyapi/experiments/v1alpha1/numstr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestFromCluster(t *testing.T) {
	one := intstr.FromInt(1)
	two := intstr.FromInt(2)
	three := intstr.FromString("three")
	now := time.Now()
	cases := []struct {
		desc     string
		in       *redskyv1beta1.Experiment
		out      *redskyapi.Experiment
		name     redskyapi.ExperimentName
		baseline *redskyapi.TrialAssignments
	}{
		{
			desc: "basic",
			in: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "basic",
					CreationTimestamp: metav1.Time{
						Time: now,
					},
					Annotations: map[string]string{
						redskyv1beta1.AnnotationExperimentURL: "self_111",
						redskyv1beta1.AnnotationNextTrialURL:  "next_trial_111",
					},
				},
			},
			out: &redskyapi.Experiment{
				ExperimentMeta: redskyapi.ExperimentMeta{
					LastModified: now,
					SelfURL:      "self_111",
					NextTrialURL: "next_trial_111",
				},
			},
		},
		{
			desc: "optimization",
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Optimization: []redskyv1beta1.Optimization{
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
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Parameters: []redskyv1beta1.Parameter{
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
						Bounds: &redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(111, 10)),
							Max: json.Number(strconv.FormatInt(222, 10)),
						},
					},
					{
						Type: redskyapi.ParameterTypeInteger,
						Name: "two",
						Bounds: &redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(1111, 10)),
							Max: json.Number(strconv.FormatInt(2222, 10)),
						},
					},
					{
						Type: redskyapi.ParameterTypeInteger,
						Name: "three",
						Bounds: &redskyapi.Bounds{
							Min: json.Number(strconv.FormatInt(11111, 10)),
							Max: json.Number(strconv.FormatInt(22222, 10)),
						},
					},
				},
			},
		},
		{
			desc: "orderConstraints",
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Constraints: []redskyv1beta1.Constraint{
						{
							Name: "one-two",
							Order: &redskyv1beta1.OrderConstraint{
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
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Constraints: []redskyv1beta1.Constraint{
						{
							Name: "one-two",
							Sum: &redskyv1beta1.SumConstraint{
								Bound: resource.MustParse("1"),
								Parameters: []redskyv1beta1.SumConstraintParameter{
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
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Metrics: []redskyv1beta1.Metric{
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
		{
			desc: "baseline",
			in: &redskyv1beta1.Experiment{
				Spec: redskyv1beta1.ExperimentSpec{
					Parameters: []redskyv1beta1.Parameter{
						{Name: "one", Min: 0, Max: 1, Baseline: &one},
						{Name: "two", Min: 0, Max: 2, Baseline: &two},
						{Name: "three", Values: []string{"three"}, Baseline: &three},
					},
				},
			},
			out: &redskyapi.Experiment{
				Parameters: []redskyapi.Parameter{
					{
						Type:   redskyapi.ParameterTypeInteger,
						Name:   "one",
						Bounds: &redskyapi.Bounds{Min: "0", Max: "1"},
					},
					{
						Type:   redskyapi.ParameterTypeInteger,
						Name:   "two",
						Bounds: &redskyapi.Bounds{Min: "0", Max: "2"},
					},
					{
						Type:   redskyapi.ParameterTypeCategorical,
						Name:   "three",
						Values: []string{"three"},
					},
				},
			},
			baseline: &redskyapi.TrialAssignments{
				Labels: map[string]string{"baseline": "true"},
				Assignments: []redskyapi.Assignment{
					{ParameterName: "one", Value: numstr.FromInt64(1)},
					{ParameterName: "two", Value: numstr.FromInt64(2)},
					{ParameterName: "three", Value: numstr.FromString("three")},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			name, out, baseline, err := FromCluster(c.in)
			if assert.NoError(t, err) {
				assert.Equal(t, c.in.Name, name.Name())
				assert.Equal(t, c.out, out)
				assert.Equal(t, c.baseline, baseline)
			}
		})
	}
}

func TestToCluster(t *testing.T) {
	cases := []struct {
		desc   string
		exp    *redskyv1beta1.Experiment
		ee     *redskyapi.Experiment
		expOut *redskyv1beta1.Experiment
	}{
		{
			desc: "basic",
			exp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			ee: &redskyapi.Experiment{
				ExperimentMeta: redskyapi.ExperimentMeta{
					SelfURL:      "self_111",
					NextTrialURL: "next_trial_111",
				},
				Optimization: []redskyapi.Optimization{
					{Name: "one", Value: "111"},
					{Name: "two", Value: "222"},
					{Name: "three", Value: "333"},
				},
			},
			expOut: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redskyv1beta1.AnnotationExperimentURL: "self_111",
						redskyv1beta1.AnnotationNextTrialURL:  "next_trial_111",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Spec: redskyv1beta1.ExperimentSpec{
					Optimization: []redskyv1beta1.Optimization{
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
		trial      *redskyv1beta1.Trial
		suggestion *redskyapi.TrialAssignments
		trialOut   *redskyv1beta1.Trial
	}{
		{
			desc: "empty name with generate name",
			trial: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "generate_name",
					Annotations:  map[string]string{},
				},
			},
			suggestion: &redskyapi.TrialAssignments{
				TrialMeta: redskyapi.TrialMeta{
					SelfURL: "some/path/1",
				},
				Assignments: []redskyapi.Assignment{
					{ParameterName: "one", Value: numstr.FromInt64(111)},
					{ParameterName: "two", Value: numstr.FromInt64(222)},
					{ParameterName: "three", Value: numstr.FromInt64(333)},
				},
			},
			trialOut: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "generate_name001",
					GenerateName: "generate_name",
					Annotations: map[string]string{
						redskyv1beta1.AnnotationReportTrialURL: "some/path/1",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Status: redskyv1beta1.TrialStatus{
					Phase:       "Created",
					Assignments: "one=111, two=222, three=333",
				},
				Spec: redskyv1beta1.TrialSpec{
					Assignments: []redskyv1beta1.Assignment{
						{Name: "one", Value: intstr.FromInt(111)},
						{Name: "two", Value: intstr.FromInt(222)},
						{Name: "three", Value: intstr.FromInt(333)},
					},
				},
			},
		},
		{
			desc: "name with generate name",
			trial: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "generate_name",
					Annotations:  map[string]string{},
				},
			},
			suggestion: &redskyapi.TrialAssignments{
				TrialMeta: redskyapi.TrialMeta{
					SelfURL: "some/path/one",
				},
				Assignments: []redskyapi.Assignment{
					{ParameterName: "one", Value: numstr.FromInt64(111)},
					{ParameterName: "two", Value: numstr.FromInt64(222)},
					{ParameterName: "three", Value: numstr.FromInt64(333)},
				},
			},
			trialOut: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "generate_nameone",
					GenerateName: "generate_name",
					Annotations: map[string]string{
						redskyv1beta1.AnnotationReportTrialURL: "some/path/one",
					},
					Finalizers: []string{
						Finalizer,
					},
				},
				Status: redskyv1beta1.TrialStatus{
					Phase:       "Created",
					Assignments: "one=111, two=222, three=333",
				},
				Spec: redskyv1beta1.TrialSpec{
					Assignments: []redskyv1beta1.Assignment{
						{Name: "one", Value: intstr.FromInt(111)},
						{Name: "two", Value: intstr.FromInt(222)},
						{Name: "three", Value: intstr.FromInt(333)},
					},
				},
			},
		},
		{
			desc: "32bit overflow",
			trial: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			suggestion: &redskyapi.TrialAssignments{
				Assignments: []redskyapi.Assignment{
					{ParameterName: "overflow", Value: numstr.FromInt64(math.MaxInt64)},
				},
			},
			trialOut: &redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"redskyops.dev/report-trial-url": "",
					},
					Finalizers: []string{"serverFinalizer.redskyops.dev"},
				},
				Spec: redskyv1beta1.TrialSpec{
					Assignments: []redskyv1beta1.Assignment{
						{Name: "overflow", Value: intstr.FromInt(math.MaxInt32)},
					},
				},
				Status: redskyv1beta1.TrialStatus{
					Phase:       "Created",
					Assignments: fmt.Sprintf("overflow=%d", math.MaxInt32),
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
		experiment  redskyv1beta1.Experiment
		trial       redskyv1beta1.Trial
		expectedOut *redskyapi.TrialValues
	}{
		{
			desc: "no conditions",
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					Conditions: []redskyv1beta1.TrialCondition{},
				},
			},
			expectedOut: &redskyapi.TrialValues{},
		},
		{
			desc: "not failed",
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					Conditions: []redskyv1beta1.TrialCondition{
						{Type: redskyv1beta1.TrialComplete, Status: corev1.ConditionTrue},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{},
		},
		{
			desc: "failed",
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					Conditions: []redskyv1beta1.TrialCondition{
						{
							Type:    redskyv1beta1.TrialFailed,
							Status:  corev1.ConditionTrue,
							Reason:  corev1.PodReasonUnschedulable,
							Message: "0/3 nodes are available: 3 Insufficient cpu.",
						},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{
				Failed:         true,
				FailureReason:  corev1.PodReasonUnschedulable,
				FailureMessage: "0/3 nodes are available: 3 Insufficient cpu.",
			},
		},
		{
			desc: "conditions not failed",
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					Conditions: []redskyv1beta1.TrialCondition{
						{Type: redskyv1beta1.TrialComplete, Status: corev1.ConditionTrue},
					},
				},
				Spec: redskyv1beta1.TrialSpec{
					Values: []redskyv1beta1.Value{
						{Name: "one", Value: "111.111", Error: "1111.1111"},
						{Name: "two", Value: "222.222", Error: "2222.2222"},
						{Name: "three", Value: "333.333", Error: "3333.3333"},
					},
				},
			},
			expectedOut: &redskyapi.TrialValues{
				Values: []redskyapi.Value{
					{MetricName: "one", Value: 111.111, Error: 1111.1111},
					{MetricName: "two", Value: 222.222, Error: 2222.2222},
					{MetricName: "three", Value: 333.333, Error: 3333.3333},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			out := FromClusterTrial(&c.trial)
			assert.Equal(t, c.expectedOut, out)
		})
	}
}

func TestStopExperiment(t *testing.T) {
	cases := []struct {
		desc        string
		exp         *redskyv1beta1.Experiment
		err         error
		expectedOut bool
		expectedExp *redskyv1beta1.Experiment
	}{
		{
			desc: "no error",
			exp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err:         nil,
			expectedOut: false,
			expectedExp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error wrong type",
			exp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
			err: &redskyapi.Error{
				Type: redskyapi.ErrExperimentNameInvalid,
			},
			expectedOut: false,
			expectedExp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			desc: "error",
			exp: &redskyv1beta1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						redskyv1beta1.AnnotationNextTrialURL: "111",
					},
				},
			},
			err: &redskyapi.Error{
				Type: redskyapi.ErrExperimentStopped,
			},
			expectedOut: true,
			expectedExp: &redskyv1beta1.Experiment{
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
