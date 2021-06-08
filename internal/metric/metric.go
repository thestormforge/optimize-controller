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
	"strconv"

	"github.com/go-logr/logr"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/template"
	"k8s.io/apimachinery/pkg/runtime"
)

// CaptureMetric captures a point-in-time metric value and it's error rate.
func CaptureMetric(ctx context.Context, log logr.Logger, trial *optimizev1beta2.Trial, metric *optimizev1beta2.Metric, target runtime.Object) (float64, float64, error) {
	// Execute the queries as Go templates
	var err error
	if metric.Query, metric.ErrorQuery, err = template.New().RenderMetricQueries(metric, trial, target); err != nil {
		return 0, 0, err
	}

	// Capture the value based on the metric type
	switch metric.Type {
	case optimizev1beta2.MetricKubernetes, "":
		value, err := strconv.ParseFloat(metric.Query, 64)
		return value, math.NaN(), err
	case optimizev1beta2.MetricPrometheus:
		return capturePrometheusMetric(ctx, log, metric, trial.Status.CompletionTime.Time)
	case optimizev1beta2.MetricDatadog:
		return captureDatadogMetric(metric, trial.Status.StartTime.Time, trial.Status.CompletionTime.Time)
	case optimizev1beta2.MetricJSONPath:
		return captureJSONPathMetric(metric)
	case optimizev1beta2.MetricNewRelic:
		return captureNewRelicMetric(metric, trial.Status.StartTime.Time, trial.Status.CompletionTime.Time)
	default:
		return 0, 0, fmt.Errorf("unknown metric type: %s", metric.Type)
	}
}
