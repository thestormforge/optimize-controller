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

	// Should we handle errors here
	// as in something like
	// go func() for err := <- errCh { api.UpdateStatus("failed", err)  }

	return &Runner{
		client: kclient,
		appCh:  appCh,
		errCh:  errCh,
		// Dont think I need this anymore with the `scan.MiniKubectl`
		//kubectlExecFn: inClusterKubectl,
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

			g := &Generator{
				Application: *app,
			}

			// Exposed for testing so we can pass through
			// fake kubectl output
			if r.kubectlExecFn != nil {
				g.FilterOptions = scan.FilterOptions{KubectlExecutor: r.kubectlExecFn}
			}

			var output bytes.Buffer
			if err := g.Execute(kio.ByteWriter{Writer: &output}); err != nil {
				r.errCh <- fmt.Errorf("%s: %w", "failed to generate experiment", err)
				continue
			}

			generatedApplicationBytes := output.Bytes()

			exp := &redskyv1beta1.Experiment{}
			if err := yaml.Unmarshal(generatedApplicationBytes, exp); err != nil {
				// api.UpdateStatus("failed")
				r.errCh <- fmt.Errorf("%s: %w", "invalid experiment generated", err)
				continue
			}

			// TODO this will get replaced with a api call to get the number of replicas
			// this will denote that we are OK to run the experiment
			var replicas int32
			replicas = 0
			if _, userConfirmed := app.Annotations[redskyappsv1alpha1.AnnotationUserConfirmed]; userConfirmed {
				replicas = 1
			}
			exp.Spec.Replicas = &replicas

			// TODO get experiment URL from annotation on application
			// and set it in the experiment annotations

			if exp.Spec.Replicas != nil && *exp.Spec.Replicas > 0 {
				// Create additional RBAC ( primarily for setup task )
				r.createServiceAccount(ctx, generatedApplicationBytes)

				r.createClusterRole(ctx, generatedApplicationBytes)

				r.createClusterRoleBinding(ctx, generatedApplicationBytes)

				// Create configmap for load test
				r.createConfigMap(ctx, generatedApplicationBytes)

				// TODO do we need to handle secrets here as well ( ex, SF JWT )
			}

			r.createExperiment(ctx, exp)
		}
	}
}

func (r *Runner) createServiceAccount(ctx context.Context, data []byte) {
	serviceAccount := &corev1.ServiceAccount{}
	if err := yaml.Unmarshal(data, serviceAccount); err != nil {
		r.errCh <- fmt.Errorf("%s: %w", "invalid service account", err)
		return
	}

	// Only create the service account if it does not exist
	existingServiceAccount := &corev1.ServiceAccount{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, existingServiceAccount); err != nil {
		if err := r.client.Create(ctx, serviceAccount); err != nil {
			r.errCh <- fmt.Errorf("%s: %w", "failed to create service account", err)
		}
	}
}

func (r *Runner) createClusterRole(ctx context.Context, data []byte) {
	clusterRole := &rbacv1.ClusterRole{}
	if err := yaml.Unmarshal(data, clusterRole); err != nil {
		r.errCh <- fmt.Errorf("%s: %w", "invalid cluster role", err)
		return
	}

	// Only create the service account if it does not exist
	existingClusterRole := &rbacv1.ClusterRole{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, existingClusterRole); err != nil {
		if err := r.client.Create(ctx, clusterRole); err != nil {
			r.errCh <- fmt.Errorf("%s: %w", "failed to create clusterRole", err)
		}
	}
}

func (r *Runner) createClusterRoleBinding(ctx context.Context, data []byte) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := yaml.Unmarshal(data, clusterRoleBinding); err != nil {
		r.errCh <- fmt.Errorf("%s: %w", "invalid cluster role binding", err)
		return
	}

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, existingClusterRoleBinding); err != nil {
		if err := r.client.Create(ctx, clusterRoleBinding); err != nil {
			r.errCh <- fmt.Errorf("%s: %w", "failed to create cluster role binding", err)
		}
	}
}

func (r *Runner) createConfigMap(ctx context.Context, data []byte) {
	configMap := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(data, configMap); err != nil {
		r.errCh <- fmt.Errorf("%s: %w", "invalid config map", err)
		return
	}

	existingConfigMap := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap); err != nil {
		if err := r.client.Create(ctx, configMap); err != nil {
			r.errCh <- fmt.Errorf("%s: %w", "failed to create config map", err)
		}
	} else {
		if err := r.client.Update(ctx, configMap); err != nil {
			r.errCh <- fmt.Errorf("%s: %w", "failed to update config map", err)
		}
	}
}

func (r *Runner) createExperiment(ctx context.Context, exp *redskyv1beta1.Experiment) {
	existingExperiment := &redskyv1beta1.Experiment{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: exp.Name, Namespace: exp.Namespace}, existingExperiment); err != nil {
		if err := r.client.Create(ctx, exp); err != nil {
			// api.UpdateStatus("failed")
			r.errCh <- fmt.Errorf("%s: %w", "unable to create experiment in cluster", err)
		}
	} else {
		// Update the experiment ( primarily to set replicas from 0 -> 1 )
		if err := r.client.Update(ctx, exp); err != nil {
			// api.UpdateStatus("failed")
			r.errCh <- fmt.Errorf("%s: %w", "unable to start experiment", err)
		}
	}
}
