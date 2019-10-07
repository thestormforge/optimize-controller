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
	"os"
	"time"

	datadog "github.com/zorkian/go-datadog-api"
)

func captureDatadogMetric(query string, startTime, completionTime time.Time) (float64, float64, error) {
	apiKey := os.Getenv("DATADOG_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("DD_API_KEY")
	}

	applicationKey := os.Getenv("DATADOG_APP_KEY")
	if applicationKey == "" {
		apiKey = os.Getenv("DD_APP_KEY")
	}

	client := datadog.NewClient(apiKey, applicationKey)

	metrics, err := client.QueryMetrics(startTime.Unix(), completionTime.Unix(), query)
	if err != nil {
		return 0, 0, err
	}

	// TODO Is it reasonable that we would get a single data point?

	if len(metrics) != 1 {
		return 0, 0, fmt.Errorf("expected one series")
	}

	if len(metrics[0].Points) != 1 {
		return 0, 0, fmt.Errorf("expected one data point")
	}

	return *metrics[0].Points[0][1], 0, nil
}
