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
	"testing"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func mustQuantity(str string) *resource.Quantity {
	q := resource.MustParse(str)
	return &q
}

func TestCheckMetricBounds(t *testing.T) {
	cases := []struct {
		desc     string
		metric   redskyv1beta1.Metric
		value    redskyv1beta1.Value
		hasError bool
	}{
		{
			desc: "empty",
		},
		{
			desc:  "no bounds",
			value: redskyv1beta1.Value{Value: "1.0"},
		},

		{
			desc:     "value too low",
			metric:   redskyv1beta1.Metric{Min: mustQuantity("2.0")},
			value:    redskyv1beta1.Value{Value: "1.0"},
			hasError: true,
		},
		{
			desc:     "value too high",
			metric:   redskyv1beta1.Metric{Max: mustQuantity("1.0")},
			value:    redskyv1beta1.Value{Value: "2.0"},
			hasError: true,
		},

		{
			desc:   "value above less precise min",
			metric: redskyv1beta1.Metric{Min: mustQuantity("1.2345")},
			value:  redskyv1beta1.Value{Value: "1.23456789"},
		},
		{
			desc:     "value above less precise max ",
			metric:   redskyv1beta1.Metric{Max: mustQuantity("1.2345")},
			value:    redskyv1beta1.Value{Value: "1.23456789"},
			hasError: true,
		},

		{
			desc:     "suffix max",
			metric:   redskyv1beta1.Metric{Max: mustQuantity("100m")},
			value:    redskyv1beta1.Value{Value: "0.2"},
			hasError: true,
		},
		{
			desc:   "suffix min",
			metric: redskyv1beta1.Metric{Min: mustQuantity("100m")},
			value:  redskyv1beta1.Value{Value: "0.2"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := CheckMetricBounds(&c.metric, &c.value)
			if c.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
