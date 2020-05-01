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
	"fmt"
	"math"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func capturePrometheusMetric(m *redskyv1alpha1.Metric, target runtime.Object, completionTime time.Time) (value float64, stddev float64, err error) {
	urls, err := toURL(target, m)
	if err == nil {
		for _, u := range urls {
			if value, stddev, cerr := captureOnePrometheusMetric(u, m.Query, m.ErrorQuery, completionTime); cerr != nil {
				err = cerr
			} else {
				return value, stddev, nil
			}
		}
	}
	return 0, 0, err
}

func captureOnePrometheusMetric(address, query, errorQuery string, completionTime time.Time) (float64, float64, error) {
	// Get the Prometheus client based on the metric URL
	// TODO Cache these by URL
	c, err := prom.NewClient(prom.Config{Address: address})
	if err != nil {
		return 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	ts, err := promAPI.Targets(context.TODO())
	if err != nil {
		return 0, 0, err
	}
	for _, t := range ts.Active {
		if t.Health == promv1.HealthGood {
			if t.LastScrape.Before(completionTime) {
				// TODO Can we make a more informed delay?
				return 0, 0, &CaptureError{RetryAfter: 5 * time.Second}
			}
		}
	}

	// Execute query
	v, _, err := promAPI.Query(context.TODO(), query, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// Only accept scalar results
	if v.Type() != model.ValScalar {
		return 0, 0, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	// Scalar result
	result := float64(v.(*model.Scalar).Value)
	if math.IsNaN(result) {
		err := &CaptureError{Message: "metric data not available", Address: address, Query: query, CompletionTime: completionTime}
		if strings.HasPrefix(query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	var errorResult float64
	if errorQuery != "" {
		ev, _, err := promAPI.Query(context.TODO(), errorQuery, completionTime)
		if err != nil {
			return 0, 0, err
		}
		if ev.Type() != model.ValScalar {
			return 0, 0, fmt.Errorf("expected scalar error query result, got %s", v.Type())
		}
		errorResult = float64(v.(*model.Scalar).Value)
		if math.IsNaN(errorResult) {
			errorResult = 0
		}
	}

	return result, errorResult, nil
}
