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

package controller

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIgnoreNotFound(t *testing.T) {
	cases := []struct {
		desc        string
		in          error
		expectedErr error
	}{
		{
			desc:        "nil",
			in:          nil,
			expectedErr: nil,
		},
		{
			desc: "not found error",
			in: &errors.StatusError{
				ErrStatus: metav1.Status{
					Reason: metav1.StatusReasonNotFound,
				},
			},
			expectedErr: nil,
		},
		{
			desc: "api error experiment not found",
			in: &api.Error{
				Type: experimentsv1alpha1.ErrExperimentNotFound,
			},
			expectedErr: nil,
		},
		{
			desc: "api error trial not found",
			in: &api.Error{
				Type: experimentsv1alpha1.ErrTrialNotFound,
			},
			expectedErr: nil,
		},
		{
			desc:        "other error",
			in:          fmt.Errorf("111"),
			expectedErr: fmt.Errorf("111"),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := IgnoreNotFound(c.in)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}

func TestIgnoreAlreadyExists(t *testing.T) {
	cases := []struct {
		desc        string
		in          error
		expectedErr error
	}{
		{
			desc:        "nil",
			in:          nil,
			expectedErr: nil,
		},
		{
			desc: "already exists",
			in: &errors.StatusError{
				ErrStatus: metav1.Status{
					Reason: metav1.StatusReasonAlreadyExists,
				},
			},
			expectedErr: nil,
		},
		{
			desc:        "other error",
			in:          fmt.Errorf("111"),
			expectedErr: fmt.Errorf("111"),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := IgnoreAlreadyExists(c.in)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}

func TestIgnoreReportError(t *testing.T) {
	cases := []struct {
		desc        string
		in          error
		expectedErr error
	}{
		{
			desc:        "nil",
			in:          nil,
			expectedErr: nil,
		},
		{
			desc: "is not found error",
			in: &errors.StatusError{
				ErrStatus: metav1.Status{
					Reason: metav1.StatusReasonNotFound,
				},
			},
			expectedErr: nil,
		},
		{
			desc: "trial already reported",
			in: &api.Error{
				Type: experimentsv1alpha1.ErrTrialAlreadyReported,
			},
			expectedErr: nil,
		},
		{
			desc:        "other error",
			in:          fmt.Errorf("111"),
			expectedErr: fmt.Errorf("111"),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := IgnoreReportError(c.in)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}
