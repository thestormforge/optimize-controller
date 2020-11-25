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

package v1alpha1

import (
	"testing"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestConvert_v1alpha1_Metric_To_v1beta1_Metric(t *testing.T) {
	// The goal of this test is to confirm that if you start with v1alpha1 data, you should be safe
	// as we can do a round trip conversion. The same isn't necessarily true if you start with v1beta1.

	cases := []struct {
		v1alpha1Metric Metric
		v1beta1Metric  redskyv1beta1.Metric
	}{
		{
			v1alpha1Metric: Metric{
				Name:  "constant",
				Query: "1.1",
			},
			v1beta1Metric: redskyv1beta1.Metric{
				Name:  "constant",
				Query: "1.1",
			},
		},

		{
			v1alpha1Metric: Metric{
				Name:  "throughput",
				Type:  "jsonpath",
				Query: `{.total}`,
				Path:  "/metrics",
				Port:  intstr.FromInt(5000),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"component": "result-exporter"},
				},
			},
			// If you start with v1alpha1, we preserve the selector and use "redskyops.dev" as a placeholder
			v1beta1Metric: redskyv1beta1.Metric{
				Name:  "throughput",
				Type:  redskyv1beta1.MetricJSONPath,
				Query: `{.total}`,
				URL:   "http://redskyops.dev:5000/metrics",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"component": "result-exporter"},
				},
			},
		},

		{
			v1alpha1Metric: Metric{
				Name:     "cost",
				Minimize: true,
				Type:     "pods",
				Query:    `{{resourceRequests .Pods "cpu=0.022,memory=0.000000000003"}}`,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "voting-app"},
				},
			},
			// In v1beta1 you need to change the type to "kubernetes" and add a pod target reference
			v1beta1Metric: redskyv1beta1.Metric{
				Name:     "cost",
				Minimize: true,
				Type:     redskyv1beta1.MetricKubernetes,
				Query:    `{{resourceRequests .Pods "cpu=0.022,memory=0.000000000003"}}`,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "voting-app"},
				},
			},
		},

		{
			v1alpha1Metric: Metric{
				Name:     "cost",
				Minimize: true,
				Type:     "prometheus",
				Query:    `({{ cpuRequests . "app=postgres" }} * 17) + ({{ memoryRequests . "app=postgres" | GB }} * 3)`,
				Port:     intstr.FromInt(9090),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "prometheus"},
				},
			},
			// This takes a v1alpha1 metric for the built-in Prometheus and preserves default behaviors
			// NOTE: Since no URL is generated, it will be defaulted later
			// NOTE: Having a port number of 9090 is required to get the conversion to round trip correctly
			v1beta1Metric: redskyv1beta1.Metric{
				Name:     "cost",
				Minimize: true,
				Type:     redskyv1beta1.MetricPrometheus,
				Query:    `({{ cpuRequests . "app=postgres" }} * 17) + ({{ memoryRequests . "app=postgres" | GB }} * 3)`,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "prometheus"},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.v1alpha1Metric.Name+"_v1alpha1_to_v1beta1", func(t *testing.T) {
			in := c.v1alpha1Metric
			actual := redskyv1beta1.Metric{}
			if err := Convert_v1alpha1_Metric_To_v1beta1_Metric(&in, &actual, nil); assert.NoError(t, err) {
				assert.Equal(t, c.v1beta1Metric, actual)
			}
			if err := Convert_v1beta1_Metric_To_v1alpha1_Metric(&actual, &in, nil); assert.NoError(t, err) {
				assert.Equal(t, c.v1alpha1Metric, in)
			}
		})
		t.Run(c.v1beta1Metric.Name+"_v1beta1_to_v1alpha1", func(t *testing.T) {
			in := c.v1beta1Metric
			actual := Metric{}
			if err := Convert_v1beta1_Metric_To_v1alpha1_Metric(&in, &actual, nil); assert.NoError(t, err) {
				assert.Equal(t, c.v1alpha1Metric, actual)
			}
			if err := Convert_v1alpha1_Metric_To_v1beta1_Metric(&actual, &in, nil); assert.NoError(t, err) {
				assert.Equal(t, c.v1beta1Metric, in)
			}
		})
	}
}

func TestConvert_v1beta1_Metric_To_v1alpha1_Metric(t *testing.T) {
	cases := []struct {
		v1beta1Metric  redskyv1beta1.Metric
		v1alpha1Metric Metric
	}{
		{
			v1beta1Metric: redskyv1beta1.Metric{
				Name:     "cpu_requests",
				Minimize: true,
				Type:     redskyv1beta1.MetricPrometheus,
				Query:    "{{ cpuRequests . \"app=postgres\" }}",
				URL:      "http://my-prometheus.monitoring:9999",
			},
			v1alpha1Metric: Metric{
				Name:     "cpu_requests",
				Minimize: true,
				Type:     "prometheus",
				Query:    "{{ cpuRequests . \"app=postgres\" }}",
				Port:     intstr.FromInt(9999),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.v1alpha1Metric.Name, func(t *testing.T) {
			in := c.v1beta1Metric
			actual := Metric{}
			if err := Convert_v1beta1_Metric_To_v1alpha1_Metric(&in, &actual, nil); assert.NoError(t, err) {
				assert.Equal(t, c.v1alpha1Metric, actual)
			}

			// But now the reverse is not true (likely because we can't reconstruct the URL)
			if err := Convert_v1alpha1_Metric_To_v1beta1_Metric(&actual, &in, nil); assert.NoError(t, err) {
				assert.NotEqual(t, c.v1beta1Metric, in)
			}
		})
	}
}
