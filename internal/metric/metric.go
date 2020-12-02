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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/template"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
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
func CaptureMetric(log logr.Logger, metric *redskyv1beta1.Metric, trial *redskyv1beta1.Trial, target runtime.Object) (float64, float64, error) {
	// Work on a copy so we can render the queries in place
	metric = metric.DeepCopy()

	// Execute the query as a template against the current state of the trial
	var err error
	if metric.Query, metric.ErrorQuery, err = template.New().RenderMetricQueries(metric, trial, target); err != nil {
		return 0, 0, err
	}

	// Capture the value based on the metric type
	switch metric.Type {
	case redskyv1beta1.MetricLocal, redskyv1beta1.MetricPods, "":
		// Just parse the query as a float
		value, err := strconv.ParseFloat(metric.Query, 64)
		return value, 0, err
	case redskyv1beta1.MetricPrometheus:
		return capturePrometheusMetric(log, metric, target, trial.Status.CompletionTime.Time)
	case redskyv1beta1.MetricDatadog:
		return captureDatadogMetric(metric.Scheme, metric.Query, trial.Status.StartTime.Time, trial.Status.CompletionTime.Time)
	case redskyv1beta1.MetricJSONPath:
		return captureJSONPathMetric(metric, target)
	default:
		return 0, 0, fmt.Errorf("unknown metric type: %s", metric.Type)
	}
}

func toURL(target runtime.Object, m *redskyv1beta1.Metric) ([]string, error) {
	// Allow a specified URL to take precedence over a selector
	if m.URL != "" {
		return []string{m.URL}, nil
	}

	// Make sure we got a service list
	// TODO We can probably handle a pod list by addressing it directly
	list, ok := target.(*corev1.ServiceList)
	if !ok {
		return nil, fmt.Errorf("expected target to be a service list")
	}

	// Get URL components
	scheme := strings.ToLower(m.Scheme)
	if scheme == "" {
		scheme = "http"
	} else if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("scheme must be 'http' or 'https': %s", scheme)
	}
	path := "/" + strings.TrimLeft(m.Path, "/")

	// Construct a URL for each service (
	var urls []string
	for _, s := range list.Items {

		// Prefer IP literals instead of host names to avoid DNS lookups
		host := s.Spec.ClusterIP
		if host == "None" {
			// fall back to service name if it is a headless service
			host = fmt.Sprintf("%s.%s", s.Name, s.Namespace)
		}

		var ports []int

		switch m.Port.Type {
		case intstr.Int:
			if m.Port.IntValue() > 0 {
				// Explicitly add port number
				ports = append(ports, m.Port.IntValue())
			} else {
				// Aggregate all ports on the service
				for _, sp := range s.Spec.Ports {
					ports = append(ports, int(sp.Port))
				}
			}
		case intstr.String:
			// Attempt to match port name
			for _, sp := range s.Spec.Ports {
				if sp.Name == m.Port.StrVal || len(s.Spec.Ports) == 1 {
					ports = append(ports, int(sp.Port))
					break
				}
			}
		}

		if len(ports) == 0 {
			return nil, fmt.Errorf("metric '%s' has unresolvable port: %s", m.Name, m.Port.String())
		}

		for _, port := range ports {
			urls = append(urls, fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path))
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("unable to find metric targets for '%s'", m.Name)
	}

	return urls, nil
}
