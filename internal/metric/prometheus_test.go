package metric

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusCheckReady(t *testing.T) {
	testCases := []struct {
		desc          string
		expectedError *CaptureError
		// Time we want to fetch metrics in prometheus at
		completedTime time.Time
		// Offset from completedTime that will be used to report target lastScrape time
		scrapeOffset time.Duration
	}{
		{
			desc:          "scrape done (1m after completion)",
			completedTime: time.Now().UTC().Add(-time.Minute),
			scrapeOffset:  +time.Minute,
		},
		{
			desc:          "scrape too soon (1s before completion)",
			completedTime: time.Now().UTC(),
			scrapeOffset:  -time.Second,
			expectedError: &CaptureError{Message: "waiting for final scrape", RetryAfter: scrapeInterval},
		},
		{
			desc:          "scrape not done (1m before completion)",
			completedTime: time.Now().UTC(),
			scrapeOffset:  -time.Minute,
			expectedError: &CaptureError{Message: "waiting for final scrape", RetryAfter: scrapeInterval},
		},
		{
			desc:          "future scrape (wait for n+1)",
			completedTime: time.Now().UTC(),
			scrapeOffset:  0,
			expectedError: &CaptureError{Message: "waiting for final scrape", RetryAfter: scrapeInterval},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {

			promSrv := promTargetsHttpTestServer(tc.completedTime.Add(tc.scrapeOffset))
			defer promSrv.Close()

			c, err := prom.NewClient(prom.Config{Address: promSrv.URL})
			require.NoError(t, err)

			_, err = checkReady(context.Background(), promv1.NewAPI(c), tc.completedTime)

			if tc.expectedError != nil {
				require.Error(t, err)

				assert.Equal(t, tc.expectedError.RetryAfter, err.(*CaptureError).RetryAfter)
				assert.Equal(t, tc.expectedError.Message, err.(*CaptureError).Message)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func promTargetsHttpTestServer(scrapeTime time.Time) *httptest.Server {
	respStr := `{"status":"success","data":{"activeTargets":[{"discoveredLabels":{"job":"kube-state-metrics"},"labels":{"instance":"localhost:8080","job":"kube-state-metrics"},"scrapePool":"kube-state-metrics","scrapeUrl":"http://localhost:8080/metrics","globalUrl":"http://optimize-default-prometheus-server-94df65748-bljzg:8080/metrics","lastError":"","lastScrape":%q,"lastScrapeDuration":0.0030478,"health":"up"},{"discoveredLabels":{"instance":"kind-control-plane","job":"kubernetes-cadvisor"},"labels":{"beta_kubernetes_io_arch":"amd64","beta_kubernetes_io_os":"linux","instance":"kind-control-plane","job":"kubernetes-cadvisor","kubernetes_io_arch":"amd64","kubernetes_io_hostname":"kind-control-plane","kubernetes_io_os":"linux"},"scrapePool":"kubernetes-cadvisor","scrapeUrl":"https://172.18.0.2:10250/metrics/cadvisor","globalUrl":"https://172.18.0.2:10250/metrics/cadvisor","lastError":"","lastScrape":%q,"lastScrapeDuration":0.0626849,"health":"up"},{"discoveredLabels":{"job":"prometheus-pushgateway"},"labels":{"instance":"localhost:9091","job":"prometheus-pushgateway"},"scrapePool":"prometheus-pushgateway","scrapeUrl":"http://localhost:9091/metrics","globalUrl":"http://optimize-default-prometheus-server-94df65748-bljzg:9091/metrics","lastError":"","lastScrape":%q,"lastScrapeDuration":0.0016028,"health":"up"}],"droppedTargets":[]}}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := scrapeTime.Format(time.RFC3339Nano)
		fmt.Fprintf(w, respStr, t, t, t)
	}))
}
