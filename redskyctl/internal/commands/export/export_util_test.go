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

package export

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/config"
	expapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createTempExperimentFile(t *testing.T) (*redsky.Experiment, *os.File) {
	samplePatch := `spec:
  template:
    spec:
      containers:
        - name: postgres
          resources:
            limits:
              cpu: "{{ .Values.cpu }}m"
              memory: "{{ .Values.memory }}Mi"
            requests:
              cpu: "{{ .Values.cpu }}m"
              memory: "{{ .Values.memory }}Mi"`

	sampleExperiment := &redsky.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sampleExperiment",
			Namespace: "default",
		},
		Spec: redsky.ExperimentSpec{
			Parameters: []redsky.Parameter{},
			Metrics:    []redsky.Metric{},
			Patches: []redsky.PatchTemplate{
				{
					TargetRef: &corev1.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Name:       "postgres",
					},
					Patch: samplePatch,
				},
			},
			TrialTemplate: redsky.TrialTemplateSpec{},
		},
		Status: redsky.ExperimentStatus{},
	}

	tmpfile, err := ioutil.TempFile("", "experiment")
	require.NoError(t, err)

	b, err := json.Marshal(sampleExperiment)
	require.NoError(t, err)

	_, err = tmpfile.Write(b)
	require.NoError(t, err)

	return sampleExperiment, tmpfile
}

var wannabeTrial = expapi.TrialItem{
	// This is our target trial
	TrialAssignments: expapi.TrialAssignments{
		Assignments: []expapi.Assignment{
			{
				ParameterName: "cpu",
				Value:         "100",
			},
			{
				ParameterName: "memory",
				Value:         "200",
			},
		},
	},
	Number: 1234,
	Status: expapi.TrialCompleted,
}

type fakeClient struct {
	filename string
}

func (f *fakeClient) Endpoints() (config.Endpoints, error) { return nil, nil }

func (f *fakeClient) Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error) {
	return nil, nil
}

func (f *fakeClient) SystemNamespace() (string, error) { return "", nil }

func (f *fakeClient) Kubectl(ctx context.Context, arg ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/paste", f.filename)
	return cmd, nil
}

// Implement the api interface
type fakeRedSkyServer struct{}

func (f *fakeRedSkyServer) Options(ctx context.Context) (expapi.ServerMeta, error) {
	return expapi.ServerMeta{}, nil
}

func (f *fakeRedSkyServer) GetAllExperiments(ctx context.Context, query *expapi.ExperimentListQuery) (expapi.ExperimentList, error) {
	return expapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetAllExperimentsByPage(ctx context.Context, notsure string) (expapi.ExperimentList, error) {
	return expapi.ExperimentList{}, nil
}

func (f *fakeRedSkyServer) GetExperimentByName(ctx context.Context, name expapi.ExperimentName) (expapi.Experiment, error) {
	exp := expapi.Experiment{
		ExperimentMeta: expapi.ExperimentMeta{
			TrialsURL: "http://sometrial",
		},
		DisplayName: "postgres-example",
		Metrics: []expapi.Metric{
			{
				Name:     "cost",
				Minimize: true,
			},
			{
				Name:     "duration",
				Minimize: true,
			},
		},
		Observations: 320,
		Optimization: []expapi.Optimization{},
		Parameters: []expapi.Parameter{
			{
				Name: "cpu",
				Type: expapi.ParameterTypeInteger,
				Bounds: expapi.Bounds{
					Max: "4000",
					Min: "100",
				},
			},
			{
				Name: "memory",
				Type: expapi.ParameterTypeInteger,
				Bounds: expapi.Bounds{
					Max: "4000",
					Min: "500",
				},
			},
		},
	}

	return exp, nil
}

func (f *fakeRedSkyServer) GetExperiment(ctx context.Context, name string) (expapi.Experiment, error) {
	return expapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) CreateExperiment(ctx context.Context, name expapi.ExperimentName, exp expapi.Experiment) (expapi.Experiment, error) {
	return expapi.Experiment{}, nil
}

func (f *fakeRedSkyServer) DeleteExperiment(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) GetAllTrials(ctx context.Context, name string, query *expapi.TrialListQuery) (expapi.TrialList, error) {
	// TODO implement some query filter magic if we really want to.
	// Otherwise, we should clean this up to mimic just the necessary output
	tl := expapi.TrialList{
		Trials: []expapi.TrialItem{
			wannabeTrial,
			// This one should not be used because number doesnt match up
			{
				TrialAssignments: expapi.TrialAssignments{
					Assignments: []expapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         "999",
						},
						{
							ParameterName: "memory",
							Value:         "999",
						},
					},
				},
				Number: 319,
				Status: expapi.TrialCompleted,
			},
			// This one should not be used because status is not completed
			{
				TrialAssignments: expapi.TrialAssignments{
					Assignments: []expapi.Assignment{
						{
							ParameterName: "cpu",
							Value:         "999",
						},
						{
							ParameterName: "memory",
							Value:         "999",
						},
					},
				},
				Number: 320,
				Status: expapi.TrialFailed,
			},
		},
	}
	return tl, nil
}

func (f *fakeRedSkyServer) CreateTrial(ctx context.Context, name string, assignments expapi.TrialAssignments) (string, error) {
	return "", nil
}

func (f *fakeRedSkyServer) NextTrial(ctx context.Context, name string) (expapi.TrialAssignments, error) {
	return expapi.TrialAssignments{}, nil
}

func (f *fakeRedSkyServer) ReportTrial(ctx context.Context, name string, values expapi.TrialValues) error {
	return nil
}

func (f *fakeRedSkyServer) AbandonRunningTrial(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRedSkyServer) LabelExperiment(ctx context.Context, name string, labels expapi.ExperimentLabels) error {
	return nil
}

func (f *fakeRedSkyServer) LabelTrial(ctx context.Context, name string, labels expapi.TrialLabels) error {
	return nil
}

// hack to allow offline testing
var pgDeployment = []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "annotations": {
            "deployment.kubernetes.io/revision": "2"
        },
        "generation": 2,
        "labels": {
            "component": "postgres"
        },
        "name": "postgres",
        "namespace": "default"
    },
    "spec": {
        "progressDeadlineSeconds": 600,
        "replicas": 1,
        "revisionHistoryLimit": 10,
        "selector": {
            "matchLabels": {
                "component": "postgres"
            }
        },
        "strategy": {
            "type": "Recreate"
        },
        "template": {
            "metadata": {
                "creationTimestamp": null,
                "labels": {
                    "component": "postgres"
                }
            },
            "spec": {
                "containers": [
                    {
                        "env": [
                            {
                                "name": "PGDATA",
                                "value": "/var/lib/postgresql/data/pgdata"
                            },
                            {
                                "name": "POSTGRES_DB",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_DATABASE",
                                        "name": "postgres-secret"
                                    }
                                }
                            },
                            {
                                "name": "POSTGRES_USER",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_USERNAME",
                                        "name": "postgres-secret"
                                    }
                                }
                            },
                            {
                                "name": "POSTGRES_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_PASSWORD",
                                        "name": "postgres-secret"
                                    }
                                }
                            }
                        ],
                        "image": "postgres:11.1-alpine",
                        "imagePullPolicy": "IfNotPresent",
                        "livenessProbe": {
                            "exec": {
                                "command": [
                                    "pg_isready",
                                    "-h",
                                    "localhost",
                                    "-U",
                                    "test_user",
                                    "-d",
                                    "test_db"
                                ]
                            },
                            "failureThreshold": 3,
                            "initialDelaySeconds": 10,
                            "periodSeconds": 5,
                            "successThreshold": 1,
                            "timeoutSeconds": 1
                        },
                        "name": "postgres",
                        "ports": [
                            {
                                "containerPort": 5432,
                                "name": "postgres",
                                "protocol": "TCP"
                            }
                        ],
                        "readinessProbe": {
                            "failureThreshold": 3,
                            "initialDelaySeconds": 15,
                            "periodSeconds": 5,
                            "successThreshold": 1,
                            "tcpSocket": {
                                "port": 5432
                            },
                            "timeoutSeconds": 1
                        },
                        "resources": {
                            "limits": {
                                "cpu": "424m",
                                "memory": "981Mi"
                            },
                            "requests": {
                                "cpu": "424m",
                                "memory": "981Mi"
                            }
                        },
                        "securityContext": {
                            "allowPrivilegeEscalation": false,
                            "runAsUser": 70
                        },
                        "terminationMessagePath": "/dev/termination-log",
                        "terminationMessagePolicy": "File",
                        "volumeMounts": [
                            {
                                "mountPath": "/var/lib/postgresql/data",
                                "name": "postgres-storage"
                            }
                        ]
                    }
                ],
                "dnsPolicy": "ClusterFirst",
                "restartPolicy": "Always",
                "schedulerName": "default-scheduler",
                "securityContext": {},
                "terminationGracePeriodSeconds": 30,
                "volumes": [
                    {
                        "emptyDir": {},
                        "name": "postgres-storage"
                    }
                ]
            }
        }
    },
    "status": {
        "availableReplicas": 1,
        "observedGeneration": 2,
        "readyReplicas": 1,
        "replicas": 1,
        "updatedReplicas": 1
    }
}`)
