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
	"net/http"
	"strconv"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"k8s.io/client-go/util/jsonpath"
)

// TODO We need some type of client util to encapsulate this
// TODO Combine it with the Prometheus clients?
var httpClient = &http.Client{Timeout: 10 * time.Second}

func captureMetric(m *redskyv1alpha1.Metric, u string, trial *redskyv1alpha1.Trial) (float64, float64, time.Duration, error) {
	// Execute the query as a template against the current state of the trial
	q, err := executeMetricQueryTemplate(m, trial)
	if err != nil {
		return 0, 0, 0, err
	}

	// Capture the value based on the metric type
	switch m.Type {
	case redskyv1alpha1.MetricLocal, "":
		return captureLocalMetric(q)
	case redskyv1alpha1.MetricPrometheus:
		return capturePrometheusMetric(u, q, trial.Status.CompletionTime.Time)
	case redskyv1alpha1.MetricJSONPath:
		return captureJSONPathMetric(u, m.Name, q)
	default:
		return 0, 0, 0, fmt.Errorf("unknown metric type: %s", m.Type)
	}
}

func captureLocalMetric(query string) (float64, float64, time.Duration, error) {
	// Just parse the query as a float
	value, err := strconv.ParseFloat(query, 64)
	return value, 0, 0, err
}

func capturePrometheusMetric(address, query string, completionTime time.Time) (float64, float64, time.Duration, error) {
	// Get the Prometheus client based on the metric URL
	// TODO Cache these by URL
	c, err := prom.NewClient(prom.Config{Address: address})
	if err != nil {
		return 0, 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	log.Info("1: Prometheus API created", "address", address)

	// Make sure Prometheus is ready
	ts, err := promAPI.Targets(context.TODO())

	if err != nil {
		log.Error(err, "2: Prometheus targets failed")

		return 0, 0, 0, err
	}
	log.Info("3: Prometheus targets acquired", "activeLen", len(ts.Active))

	for _, t := range ts.Active {
		if t.Health == promv1.HealthGood {
			if t.LastScrape.Before(completionTime) {
				// TODO This is for debugging, it should probably be removed
				log.Info("4: Prometheus target is not ready", "scrapeUrl", t.ScrapeURL, "lastScrape", t.LastScrape, "completionTime", completionTime)
				// TODO Can we make a more informed delay?
				return 0, 0, 5 * time.Second, nil
			} else {
				log.Info("5: Prometheus target is ready", "scrapeUrl", t.ScrapeURL)
			}
		} else {
			log.Info("6: Skipping last scrape check for unhealthy Prometheus target", "lastError", t.LastError)
		}
	}

	log.Info("7: Prometheus targets ready")

	// Execute query
	v, err := promAPI.Query(context.TODO(), query, completionTime)
	if err != nil {
		log.Error(err, "9: Prometheus query error")
		return 0, 0, 0, err
	}

	log.Info("9: Prometheus query executed")

	// Only accept scalar results
	if v.Type() != model.ValScalar {
		return 0, 0, 0, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	// Scalar result
	return float64(v.(*model.Scalar).Value), 0, 0, nil
}

func captureJSONPathMetric(url, name, query string) (float64, float64, time.Duration, error) {
	// Fetch the URL
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req.WithContext(context.TODO()))
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// TODO ???
		}
	}()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		// TODO Should we not ignore this?
		return 0, 0, 0, nil
	}

	// Unmarshal as generic JSON
	data := make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		return 0, 0, 0, err
	}

	// Evaluate the JSON path
	jp := jsonpath.New(name)
	if err := jp.Parse(query); err != nil {
		return 0, 0, 0, err
	}
	values, err := jp.FindResults(data)
	if err != nil {
		return 0, 0, 0, err
	}

	// TODO No idea what we are looking for here...
	var r string
	for _, v := range values {
		for _, vv := range v {
			r = vv.String()
		}
	}
	value, err := strconv.ParseFloat(r, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return value, 0, 0, nil
}
