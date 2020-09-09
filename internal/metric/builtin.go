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
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
)

var BuiltIn = []redskyv1beta1.Metric{cpuUtilizationMetric}

var cpuUtilizationMetric = redskyv1beta1.Metric{
	Name: "rso cpu utilization",
	Type: redskyv1beta1.MetricBuiltIn,
	// TODO figure out how we want to inject labels
	Query: cpuUtilizationQuery,
}

var cpuUtilizationQuery = `
scalar(
  sum(
    sum(
      sum(kube_pod_container_status_running == 1) by (pod)
      *
      on (pod) group_left kube_pod_labels{{rsoTargetLabel .Trial}}
    ) by (pod)
    *
    on (pod) group_right max(container_cpu_usage_seconds_total{container="", image=""}) by (pod)
  )
  /
  sum(
    sum(
      sum(kube_pod_container_status_running == 1) by (pod)
      *
      on (pod) group_left kube_pod_labels{{rsoTargetLabel .Trial}}
    ) by (pod)
    *
    on (pod) group_left sum_over_time(kube_pod_container_resource_limits_cpu_cores[1h:1s])
  )
)
`
