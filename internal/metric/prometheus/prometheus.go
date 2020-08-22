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

package prometheus

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusCollector implements the Source interface and contains all of the
// necessary metadata to query a prometheus endpoint.
type PrometheusCollector struct {
	api  promv1.API
	name string
	// this name sucks but i cant think of a more appropriate one right now
	resultQuery    string
	errorQuery     string
	startTime      time.Time
	completionTime time.Time
}

func NewCollector(url, name, query, errorQuery string, startTime, endTime time.Time) (*PrometheusCollector, error) {
	// Nothing fancy here
	c, err := prom.NewClient(prom.Config{Address: url})
	if err != nil {
		return nil, err
	}

	return &PrometheusCollector{
		api:            promv1.NewAPI(c),
		name:           name,
		resultQuery:    query,
		errorQuery:     errorQuery,
		startTime:      startTime,
		completionTime: endTime,
	}, nil
}

// Tried to leave as much of the original in place to reduce the changes for discussion since it's not super
// relevant right now. I did get carried away on the commit before this, so I added this second commit to keep
// things straight and not lose out on the previous work :D
func (p *PrometheusCollector) Capture(ctx context.Context) (value float64, stddev float64, err error) {
	return p.captureOnePrometheusMetric(ctx)
}

func (p *PrometheusCollector) captureOnePrometheusMetric(ctx context.Context) (float64, float64, error) {
	// Make sure Prometheus is ready
	targets, err := p.api.Targets(ctx)
	if err != nil {
		return 0, 0, err
	}

	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			continue
		}

		if target.LastScrape.Before(p.completionTime) {
			// We do run into an importing issue (cyclical) with trying to have well defined errors being used here.
			// To address the import issues
			//   - We could move this to a `internal/errors` but I'm unsure how I feel about a large errors package;
			//		 feels appropraite to be locally scoped here
			//   - We could drop CaptureError altogether. CaptureError.RetryAfter is used to signal the controller to requeue
			//     which we could instead handle here considering this portion of the CaptureError does not impact the
			//     remaining attempts. This would simplify the error type we return ( just a regular error )
			//     and prevent another reconcile loop.

			// TODO Can we make a more informed delay?
			return 0, 0, &CaptureError{RetryAfter: 5 * time.Second}
		}
	}

	// Execute query
	v, _, err := p.api.Query(ctx, p.resultQuery, p.completionTime)
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
		err := &CaptureError{Message: "metric data not available", Address: "", Query: p.resultQuery, CompletionTime: p.completionTime}
		if strings.HasPrefix(p.resultQuery, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	var errorResult float64
	if p.errorQuery != "" {
		ev, _, err := p.api.Query(ctx, p.errorQuery, p.completionTime)
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
