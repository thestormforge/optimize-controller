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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCaptureMetric(t *testing.T) {
	// Offset all times by 10min
	// Primarily to work with remote prometheus server, but no harm in baselining
	// all tests this way
	now := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Minute))
	later := metav1.NewTime(now.Add(5 * time.Second))

	jsonHttpTest := jsonPathHttpTestServer()
	defer jsonHttpTest.Close()
	jurl, err := url.Parse(jsonHttpTest.URL)
	require.NoError(t, err)
	jsonHttpTestIP, sPort, err := net.SplitHostPort(jurl.Host)
	require.NoError(t, err)
	jsonHttpTestPort, err := strconv.ParseInt(sPort, 10, 32)
	require.NoError(t, err)

	testCases := []struct {
		desc     string
		metric   *redskyv1alpha1.Metric
		obj      runtime.Object
		expected float64
	}{
		{
			desc: "default local",
			metric: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
			},
			obj:      &corev1.PodList{},
			expected: 5,
		},
		{
			desc: "explicit local",
			metric: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  redskyv1alpha1.MetricLocal,
			},
			obj:      &corev1.PodList{},
			expected: 5,
		},
		{
			desc: "default prometheus",
			metric: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "scalar(prometheus_build_info)",
				Type:  redskyv1alpha1.MetricPrometheus,
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "demo.robustperception.io",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
							},
						},
					},
				},
			},
			expected: 1,
		},
		{
			desc: "default jonPath",
			metric: &redskyv1alpha1.Metric{
				Name:  "testMetric",
				Query: "{.current_response_time_percentile_95}",
				Type:  redskyv1alpha1.MetricJSONPath,
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: jsonHttpTestIP,
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     int32(jsonHttpTestPort),
								},
							},
						},
					},
				},
			},
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			trial := &redskyv1alpha1.Trial{
				Status: redskyv1alpha1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			}

			duration, _, err := CaptureMetric(tc.metric, trial, tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, duration)
		})
	}
}

func jsonPathHttpTestServer() *httptest.Server {
	response := map[string]int{"current_response_time_percentile_95": 5}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response)
		return
	}))
}
