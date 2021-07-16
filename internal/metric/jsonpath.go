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

package metric

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"time"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"k8s.io/client-go/util/jsonpath"
)

// TODO We need some type of client util to encapsulate this
var httpClient = &http.Client{Timeout: 10 * time.Second}

func captureJSONPathMetric(m *optimizev1beta2.Metric) (value float64, valueError float64, err error) {
	// Fetch the URL
	req, err := http.NewRequest(http.MethodGet, m.URL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req.WithContext(context.TODO()))
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		// TODO Should we not ignore this?
		return 0, math.NaN(), nil
	}

	// Unmarshal as generic JSON
	data := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, err
	}

	// Evaluate the JSON path
	jp := jsonpath.New(m.Name)
	if err := jp.Parse(m.Query); err != nil {
		return 0, 0, err
	}
	values, err := jp.FindResults(data)
	if err != nil {
		return 0, 0, err
	}

	// Convert the result to a float
	if len(values) == 1 && len(values[0]) == 1 {
		v := reflect.ValueOf(values[0][0].Interface())
		switch v.Kind() {
		case reflect.Float64:
			return v.Float(), math.NaN(), nil
		case reflect.String:
			v, err := strconv.ParseFloat(v.String(), 64)
			if err != nil {
				return 0, 0, err
			}
			return v, math.NaN(), nil
		default:
			return 0, 0, fmt.Errorf("could not convert match to a floating point number")
		}
	}

	// If we made it this far we weren't able to extract the value
	return 0, 0, fmt.Errorf("query '%s' did not match", m.Query)
}
