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

package generation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertPrometheusSelector(t *testing.T) {
	cases := []struct {
		desc           string
		metricSelector string
		expected       *metav1.LabelSelector
	}{
		{
			desc: "empty",
		},
		{
			desc:           "multiple equality",
			metricSelector: `a="A",b="B",c="C"`,
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "A",
					"b": "B",
					"c": "C",
				},
			},
		},
		{
			desc:           "inequality",
			metricSelector: `a!="A"`,
			expected: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "a", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"A"}},
				},
			},
		},
		{
			desc:           "multiple inequality",
			metricSelector: `a!="A",b!="B",c!="C"`,
			expected: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "a", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"A"}},
					{Key: "b", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"B"}},
					{Key: "c", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"C"}},
				},
			},
		},
		{
			desc:           "fake regexp set",
			metricSelector: `a=~"A|B|C"`,
			expected: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "a", Operator: metav1.LabelSelectorOpIn, Values: []string{"A", "B", "C"}},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := convertPrometheusSelector(c.metricSelector)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestConvertPrometheusSelectorError(t *testing.T) {
	cases := []struct {
		desc           string
		metricSelector string
		errStr         string
	}{
		{
			desc:           "no operator",
			metricSelector: `a A`,
			errStr:         `invalid metric selector`,
		},
		{
			desc:           "unknown operator",
			metricSelector: `a=="A"`,
			errStr:         `invalid metric selector`,
		},
		{
			desc:           "unquoted value",
			metricSelector: `a=A`,
			errStr:         `invalid metric selector`,
		},
		{
			desc:           "cannot fake regexp",
			metricSelector: `a=~".+"`,
			errStr:         `invalid metric selector`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err := convertPrometheusSelector(c.metricSelector)
			assert.EqualError(t, err, c.errStr)
		})
	}
}
