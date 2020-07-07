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
	"math"
	"os"
	"time"

	datadog "github.com/zorkian/go-datadog-api"
)

func captureDatadogMetric(aggregator, query string, startTime, completionTime time.Time) (float64, float64, error) {
	apiKey := os.Getenv("DATADOG_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("DD_API_KEY")
	}

	applicationKey := os.Getenv("DATADOG_APP_KEY")
	if applicationKey == "" {
		applicationKey = os.Getenv("DD_APP_KEY")
	}

	client := datadog.NewClient(apiKey, applicationKey)

	metrics, err := client.QueryMetrics(startTime.Unix(), completionTime.Unix(), query)
	if err != nil {
		return 0, 0, err
	}

	if len(metrics) != 1 {
		return 0, 0, fmt.Errorf("expected one series")
	}

	var value, n float64
	for _, p := range metrics[0].Points {
		if p[1] == nil {
			continue
		}

		// TODO What is `metrics[0].Aggr`?
		switch aggregator {
		case "avg", "":
			value = value + *p[1]
			n++
		case "last":
			value = *p[1]
		case "max":
			value = math.Max(value, *p[1])
		case "min":
			value = math.Min(value, *p[1])
		case "sum":
			value = value + *p[1]
		default:
			return 0, 0, fmt.Errorf("unsupported aggregator: %s (expected: avg, last, max, min, sum)", aggregator)
		}
	}

	if n > 0 {
		value = value / n
	}

	return value, 0, nil
}
