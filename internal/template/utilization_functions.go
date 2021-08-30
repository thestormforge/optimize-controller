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
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func cpuUtilization(data MetricData, labelSelectors ...string) (string, error) {
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

	return renderUtilization(data, labelSelectors, cpuUtilizationQueryTemplate)
}

func memoryUtilization(data MetricData, labelSelectors ...string) (string, error) {
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

	return renderUtilization(data, labelSelectors, memoryUtilizationQueryTemplate)
}

func cpuRequests(data MetricData, labelSelectors ...string) (string, error) {
	cpuResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_cpu_cores[{{ .Range }}])
    *
    on (pod) group_left
    max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
  )
)`

	return renderUtilization(data, labelSelectors, cpuResourcesQueryTemplate)
}

func memoryRequests(data MetricData, labelSelectors ...string) (string, error) {
	memoryResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_memory_bytes[{{ .Range }}])
    *
    on (pod) group_left
    max_over_time(kube_pod_labels{{ .MetricSelector }}[{{ .Range }}])
  )
)`

	return renderUtilization(data, labelSelectors, memoryResourcesQueryTemplate)
}

// https://github.com/prometheus/prometheus/blob/3240cf83f08e448e0b96a4a1f96c0e8b2d51cf61/util/strutil/strconv.go#L23
var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func renderUtilization(metricData MetricData, labelSelectors []string, query string) (string, error) {
	// We are accepting Kubernetes label selectors and using them to generate a PromQL metric selector
	sel, err := labels.Parse(strings.Join(labelSelectors, ","))
	if err != nil {
		return "", err
	}

	// Always include the trial namespace first
	requirements, _ := sel.Requirements()
	labelMatchers := make([]string, 0, 1+len(requirements))
	labelMatchers = append(labelMatchers, fmt.Sprintf("namespace=%q", metricData.Trial.Namespace))
	for _, req := range requirements {
		// Force a "label_" prefix
		key := strings.TrimPrefix(req.Key(), "label_")
		// Translate kube label into prometheus label
		key = invalidLabelCharRE.ReplaceAllString(key, "_")

		// If we got this far the cardinality will be correct (e.g. only one element for =)
		value := strings.Join(req.Values().List(), "|")

		// Convert the operator
		var op string
		switch req.Operator() {
		case selection.Equals, selection.DoubleEquals:
			op = "="
		case selection.NotEquals:
			op = "!="
		case selection.In:
			op = "=~"
		case selection.NotIn:
			op = "!~"
		case selection.Exists:
			op = "=~"
			value = ".+"
		case selection.DoesNotExist:
			op = "="
			value = ""
		default:
			return "", fmt.Errorf("unsupported label selector: %s", req.String())
		}

		labelMatchers = append(labelMatchers, fmt.Sprintf("label_%s%s%q", key, op, value))
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
