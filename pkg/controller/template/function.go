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

package template

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/util/jsonpath"
)

// FuncMap returns the functions used for template evaluation
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	extra := template.FuncMap{
		"duration":  duration,
		"percent":   percent,
		"sum":       sum,
		"resources": resources,
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

// duration returns a floating point number representing the number of seconds between two times
func duration(start, completion time.Time) float64 {
	if start.Before(completion) {
		return completion.Sub(start).Seconds()
	}
	return 0
}

// percent returns a percentage of an integer value using an integer (0-100) percentage
func percent(value int64, percent int64) string {
	return fmt.Sprintf("%d", int64(float64(value)*(float64(percent)/100.0)))
}

// sum evaluates a JSON path expression against an object and returns the sum of the results coerced to an integral value
func sum(data interface{}, path string) int64 {
	var sum int64
	jp := jsonpath.New("sum")
	if err := jp.Parse(path); err != nil {
		return sum
	}
	values, err := jp.FindResults(data)
	if err != nil {
		return sum
	}
	if len(values) == 1 {
		for i := range values[0] {
			v := reflect.ValueOf(values[0][i].Interface())
			if v.Kind() == reflect.Int64 {
				sum += v.Int()
			} else if v.CanInterface() {
				switch vv := v.Interface().(type) {
				case resource.Quantity:
					sum += vv.MilliValue()
				}
			}
		}
	}
	return sum
}

// total_resources uses a map of resource types to weights to calculate a weighted sum of the resources
func resources(pods corev1.PodList, weights string) float64 {
	var totalResources float64
	parsedWeights := make(map[string]float64)

	for _, singleEntry := range strings.Split(weights, ",") {
		parsedEntry := strings.Split(singleEntry, "=")
		weight, err := strconv.ParseInt(parsedEntry[1], 10, 64)
		if err != nil {
			return 0.0
		}
		parsedWeights[parsedEntry[0]] = float64(weight)
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for resourceType, weight := range parsedWeights {
				resourceValue, found := container.Resources.Requests[corev1.ResourceName(resourceType)]
				if !found {
					return 0.0
				}
				amount := resourceValue.MilliValue()
				totalResources += weight * float64(amount)
			}
		}
	}
	return totalResources
}
