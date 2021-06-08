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
	"strconv"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"k8s.io/apimachinery/pkg/api/resource"
)

// CheckMetricBounds ensures the specified
func CheckMetricBounds(m *optimizev1beta2.Metric, v *optimizev1beta2.Value) error {
	// If the value isn't a valid number, ignore the bounds check
	value, err := strconv.ParseFloat(v.Value, 64)
	if err != nil {
		return nil
	}

	if m.Min != nil {
		min := float64(m.Min.ScaledValue(resource.Nano)) / 1000000000
		if value < min {
			return fmt.Errorf("metric value %f for %s is below the minimum of %s", value, m.Name, m.Min.String())
		}
	}

	if m.Max != nil {
		max := float64(m.Max.ScaledValue(resource.Nano)) / 1000000000
		if value > max {
			return fmt.Errorf("metric value %f for %s is above the maximum of %s", value, m.Name, m.Max.String())
		}
	}

	return nil
}
