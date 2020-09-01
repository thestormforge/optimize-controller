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
	Name:  "cpu utilization",
	Type:  redskyv1beta1.MetricBuiltIn,
	Query: ``,
}
