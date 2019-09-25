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
package trial

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/template"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/jsonpath"
)

// TODO We need some type of client util to encapsulate this
// TODO Combine it with the Prometheus clients?
var httpClient = &http.Client{Timeout: 10 * time.Second}

type MetricError struct {
	// A description of what went wrong
	Message string
	// The URL used to capture the metric
	Address string
	// The metric query that failed
	Query string
	// The completion time at which the query was executed
	CompletionTime time.Time
	// The minimum amount of time until the metric is expected to be available
	RetryAfter time.Duration
}

func (e *MetricError) Error() string {
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

func capturePrometheusMetric(m *redskyv1alpha1.Metric, target runtime.Object, completionTime time.Time) (value float64, stddev float64, err error) {
	// Iterate over the target URLs, taking the first successful attempt
	if urls, err := toURL(target, m); err == nil {
		for _, u := range urls {
			if value, stddev, err = captureOnePrometheusMetric(u, m.Query, m.ErrorQuery, completionTime); err == nil {
				return
			}
		}
	}
	return
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
				return 0, 0, &MetricError{RetryAfter: 5 * time.Second}
			}
		}
	}

	// Execute query
	v, err := promAPI.Query(context.TODO(), query, completionTime)
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
		err := &MetricError{Message: "metric data not available", Address: address, Query: query, CompletionTime: completionTime}
		if strings.HasPrefix(query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	var errorResult float64
	if errorQuery != "" {
		ev, err := promAPI.Query(context.TODO(), errorQuery, completionTime)
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

func captureJSONPathMetric(m *redskyv1alpha1.Metric, target runtime.Object) (value float64, stddev float64, err error) {
	// Iterate over the target URLs, taking the first successful attempt
	if urls, err := toURL(target, m); err == nil {
		for _, u := range urls {
			if value, stddev, err = captureOneJSONPathMetric(u, m.Name, m.Query); err == nil {
				return
			}
		}
	}
	return
}

func captureOneJSONPathMetric(url, name, query string) (float64, float64, error) {
	// Fetch the URL
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		return 0, 0, nil
	}

	// Unmarshal as generic JSON
	data := make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		return 0, 0, err
	}

	// Evaluate the JSON path
	jp := jsonpath.New(name)
	if err := jp.Parse(query); err != nil {
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
			return v.Float(), 0, nil
		case reflect.String:
			if v, err := strconv.ParseFloat(v.String(), 64); err != nil {
				return 0, 0, err
			} else {
				return v, 0, nil
			}
		default:
			return 0, 0, fmt.Errorf("could not convert match to a floating point number")
		}
	}

	// If we made it this far we weren't able to extract the value
	return 0, 0, fmt.Errorf("query '%s' did not match", query)
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
