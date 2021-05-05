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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"
)

type Runner struct {
	client        client.Client
	appCh         chan *redskyappsv1alpha1.Application
	errCh         chan error
	kubectlExecFn func(cmd *exec.Cmd) ([]byte, error)
}

func New(kclient client.Client, appCh chan *redskyappsv1alpha1.Application) (*Runner, chan error) {
	errCh := make(chan error)

	return &Runner{
		client:        kclient,
		appCh:         appCh,
		errCh:         errCh,
		kubectlExecFn: inClusterKubectl,
	}, errCh
}

// This doesnt necessarily need to live here, but seemed to make sense
func (r *Runner) Run(ctx context.Context) {
	// api applicationsv1alpha1.API
	// Just a placeholder chan to illustrate what we'll be doing
	// eventually this will be replaced with something from the api
	// ex, for app := range <- api.Watch() {

	for {
		select {
		case <-ctx.Done():
			return
		case app := <-r.appCh:
			if app.Namespace == "" {
				// api.UpdateStatus("failed")
				r.errCh <- errors.New("invalid app.yaml, missing namespace")
				continue
			}
			if app.Name == "" {
				// api.UpdateStatus("failed")
				r.errCh <- errors.New("invalid app.yaml, missing name")
				continue
			}

			filterOpts := scan.FilterOptions{
				KubectlExecutor: r.kubectlExecFn,
			}

			g := &Generator{
				Application:   *app,
				FilterOptions: filterOpts,
			}
			g.SetDefaultSelectors()

			var output bytes.Buffer
			if err := g.Execute(kio.ByteWriter{Writer: &output}); err != nil {
				r.errCh <- fmt.Errorf("%s: %w", "failed to generate experiment", err)
				continue
			}

			generatedApplicationBytes := output.Bytes()

			// TODO
			// During the 'preview' phase, we should probably only create the experiment ( no rbac,
			// configmap, secret, etc )
			// Once we're confirmed, we should do the rest
			// if userConfirmed {
			exp := &redskyv1beta1.Experiment{}
			if err := yaml.Unmarshal(generatedApplicationBytes, exp); err != nil {
				// api.UpdateStatus("failed")
				r.errCh <- fmt.Errorf("%s: %w", "invalid experiment generated", err)
				continue
			}

			var replicas int32
			replicas = 0
			if _, userConfirmed := app.Annotations[redskyappsv1alpha1.AnnotationUserConfirmed]; userConfirmed {
				replicas = 1
			}
			exp.Spec.Replicas = &replicas

			existingExperiment := &redskyv1beta1.Experiment{}
			switch r.client.Get(ctx, types.NamespacedName{Name: exp.Name, Namespace: exp.Namespace}, existingExperiment) {
			case nil:
				// Create additional RBAC ( primarily for setup task )
				serviceAccount := &corev1.ServiceAccount{}
				if err := yaml.Unmarshal(generatedApplicationBytes, serviceAccount); err != nil {
					r.errCh <- fmt.Errorf("%s: %w", "invalid service account", err)
					continue
				}
				if serviceAccount != nil {
					if err := r.client.Create(ctx, serviceAccount); err != nil {
						r.errCh <- fmt.Errorf("%s: %w", "failed to create service account", err)
						continue
					}
				}

				clusterRole := &rbacv1.ClusterRole{}
				if err := yaml.Unmarshal(generatedApplicationBytes, clusterRole); err != nil {
					r.errCh <- fmt.Errorf("%s: %w", "invalid cluster role", err)
					continue
				}
				if clusterRole != nil {
					if err := r.client.Create(ctx, clusterRole); err != nil {
						r.errCh <- fmt.Errorf("%s: %w", "failed to create service account", err)
						continue
					}
				}

				clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				if err := yaml.Unmarshal(generatedApplicationBytes, clusterRoleBinding); err != nil {
					r.errCh <- fmt.Errorf("%s: %w", "invalid cluster role binding", err)
					continue
				}
				if clusterRoleBinding != nil {
					if err := r.client.Create(ctx, clusterRoleBinding); err != nil {
						r.errCh <- fmt.Errorf("%s: %w", "failed to create cluster role binding", err)
						continue
					}
				}

				// Create configmap for load test
				configMap := &corev1.ConfigMap{}
				if err := yaml.Unmarshal(generatedApplicationBytes, configMap); err != nil {
					r.errCh <- fmt.Errorf("%s: %w", "invalid config map", err)
					continue
				}
				if configMap != nil {
					if err := r.client.Create(ctx, configMap); err != nil {
						r.errCh <- fmt.Errorf("%s: %w", "failed to create config map", err)
						continue
					}
				}

				// Update the experiment ( primarily to set replicas from 0 -> 1 )
				if err := r.client.Update(ctx, exp); err != nil {
					// api.UpdateStatus("failed")
					r.errCh <- fmt.Errorf("%s: %w", "unable to start experiment", err)
					continue
				}

			default:
				if err := r.client.Create(ctx, exp); err != nil {
					// api.UpdateStatus("failed")
					r.errCh <- fmt.Errorf("%s: %w", "unable to create experiment in cluster", err)
					continue
				}
			}

			// TODO
			// How should we handle the rejection of an application ( user wanted to make
			// changes, so we need to delete the old experiment )
			// Experiment already exists, so let's patch replicas only

			//}

			// } else {
			// can/should we use unstructured.Unstructured ?
			// or corev1.list
			// or should we iterate through each type and use the appropriate client
			/*
				js, err := yaml.YAMLToJSON(outputBytes)
				if err != nil {
					log.Println("failed to convert yaml to json")
					r.errCh <- err
				}

				ul := &unstructured.UnstructuredList{}
				if err := ul.UnmarshalJSON(js); err != nil {
					log.Println("cant unmarshal", err)
					r.errCh <- err
					continue
				}
				ul.SetGroupVersionKind(schema.FromAPIVersionAndKind("v1", "List"))

				fmt.Println(ul)

				if err := r.client.Create(ctx, ul); err != nil {
					log.Println("failed to create ul", err)
					r.errCh <- err
					continue
				}
			*/

			// }

			// log.Println("success")
			return
		}
	}
}

// filter returns a filter function to exctract a specified `kind` from the input.
/*
func manageReplicas(ok bool) kio.FilterFunc {
	return func(input []*kyaml.RNode) ([]*kyaml.RNode, error) {
		var output kio.ResourceNodeSlice
		for i := range input {
			m, err := input[i].GetMeta()
			if err != nil {
				return nil, err
			}

			if m.Kind != "Experiment" {
				continue
			}

			numReplicas := 0
			if ok {
				numReplicas = 1
			}

			input[i].Pipe(
				kyaml.LookupCreate(kyaml.ScalarNode, "spec"),
				kyaml.SetField("replicas", kyaml.NewScalarRNode(strconv.Itoa(numReplicas))),
			)
			// kyaml.LookupCreate
			// values := kyaml.NewMapRNode(&map[string]string{"replicas": strconv.Itoa(numReplicas)})
			// kyaml.SetField("spec", values)

			//if _, err = input[i].Pipe(kyaml.LookupCreate(input[i], "spec", "replicas", strconv.Itoa(numReplicas))); err != nil {
			//		return nil, err
			//}

			output = append(output, input[i])
		}
		return output, nil
	}
}
*/

func inClusterKubectl(_ *exec.Cmd) ([]byte, error) { return []byte{}, nil }
