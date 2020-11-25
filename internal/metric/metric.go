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
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/template"
	"k8s.io/apimachinery/pkg/runtime"
)

// CaptureMetric captures a point-in-time metric value and it's error rate.
func CaptureMetric(ctx context.Context, log logr.Logger, trial *redskyv1beta1.Trial, metric *redskyv1beta1.Metric, target runtime.Object) (float64, float64, error) {
	// Execute the queries as Go templates
	var err error
	if metric.Query, metric.ErrorQuery, err = template.New().RenderMetricQueries(metric, trial, target); err != nil {
		return 0, 0, err
	}

	// Capture the value based on the metric type
	switch metric.Type {
	case redskyv1beta1.MetricKubernetes, "":
		value, err := strconv.ParseFloat(metric.Query, 64)
		return value, math.NaN(), err
	case redskyv1beta1.MetricPrometheus:
		return capturePrometheusMetric(ctx, log, metric, trial.Status.CompletionTime.Time)
	case redskyv1beta1.MetricDatadog:
		return captureDatadogMetric(metric, trial.Status.StartTime.Time, trial.Status.CompletionTime.Time)
	case redskyv1beta1.MetricJSONPath:
		return captureJSONPathMetric(metric)
	default:
		return 0, 0, fmt.Errorf("unknown metric type: %s", metric.Type)
	}
}
