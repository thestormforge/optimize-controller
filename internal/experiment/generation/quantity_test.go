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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestAsScaledInt(t *testing.T) {
	cases := []struct {
		q        resource.Quantity
		scale    resource.Scale
		expected int32
	}{
		// AsScaledInt vs ScaledValue
		{
			q:        *resource.NewQuantity(1, resource.BinarySI),
			scale:    resource.Milli,
			expected: 1024,
		},
		{
			q:        *resource.NewQuantity(1, resource.DecimalSI),
			scale:    resource.Milli,
			expected: 1000,
		},

		// Binary Mega
		{
			q:        resource.MustParse("2Gi"),
			scale:    resource.Mega,
			expected: 2048,
		},
		{
			q:        resource.MustParse("2Mi"),
			scale:    resource.Mega,
			expected: 2,
		},
		{
			q:        resource.MustParse("2048Ki"),
			scale:    resource.Mega,
			expected: 2,
		},

		// Decimal Mega
		{
			q:        resource.MustParse("2G"),
			scale:    resource.Mega,
			expected: 2000,
		},
		{
			q:        resource.MustParse("2M"),
			scale:    resource.Mega,
			expected: 2,
		},
		{
			q:        resource.MustParse("2000k"),
			scale:    resource.Mega,
			expected: 2,
		},

		// Decimal Milli
		{
			q:        resource.MustParse("2.0"),
			scale:    resource.Milli,
			expected: 2000,
		},
		{
			q:        resource.MustParse("2000m"),
			scale:    resource.Milli,
			expected: 2000,
		},
		{
			q:        resource.MustParse("2000000u"),
			scale:    resource.Milli,
			expected: 2000,
		},
	}
	for _, c := range cases {
		t.Run(desc(c.q.String(), c.scale), func(t *testing.T) {
			assert.Equal(t, c.expected, AsScaledInt(c.q, c.scale))
		})
	}
}

func TestQuantitySuffix(t *testing.T) {
	cases := []struct {
		scale    resource.Scale
		format   resource.Format
		expected string
	}{
		{expected: "Mi", scale: resource.Mega, format: resource.BinarySI},
		{expected: "", scale: resource.Nano, format: resource.BinarySI},
		{expected: "", scale: resource.Scale(-96), format: resource.BinarySI},
		{expected: "n", scale: resource.Nano, format: resource.DecimalSI},
		{expected: "E", scale: resource.Exa, format: resource.DecimalSI},
		{expected: "", scale: resource.Exa + 1, format: resource.DecimalSI},
		{expected: "", scale: resource.Exa + 3, format: resource.DecimalSI},
	}
	for _, c := range cases {
		t.Run(desc(string(c.format), c.scale), func(t *testing.T) {
			assert.Equal(t, c.expected, QuantitySuffix(c.scale, c.format))
		})
	}
}

func desc(str string, scale resource.Scale) string {
	scaleString := fmt.Sprintf("Pow10(%d)", scale)
	i := scale/3 + 3
	if scale%3 == 0 && i >= 0 && i < 9 {
		scaleString = []string{"Nano", "Micro", "Milli", "", "Kilo", "Mega", "Giga", "Tera", "Peta", "Exa"}[i]
	}

	return fmt.Sprintf("%s_%s", str, scaleString)
}
