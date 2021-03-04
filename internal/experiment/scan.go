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
	"fmt"

	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/v2/api/v1beta1"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"
)

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

func (r *Runner) scan(app applications.Application) (*redskyappsv1alpha1.Application, error) {
	if app.Name == "" {
		return nil, fmt.Errorf("invalid application, missing name")
	}

	if len(app.Resources) == 0 {
		return nil, fmt.Errorf("invalid application, no resources specified")
	}

	// TODO this might(?) belong in internal/server
	// Construct a controller representation of an application from the api definition
	baseApp := &redskyappsv1alpha1.Application{}

	for _, resource := range app.Resources {
		if app.Namespace == "" {
			return nil, fmt.Errorf("invalid app.yaml, resource is missing namespace")
		}

		// TODO find out what this conversion looks like
		res := konjure.Resource{}

		baseApp.Resources = append(baseApp.Resources, res)
	}

	// TODO need scenarios
	// TODO need the rest of the application spec

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
}

func (r *Runner) scanResources(app applications.Application) (*redskyappsv1alpha1.Resources, error) {}
func (r *Runner) scanScenarios(app applications.Application) (*redskyappsv1alpha1.Scenarios, error) {}
