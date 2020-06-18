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
	"fmt"
	"path/filepath"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/patch"
	"github.com/redskyops/redskyops-controller/internal/server"
	"github.com/redskyops/redskyops-controller/redskyapi"
	expapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/util"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

var experimentFilename string

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export trial parameters",
		Long:  "Export trial parameters to a Kubernetes object",
		RunE:  doStuff,
	}

	cmd.Flags().StringVarP(&experimentFilename, "filename", "f", "", "path to experiment file")

	return cmd
}

func doStuff(cmd *cobra.Command, args []string) (err error) {
	cfg := &config.RedSkyConfig{}
	cfg.Load()

	// Read the experiment
	var experimentFile *redsky.Experiment
	experimentFile, err = util.ReadExperiment(experimentFilename, nil)
	if err != nil {
		return err
	}

	// Discover all trials for a given experiment
	rsoClient, err := redskyapi.NewClient(cmd.Context(), cfg, oauth2.NewClient(cmd.Context(), nil).Transport)
	if err != nil {
		return err
	}

	redskyapi := expapi.NewAPI(rsoClient)

	trialNames, err := experiments.ParseNames(append([]string{"trials"}, args...))
	if err != nil {
		return err
	}

	if len(trialNames) != 1 {
		return fmt.Errorf("only a single trial name is supported")
	}

	// TODO ensure trialNames[0] is a trial
	fmt.Println(trialNames[0].ExperimentName())

	experiment, err := redskyapi.GetExperimentByName(cmd.Context(), trialNames[0].ExperimentName())
	if err != nil {
		return err
	}

	query := &expapi.TrialListQuery{
		Status: []expapi.TrialStatus{expapi.TrialCompleted},
	}

	if experiment.TrialsURL == "" {
		return fmt.Errorf("unable to identify trial")
	}

	trialList, err := redskyapi.GetAllTrials(cmd.Context(), experiment.TrialsURL, query)
	if err != nil {
		return err
	}

	// Isolate the given trial we want by number
	var wantedTrial *expapi.TrialItem
	for _, trial := range trialList.Trials {
		if trial.Number == trialNames[0].Number {
			wantedTrial = &trial
			break
		}
	}

	if wantedTrial == nil {
		return fmt.Errorf("trial not found")
	}

	// Convert api trial to kube trial
	trial := &redsky.Trial{}
	server.ToClusterTrial(trial, &wantedTrial.TrialAssignments)
	// TODO: Look at trial.Spec.ExperimentRef
	trial.ObjectMeta.Labels = map[string]string{
		redsky.LabelExperiment: trialNames[0].ExperimentName().Name(),
	}

	// TODO: Figure out how to run setup tasks in dry-run mode

	// TODO: remove this
	// potentially pull from experiment.namespace?
	trial.ObjectMeta.Namespace = "default"

	// Generate patch operations
	patcher := patch.NewPatcher()
	for _, patch := range experimentFile.Spec.Patches {
		po, err := patcher.CreatePatchOperation(trial, &patch)
		if err != nil {
			return err
		}

		if po == nil {
			// TODO figure out how to find name
			return fmt.Errorf("failed to create a patch operation for %s", trial.ObjectMeta.GenerateName)
		}

		trial.Status.PatchOperations = append(trial.Status.PatchOperations, *po)
	}

	// Apply patch operations
	patches := make(map[string]types.Patch)
	for idx, patchOp := range trial.Status.PatchOperations {
		// TODO
		// what am i even doing here?
		// Kustomize only supports two types of patches ( StrategicMerge and JSONPatch )
		// https://github.com/kubernetes-sigs/kustomize/blob/master/api/types/kustomization.go#L39-L53
		// So in this instance, kustomize won't work directly
		// Instead we'll need to rely on kube tooling to apply the patch
		// Because we want to run the patch offline -- as in not patch an object in kubernetes
		// but patch a local copy of the object, we cant directly use client.Patch

		// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/patch.go
		// seems to only support 3/4 of patch types ( missing jsonpath/jsonpatch ) directly
		// so if we follow our current pattern of running RawPatch, this should allow us
		// to support all the patch types.
		// Unfortunately(?) running RawPatch.Data(targetRef) doesnt/wont actually do anything
		// because Data() just returns the actual patch data supplied when construting
		// So.. eh?
		// also, calling client.Patch() returns error, so even doing a dry run doesnt
		// seem to be an option (dumb?)
		// So what are our options?
		// - maybe create a fake client ?

		// TODO: for offline testing
		// output := pgDeployment

		patchBytes, err := createPatch(&patchOp)
		if err != nil {
			return err
		}

		p := types.Patch{
			Patch: string(patchBytes),
			Target: &types.Selector{
				Name:      patchOp.TargetRef.Name,
				Namespace: patchOp.TargetRef.Namespace,
			},
		}

		patches[fmt.Sprintf("%s-%d", "patch", idx)] = p
	}

	k := &types.Kustomization{}

	// Set up a kustomize target
	// TODO see how we can integrate this with kustomize pkg
	fs := filesys.MakeFsInMemory()
	base := "/app/base"
	for name, kpatch := range patches {
		if err = fs.WriteFile(filepath.Join(base, name), []byte(kpatch.Patch)); err != nil {
			return err
		}

		k.Patches = append(k.Patches, kpatch)
	}

	//if err = fs.WriteFile(filepath.Join(base, "src"), output); err != nil {
	if err = fs.WriteFile(filepath.Join(base, "src"), pgDeployment); err != nil {
		return err
	}
	k.Resources = append(k.Resources, "src")

	kYaml, err := yaml.Marshal(k)
	if err != nil {
		return err
	}
	if err = fs.WriteFile(filepath.Join(base, konfig.DefaultKustomizationFileName()), kYaml); err != nil {
		return err
	}
	kustomizer := krusty.MakeKustomizer(fs, krusty.MakeDefaultOptions())
	res, err := kustomizer.Run(base)
	if err != nil {
		return err
	}

	b, err := res.AsYaml()
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}

var pgDeployment = []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "annotations": {
            "deployment.kubernetes.io/revision": "2",
            "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"component\":\"postgres\"},\"name\":\"postgres\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"component\":\"postgres\"}},\"strategy\":{\"type\":\"Recreate\"},\"template\":{\"metadata\":{\"labels\":{\"component\":\"postgres\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"PGDATA\",\"value\":\"/var/lib/postgresql/data/pgdata\"},{\"name\":\"POSTGRES_DB\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_DATABASE\",\"name\":\"postgres-secret\"}}},{\"name\":\"POSTGRES_USER\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_USERNAME\",\"name\":\"postgres-secret\"}}},{\"name\":\"POSTGRES_PASSWORD\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_PASSWORD\",\"name\":\"postgres-secret\"}}}],\"image\":\"postgres:11.1-alpine\",\"livenessProbe\":{\"exec\":{\"command\":[\"pg_isready\",\"-h\",\"localhost\",\"-U\",\"test_user\",\"-d\",\"test_db\"]},\"initialDelaySeconds\":10,\"periodSeconds\":5},\"name\":\"postgres\",\"ports\":[{\"containerPort\":5432,\"name\":\"postgres\"}],\"readinessProbe\":{\"initialDelaySeconds\":15,\"periodSeconds\":5,\"tcpSocket\":{\"port\":5432}},\"resources\":{\"limits\":{\"cpu\":1,\"memory\":\"2000Mi\"},\"requests\":{\"cpu\":1,\"memory\":\"2000Mi\"}},\"securityContext\":{\"allowPrivilegeEscalation\":false,\"runAsUser\":70},\"volumeMounts\":[{\"mountPath\":\"/var/lib/postgresql/data\",\"name\":\"postgres-storage\"}]}],\"volumes\":[{\"emptyDir\":{},\"name\":\"postgres-storage\"}]}}}}\n"
        },
        "creationTimestamp": "2020-06-17T19:22:59Z",
        "generation": 2,
        "labels": {
            "component": "postgres"
        },
        "name": "postgres",
        "namespace": "default",
        "resourceVersion": "955",
        "selfLink": "/apis/apps/v1/namespaces/default/deployments/postgres",
        "uid": "a695754f-c17b-406f-b9a6-5bc74890b2ea"
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
        "conditions": [
            {
                "lastTransitionTime": "2020-06-17T19:24:27Z",
                "lastUpdateTime": "2020-06-17T19:24:27Z",
                "message": "Deployment has minimum availability.",
                "reason": "MinimumReplicasAvailable",
                "status": "True",
                "type": "Available"
            },
            {
                "lastTransitionTime": "2020-06-17T19:22:59Z",
                "lastUpdateTime": "2020-06-17T19:24:27Z",
                "message": "ReplicaSet \"postgres-679df96c5f\" has successfully progressed.",
                "reason": "NewReplicaSetAvailable",
                "status": "True",
                "type": "Progressing"
            }
        ],
        "observedGeneration": 2,
        "readyReplicas": 1,
        "replicas": 1,
        "updatedReplicas": 1
    }
}`)
