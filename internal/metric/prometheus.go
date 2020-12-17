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

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func capturePrometheusMetric(log logr.Logger, m *redskyv1beta1.Metric, target runtime.Object, completionTime time.Time) (value float64, stddev float64, err error) {
	var urls []string

	if urls, err = toURL(target, m); err != nil {
		return value, stddev, err
	}

	for _, u := range urls {
		if value, stddev, err = captureOnePrometheusMetric(log, u, m.Query, m.ErrorQuery, completionTime); err == nil {
			break
		}

		if _, ok := err.(*CaptureError); ok {
			return value, stddev, err
		}
	}

	return value, stddev, err
}

func captureOnePrometheusMetric(log logr.Logger, address, query, errorQuery string, completionTime time.Time) (float64, float64, error) {
	ctx := context.TODO()
	var value, valueError float64

	// Get the Prometheus client based on the metric URL
	// TODO Cache these by URL
	c, err := prom.NewClient(prom.Config{Address: address})
	if err != nil {
		return 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	queryTime, err := checkReady(ctx, promAPI, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// If we needed to adjust the query time, log it so we have a record of the actual time being used
	if !queryTime.Equal(completionTime) {
		log.WithValues("queryTime", queryTime).Info("Adjusted completion time for Prometheus query")
	}

	// Execute query
	value, err = queryScalar(ctx, promAPI, query, queryTime)
	if err != nil {
		return 0, 0, err
	}
	if math.IsNaN(value) {
		err := &CaptureError{Message: "metric data not available", Address: address, Query: query, CompletionTime: completionTime}
		if strings.HasPrefix(query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	if errorQuery != "" {
		valueError, err = queryScalar(ctx, promAPI, errorQuery, queryTime)
		if err != nil {
			return 0, 0, err
		}
	}

	// Ignore NaN for the value error
	if math.IsNaN(valueError) {
		valueError = 0
	}

	return value, valueError, nil
}

func checkReady(ctx context.Context, api promv1.API, t time.Time) (time.Time, error) {
	targets, err := api.Targets(ctx)
	if err != nil {
		return t, err
	}

	queryTime := t
	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			continue
		}

		if target.LastScrape.Before(t) {
			// TODO Can we make a more informed delay?
			return t, &CaptureError{RetryAfter: 5 * time.Second}
		}

		if target.LastScrape.After(queryTime) {
			queryTime = target.LastScrape
		}
	}

	return queryTime, nil
}

func queryScalar(ctx context.Context, api promv1.API, q string, t time.Time) (float64, error) {
	v, _, err := api.Query(ctx, q, t)
	if err != nil {
		return 0, err
	}

	if v.Type() != model.ValScalar {
		return 0, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	return float64(v.(*model.Scalar).Value), nil
}
