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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCaptureMetric(t *testing.T) {
	// Offset all times by 10min
	// Primarily to work with remote prometheus server, but no harm in baselining
	// all tests this way
	now := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Minute))
	later := metav1.NewTime(now.Add(5 * time.Second))

	log := zap.New(zap.UseDevMode(true))

	jsonHttpTest := jsonPathHttpTestServer()
	defer jsonHttpTest.Close()

	promHttpTest := promHttpTestServer()
	defer promHttpTest.Close()

	testCases := []struct {
		desc     string
		metric   *optimizev1beta2.Metric
		obj      runtime.Object
		expected float64
	}{
		{
			desc: "default kubernetes",
			metric: &optimizev1beta2.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
			},
			expected: 5,
		},
		{
			desc: "explicit kubernetes",
			metric: &optimizev1beta2.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  optimizev1beta2.MetricKubernetes,
			},
			expected: 5,
		},
		{
			desc: "kubernetes target",
			metric: &optimizev1beta2.Metric{
				Name:  "testMetric",
				Query: "{{with index .Target.Items 0}}{{ (indexResource .Usage \"cpu\").MilliValue }}{{ end }}",
				Type:  optimizev1beta2.MetricKubernetes,
			},
			obj: &metricsv1beta1.NodeMetricsList{
				Items: []metricsv1beta1.NodeMetrics{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "minikube",
						},
						Timestamp: metav1.Now(),
						Window:    metav1.Duration{Duration: 1 * time.Minute},
						Usage: corev1.ResourceList{
							"cpu": resource.MustParse("289m"),
						},
					},
				},
			},
			expected: 289,
		},
		{
			desc: "prometheus url",
			metric: &optimizev1beta2.Metric{
				Name:  "testMetric",
				Query: "scalar(prometheus_build_info)",
				Type:  optimizev1beta2.MetricPrometheus,
				URL:   promHttpTest.URL,
			},
			expected: 1,
		},

		{
			desc: "jsonpath url",
			metric: &optimizev1beta2.Metric{
				Name:  "testMetric",
				Query: "{.current_response_time_percentile_95}",
				Type:  optimizev1beta2.MetricJSONPath,
				URL:   jsonHttpTest.URL,
			},
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			log := log.WithValues("test", tc.desc)
			trial := &optimizev1beta2.Trial{
				Status: optimizev1beta2.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			}

			duration, _, err := CaptureMetric(context.TODO(), log, trial, tc.metric, tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, duration)
		})
	}
}

func jsonPathHttpTestServer() *httptest.Server {
	response := map[string]int{"current_response_time_percentile_95": 5}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response)
	}))
}

func promHttpTestServer() *httptest.Server {
	resp := `{"status":"success","data":{"resultType":"scalar","result":[1595471900.283,"1"]}}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, resp)
	}))
}
