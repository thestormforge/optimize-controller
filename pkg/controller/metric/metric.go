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

package metric

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/template"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CaptureError describes problems that arise while capturing metric values
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

// CaptureMetric captures a point-in-time metric value and it's error (standard deviation)
func CaptureMetric(metric *redskyv1alpha1.Metric, trial *redskyv1alpha1.Trial, target runtime.Object) (float64, float64, error) {
	// Work on a copy so we can render the queries in place
	metric = metric.DeepCopy()

	// Execute the query as a template against the current state of the trial
	var err error
	if metric.Query, metric.ErrorQuery, err = template.NewTemplateEngine().RenderMetricQueries(metric, trial); err != nil {
		return 0, 0, err
	}

	// Capture the value based on the metric type
	switch metric.Type {
	case redskyv1alpha1.MetricLocal, "":
		return captureLocalMetric(metric)
	case redskyv1alpha1.MetricPrometheus:
		return capturePrometheusMetric(metric, target, trial.Status.CompletionTime.Time)
	case redskyv1alpha1.MetricJSONPath:
		return captureJSONPathMetric(metric, target)
	default:
		return 0, 0, fmt.Errorf("unknown metric type: %s", metric.Type)
	}
}

func captureLocalMetric(m *redskyv1alpha1.Metric) (float64, float64, error) {
	// Just parse the query as a float
	value, err := strconv.ParseFloat(m.Query, 64)
	return value, 0, err
}

func toURL(target runtime.Object, m *redskyv1alpha1.Metric) ([]string, error) {
	// Make sure we got a service list
	// TODO We can probably handle a pod list by addressing it directly
	list, ok := target.(*corev1.ServiceList)
	if !ok {
		return nil, fmt.Errorf("expected service list")
	}

	// Get URL components
	scheme := strings.ToLower(m.Scheme)
	if scheme == "" {
		scheme = "http"
	} else if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("scheme must be 'http' or 'https': %s", scheme)
	}
	path := "/" + strings.TrimLeft(m.Path, "/")

	// Construct a URL for each service (use IP literals instead of host names to avoid DNS lookups)
	var urls []string
	for _, s := range list.Items {
		host := s.Spec.ClusterIP
		port := m.Port.IntValue()

		if port < 1 {
			portName := m.Port.StrVal
			// TODO Default an empty portName to scheme?
			for _, sp := range s.Spec.Ports {
				if sp.Name == portName || len(s.Spec.Ports) == 1 {
					port = int(sp.Port)
				}
			}
		}

		if port < 1 {
			return nil, fmt.Errorf("metric '%s' has unresolvable port: %s", m.Name, m.Port.String())
		}

		urls = append(urls, fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path))
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("unable to find metric targets for '%s'", m.Name)
	}
	return urls, nil
}
