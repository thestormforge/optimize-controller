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

package template

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// https://github.com/prometheus/prometheus/blob/3240cf83f08e448e0b96a4a1f96c0e8b2d51cf61/util/strutil/strconv.go#L23
var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func cpuUtilization(data MetricData, labelMatchers ...string) (string, error) {
	cpuUtilizationQueryTemplate := `
scalar(
  round(
    (
      sum(
        sum(
          increase(container_cpu_usage_seconds_total{container="", image=""}[{{ .Range }}])
        ) by (pod)
        *
        on (pod) group_left
        max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
      )
      /
      sum(
        sum_over_time(kube_pod_container_resource_requests_cpu_cores[{{ .Range }}:1s])
        *
        on (pod) group_left
        max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
      )
    )
  * 100, 0.0001)
)`

	return renderUtilization(data, labelMatchers, cpuUtilizationQueryTemplate)
}

func memoryUtilization(data MetricData, labelMatchers ...string) (string, error) {
	memoryUtilizationQueryTemplate := `
scalar(
  round(
    (
      avg(
        max(
          container_memory_max_usage_bytes
        ) by (pod)
        *
        on (pod) group_left
        max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
        /
        sum(
          kube_pod_container_resource_requests_memory_bytes
        ) by (pod)
      )
    )
  * 100, 0.0001)
)`

	return renderUtilization(data, labelMatchers, memoryUtilizationQueryTemplate)
}

func cpuRequests(data MetricData, labelMatchers ...string) (string, error) {
	cpuResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_cpu_cores[{{ .Range }}])
    *
    on (pod) group_left
    max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
  )
)`

	return renderUtilization(data, labelMatchers, cpuResourcesQueryTemplate)
}

func memoryRequests(data MetricData, labelMatchers ...string) (string, error) {
	memoryResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_memory_bytes[{{ .Range }}])
    *
    on (pod) group_left
    max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
  )
)`

	return renderUtilization(data, labelMatchers, memoryResourcesQueryTemplate)
}

func renderUtilization(metricData MetricData, extraLabelMatchers []string, query string) (string, error) {
	// NOTE: Ideally we would use `github.com/prometheus/prometheus/promql/parser#ParseMetricSelector` here
	// however the dependencies of the Prometheus server conflict.

	// Allow the extra label matchers to be variadic or comma-delimited
	extraLabelMatchers = strings.Split(strings.Join(extraLabelMatchers, ","), ",")

	// Always include the trial namespace first
	labelMatchers := make([]string, 0, 1+len(extraLabelMatchers))
	labelMatchers = append(labelMatchers, fmt.Sprintf("namespace=\"%s\"", metricData.Trial.Namespace))
	for _, labelMatcher := range extraLabelMatchers {
		if labelMatcher == "" {
			continue
		}

		key, op, value, err := splitLabelMatcher(labelMatcher)
		if err != nil {
			return "", err
		}

		labelMatchers = append(labelMatchers, key+op+value)
	}

	// Add the metric selector start and end markers (we know it will be non-empty because of the namespace)
	metricSelector := fmt.Sprintf("{%s}", strings.Join(labelMatchers, ","))

	// Wrap the standard metric data with the additional MetricSelector
	input := struct {
		MetricData
		MetricSelector string
	}{
		MetricData:     metricData,
		MetricSelector: metricSelector,
	}

	// Panic if the query in the source code does not parse
	tmpl := template.Must(template.New("query").Parse(query))

	// Execute the template
	var output bytes.Buffer
	if err := tmpl.Execute(&output, input); err != nil {
		return "", err
	}

	return output.String(), nil
}

// splitLabelMatcher returns the label name, operator and value from a label matcher.
func splitLabelMatcher(lm string) (string, string, string, error) {
	kv := regexp.MustCompile(`(=|!=|=~|!~)`).Split(lm, 2)
	if len(kv) != 2 {
		return "", "", "", fmt.Errorf("invalid label matcher: %s", lm)
	}

	key := strings.TrimSpace(kv[0])
	op := lm[len(kv[0]) : len(lm)-len(kv[1])]
	value := strings.TrimSpace(kv[1])

	// Allow quotes to be optional since specifying them in the Go template would make things complicated
	if !strings.HasPrefix(value, `"`) {
		value = strconv.Quote(value)
	}

	// TODO We should remove this
	// In the case of our built-in Prometheus, we can just alter the scape configs to not include the prefix
	// In the case of a managed Prometheus we won't have control over this prefix and it may interfere with metric selection
	if !strings.HasPrefix(key, "label_") {
		key = "label_" + key
	}

	// TODO We should remove this
	// Instead of sanitizing, we should just fail if the key contains invalid values
	key = invalidLabelCharRE.ReplaceAllString(key, "_")

	return key, op, value, nil
}
