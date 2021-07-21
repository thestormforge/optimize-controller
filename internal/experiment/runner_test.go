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

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	redskyv1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/apimachinery/pkg/runtime"

	//lint:ignore SA1019 backed out
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunner(t *testing.T) {
	pfalse := false

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
					},
					{
						Name: "nginx_memory",
						Type: "int",
						Bounds: &applications.TemplateParameterBounds{
							Max: json.Number("50"),
							Min: json.Number("12"),
						},
					}, {
						Name: "replicas",
						Type: "int",
						Bounds: &applications.TemplateParameterBounds{
							Max: json.Number("5"),
							Min: json.Number("1"),
						},
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
	redskyv1beta2.AddToScheme(scheme)
	redskyappsv1alpha1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacv1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			//lint:ignore SA1029 not important here
			ctx := context.WithValue(context.Background(), "tag", tc.tag)

			client := fake.NewFakeClientWithScheme(scheme)

			// appCh := make(chan *redskyappsv1alpha1.Application)
			fapi := &fakeAPI{templateUpdateCh: make(chan struct{})}
			runner := &Runner{
				client: client,
				//	apiClient:     applications.NewAPI(c),
				apiClient:     fapi,
				kubectlExecFn: fakeKubectlExec,
				errCh:         make(chan error),
			}

			_, err := runner.apiClient.CheckEndpoint(ctx)
			assert.NoError(t, err)

			// Start up the runner
			go func() { runner.Run(ctx) }()

			// How to best wait for this to be complete
			for {
				select {
				case <-fapi.templateUpdateCh:
					// Our fake api wont ever throw an error
					tmpl, _ := fapi.GetTemplate(ctx, "")
					assert.Equal(t, tc.expected, tmpl)
					return

				case err := <-runner.errCh:
					assert.NoError(t, err)
					return

				case <-time.After(2 * time.Second):
					// Error
					t.Log("failed to get template update")
					t.Fail()
					return
				}
			}

			/*
				expCh := make(chan struct{})
				// Makeshift watcher; trigger experiment verification tests
				// when we see the experiment through our fake client
				go func() {
					exp := &redskyv1beta2.Experiment{}
					for {
						select {
						case <-time.Tick(100 * time.Millisecond):
							if err := client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, exp); err == nil {
								expCh <- struct{}{}
								return
							}
						case <-ctx.Done():
							return
						}
					}
				}()

				// Trigger runner
				go func() {
					var updatedApp *redskyappsv1alpha1.Application
					if tc.err == nil {
						// Create a copy of the app
						updatedApp = &redskyappsv1alpha1.Application{}
						tc.app.DeepCopyInto(updatedApp)
					}

					appCh <- tc.app

					// Trigger an update of the existing experiment
					if updatedApp != nil {
						appCh <- updatedApp
					}
				}()

				for {
					select {
					case <-ctx.Done():
						return

					case <-expCh:
						exp := &redskyv1beta2.Experiment{}
						err := client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, exp)
						assert.NoError(t, err)

						// Since we explicitly set replicas, this should never be nil
						assert.NotNil(t, exp.Spec.Replicas)

						if _, ok := tc.app.Annotations[redskyappsv1alpha1.AnnotationUserConfirmed]; !ok {
							assert.Equal(t, int32(0), *exp.Spec.Replicas)
							return
						}

						assert.Equal(t, int32(1), *exp.Spec.Replicas)

						serviceAccount := &corev1.ServiceAccount{}
						err = client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, serviceAccount)
						assert.NoError(t, err)
						assert.NotNil(t, serviceAccount)

						clusterRole := &rbacv1.ClusterRole{}
						err = client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, clusterRole)
						assert.NoError(t, err)
						assert.NotNil(t, clusterRole)

						clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
						err = client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, clusterRoleBinding)
						assert.NoError(t, err)
						assert.NotNil(t, clusterRoleBinding)

						cancel()

					case err := <-runner.errCh:
						// Handle expected errors
						if tc.err != nil {
							assert.Error(t, err)
							assert.Equal(t, tc.err.Error(), err.Error())
							cancel()

							continue
						}

						// Handle unexpected errors
						assert.NoError(t, err)
						cancel()
					}
				}
			*/

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

/*
var invalidExperimentNoParams = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
}

var invalidExperimentNoObjectives = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
}

var success = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
	Objectives: []redskyappsv1alpha1.Objective{
		{
			Goals: []redskyappsv1alpha1.Goal{
				{
					Duration: &redskyappsv1alpha1.DurationGoal{
						DurationType: "trial",
					},
				},
			},
		},
	},
	Scenarios: []redskyappsv1alpha1.Scenario{
		{
			Name: "how-do-you-make-a-tissue-dance",
			StormForger: &redskyappsv1alpha1.StormForgerScenario{
				TestCase: "put-a-little-boogie-in-it",
			},
		},
	},
	StormForger: &redskyappsv1alpha1.StormForger{
		Organization: "sf",
		AccessToken: &redskyappsv1alpha1.StormForgerAccessToken{
			Literal: "Why couldn't the bicycle stand up by itself? It was two tired!",
		},
	},
}

var confirmed = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
		Annotations: map[string]string{
			redskyappsv1alpha1.AnnotationUserConfirmed: "true",
		},
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
	Objectives: []redskyappsv1alpha1.Objective{
		{
			Goals: []redskyappsv1alpha1.Goal{
				{
					Duration: &redskyappsv1alpha1.DurationGoal{
						DurationType: "trial",
					},
				},
			},
		},
	},
	Scenarios: []redskyappsv1alpha1.Scenario{
		{
			Name: "how-do-you-make-a-tissue-dance",
			StormForger: &redskyappsv1alpha1.StormForgerScenario{
				TestCase: "put-a-little-boogie-in-it",
			},
		},
	},
	StormForger: &redskyappsv1alpha1.StormForger{
		Organization: "sf",
		AccessToken: &redskyappsv1alpha1.StormForgerAccessToken{
			Literal: "Why couldn't the bicycle stand up by itself? It was two tired!",
		},
	},
}
*/
