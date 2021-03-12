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
)

// CaptureError describes problems that arise while capturing Prometheus metric values.
type CaptureError struct {
	// A description of what went wrong
	Message string
	// The URL that was used to capture the metric
	Address string
	// The metric query that failed
	Query string
	// The completion time at which the query was executed
	CompletionTime time.Time
	// The minimum amount of time until the metric is expected to be available
	RetryAfter time.Duration
}

func (e *CaptureError) Error() string {
	return e.Message
}

func capturePrometheusMetric(ctx context.Context, log logr.Logger, m *redskyv1beta1.Metric, completionTime time.Time) (value float64, valueError float64, err error) {
	// Get the Prometheus API
	c, err := prom.NewClient(prom.Config{Address: m.URL})
	if err != nil {
		return 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	lastScrapeTime, err := checkReady(ctx, promAPI, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// Execute query
	value, err = queryScalar(ctx, promAPI, m.Query, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// If we got NaN, it might be a Pushgateway metric that won't have a value unless we query from the scrape time
	if math.IsNaN(value) && lastScrapeTime.After(completionTime) {
		log.Info("Retrying Prometheus query to include final scrape", "lastScrapeTime", lastScrapeTime, "completionTime", completionTime)
		value, err = queryScalar(ctx, promAPI, m.Query, lastScrapeTime)
		if err != nil {
			return 0, 0, err
		}
	}

	// If it is still NaN, the problem is mostly likely that the query does not account for missing data
	if math.IsNaN(value) {
		err := &CaptureError{Message: "metric data not available", Address: m.URL, Query: m.Query, CompletionTime: completionTime}
		if strings.HasPrefix(m.Query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	if m.ErrorQuery != "" {
		valueError, err = queryScalar(ctx, promAPI, m.ErrorQuery, completionTime)
		if err != nil {
			return 0, 0, err
		}
	}

	return value, valueError, nil
}

func checkReady(ctx context.Context, api promv1.API, t time.Time) (time.Time, error) {
	// Choose lower then normal default scrape parameters
	// TODO We could use `api.Config` to get the actual values (global defaults and per-target settings)
	scrapeInterval := 5 * time.Second // Prometheus default is 1m
	scrapeTimeout := 3 * time.Second  // Prometheus default is 10s

	targets, err := api.Targets(ctx)
	if err != nil {
		return t, err
	}

	lastScrape := t
	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			return t, &CaptureError{
				Message:    fmt.Sprintf("scrape target is unhealthy (%s): %s", target.Health, target.LastError),
				Address:    target.ScrapeURL,
				RetryAfter: scrapeInterval,
			}
		}

		if target.LastScrape.Before(t) {
			return t, &CaptureError{
				Message:    "waiting for final scrape",
				Address:    target.ScrapeURL,
				RetryAfter: scrapeInterval,
			}
		}

		// TODO This should use `target.LastScrapeDuration` once it is available
		if ls := target.LastScrape.Add(scrapeTimeout); ls.After(lastScrape) {
			lastScrape = ls
		}
	}

	return lastScrape, nil
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
