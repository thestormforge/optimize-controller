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
	"time"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
)

// CaptureError describes problems that arise while capturing Prometheus metric values.
type CaptureError struct {
	// A description of what went wrong
	Message string
	// The URL that was used to capture the metric
	Address string
	// The metric query that failed
	Query string
	// The minimum amount of time until the metric is expected to be available
	RetryAfter time.Duration
}

func (e *CaptureError) Error() string {
	return e.Message
}

func capturePrometheusMetric(ctx context.Context, log logr.Logger, m *optimizev1beta2.Metric, completionTime time.Time) (value float64, valueError float64, err error) {
	// Get the Prometheus API
	c, err := prom.NewClient(prom.Config{Address: m.URL})
	if err != nil {
		return 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	lastScrapeEndTime, err := checkReady(ctx, promAPI, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// Execute the query
	value, err = queryScalar(ctx, promAPI, m.Query, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// If we got NaN, it might be that the final scrape hadn't finished
	if math.IsNaN(value) && lastScrapeEndTime.After(completionTime) {
		log.Info("Retrying Prometheus query to include final scrape", "lastScrapeEndTime", lastScrapeEndTime)

		value, err = queryScalar(ctx, promAPI, m.Query, lastScrapeEndTime)
		if err != nil {
			return 0, 0, err
		}
	}

	// Treat NaN as an error condition. This may be due to a bad query that does
	// not account for gaps in the timeline or it might be because Prometheus
	// never pulled in any matching metrics (despite our best efforts in checkReady)
	if math.IsNaN(value) {
		return 0, 0, &CaptureError{Message: "metric data not available", Address: m.URL, Query: m.Query}
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

// Choose lower then normal default scrape parameters
// TODO We could use `api.Config` to get the actual values (global defaults and per-target settings)
const scrapeInterval = 5 * time.Second // Prometheus default is 1m

func checkReady(ctx context.Context, api promv1.API, t time.Time) (time.Time, error) {
	targets, err := api.Targets(ctx)
	if err != nil {
		return t, err
	}

	var lastScrape time.Time

	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			return t, &CaptureError{
				Message:    fmt.Sprintf("scrape target is unhealthy (%s): %s", target.Health, target.LastError),
				Address:    target.ScrapeURL,
				RetryAfter: scrapeInterval,
			}
		}

		// Ensure we have done an additional scrape since completion time
		if target.LastScrape.Before(t.Add(scrapeInterval)) {
			return t, &CaptureError{
				Message:    "waiting for final scrape",
				Address:    target.ScrapeURL,
				RetryAfter: scrapeInterval,
			}
		}

		lastScrape = target.LastScrape
	}

	return lastScrape, nil
}

func queryScalar(ctx context.Context, api promv1.API, q string, t time.Time) (float64, error) {
	v, _, err := api.Query(ctx, q, t)
	if err != nil {
		return 0, err
	}

	switch vt := v.(type) {

	case *model.Scalar:
		return float64(vt.Value), nil

	case *model.Vector:
		// Strictly mimic `scalar(q)` by returning NaN if the vector isn't a single element
		// https://prometheus.io/docs/prometheus/latest/querying/functions/#scalar
		if len(*vt) != 1 {
			return math.NaN(), nil
		}
		return float64((*vt)[0].Value), nil

	default:
		return 0, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}
}
