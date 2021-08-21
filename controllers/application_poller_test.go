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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPoller(t *testing.T) {
	pfalse := false
	one := api.FromInt64(1)
	twentyfive := api.FromInt64(25)
	fifty := api.FromInt64(50)

	testCases := []struct {
		desc     string
		tag      string
		expected applications.Template
	}{
		{
			desc: "scan",
			tag:  "scan",
			expected: applications.Template{
				Parameters: []applications.TemplateParameter{
					{
						Name: "nginx_cpu",
						Type: "int",
						Bounds: &applications.TemplateParameterBounds{
							Max: json.Number("2000"),
							Min: json.Number("25"),
						},
						Baseline: &fifty,
					},
					{
						Name: "nginx_memory",
						Type: "int",
						Bounds: &applications.TemplateParameterBounds{
							Max: json.Number("50"),
							Min: json.Number("12"),
						},
						Baseline: &twentyfive,
					}, {
						Name: "replicas",
						Type: "int",
						Bounds: &applications.TemplateParameterBounds{
							Max: json.Number("5"),
							Min: json.Number("1"),
						},
						Baseline: &one,
					},
				},
				Metrics: []applications.TemplateMetric{
					{
						Name:     "p95",
						Minimize: true,
					},
					{
						Name:     "cost",
						Minimize: true,
					},
					{
						Name:     "cost-cpu-requests",
						Minimize: true,
						Optimize: &pfalse,
					},
					{
						Name:     "cost-memory-requests",
						Minimize: true,
						Optimize: &pfalse,
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	optimizev1beta2.AddToScheme(scheme)
	optimizeappsv1alpha1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacv1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			os.Setenv("STORMFORGER_MYORG_JWT", "funnyjwtjokehere")
			defer os.Unsetenv("STORMFORGER_MYORG_JWT")

			ctx := context.WithValue(context.Background(), "tag", tc.tag)

			client := fake.NewFakeClientWithScheme(scheme)

			fapi := &fakeAPI{
				templateUpdateCh: make(chan struct{}),
				failureCh:        make(chan applications.ActivityFailure),
			}
			poller := &Poller{
				client:        client,
				log:           zapr.NewLogger(zap.NewNop()),
				apiClient:     fapi,
				kubectlExecFn: fakeKubectlExec,
			}

			_, err := poller.apiClient.CheckEndpoint(ctx)
			assert.NoError(t, err)

			// Start up the poller
			go func() {
				ch := make(chan struct{})
				poller.Start(ch)
			}()

			// How to best wait for this to be complete
			for {
				select {
				case <-fapi.templateUpdateCh:
					// Our fake api wont ever throw an error
					tmpl, _ := fapi.GetTemplate(ctx, "")
					assert.Equal(t, tc.expected, tmpl)
					return

				case err := <-fapi.failureCh:
					t.Log(err)
					assert.Nil(t, err)
					return

				case <-time.After(5 * time.Second):
					// Error
					t.Log("failed to get template update")
					t.Fail()
					return
				}
			}
		})
	}
}

func fakeKubectlExec(cmd *exec.Cmd) ([]byte, error) {
	return []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
    component: api
  namespace: engineering
spec:
  selector:
    matchLabels:
      component: api
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
        app.kubernetes.io: name=app-1
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          limits:
            memory: 25Mi
            cpu: 50m
          requests:
            memory: 25Mi
            cpu: 50m`), nil
}
