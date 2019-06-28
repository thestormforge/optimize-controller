package trial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/util/jsonpath"
)

// TODO We need some type of client util to encapsulate this
// TODO Combine it with the Prometheus clients?
var httpClient = &http.Client{Timeout: 10 * time.Second}

func captureMetric(m *redskyv1alpha1.Metric, u string, trial *redskyv1alpha1.Trial) (float64, float64, *time.Duration, error) {
	// Execute the query as a template against the current state of the trial
	q, err := executeMetricQueryTemplate(m, trial)
	if err != nil {
		return 0, 0, nil, err
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
		return 0, 0, nil, fmt.Errorf("unknown metric type: %s", m.Type)
	}
}

func captureLocalMetric(query string) (float64, float64, *time.Duration, error) {
	// Just parse the query as a float
	value, err := strconv.ParseFloat(query, 64)
	return value, 0, nil, err
}

func capturePrometheusMetric(address, query string, completionTime time.Time) (float64, float64, *time.Duration, error) {
	// Get the Prometheus client based on the metric URL
	// TODO Cache these by URL
	c, err := prom.NewClient(prom.Config{Address: address})
	if err != nil {
		return 0, 0, nil, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	ts, err := promAPI.Targets(context.TODO())
	if err != nil {
		return 0, 0, nil, err
	}
	for _, t := range ts.Active {
		if t.LastScrape.Before(completionTime) {
			// TODO Can we make a more informed delay?
			delay := 5 * time.Second
			return 0, 0, &delay, nil
		}
	}

	// Execute query
	v, err := promAPI.Query(context.TODO(), query, completionTime)
	if err != nil {
		return 0, 0, nil, err
	}

	// Only accept scalar results
	if v.Type() != model.ValScalar {
		return 0, 0, nil, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	// Scalar result
	return float64(v.(*model.Scalar).Value), 0, nil, nil
}

func captureJSONPathMetric(url, name, query string) (float64, float64, *time.Duration, error) {
	// Fetch the URL
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req.WithContext(context.TODO()))
	if err != nil {
		return 0, 0, nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// TODO ???
		}
	}()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		// TODO Should we not ignore this?
		return 0, 0, nil, nil
	}

	// Unmarshal as generic JSON
	data := make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		return 0, 0, nil, err
	}

	// Evaluate the JSON path
	jp := jsonpath.New(name)
	if err := jp.Parse(query); err != nil {
		return 0, 0, nil, err
	}
	values, err := jp.FindResults(data)
	if err != nil {
		return 0, 0, nil, err
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
		return 0, 0, nil, err
	}
	return value, 0, nil, nil
}
