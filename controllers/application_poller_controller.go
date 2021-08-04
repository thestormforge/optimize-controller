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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/go-logr/logr"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"
)

type Poller struct {
	client        client.Client
	log           logr.Logger
	apiClient     applications.API
	kubectlExecFn func(cmd *exec.Cmd) ([]byte, error)
}

func NewPoller(kclient client.Client, logger logr.Logger) (*Poller, error) {
	appAPI, err := server.NewApplicationAPI(context.Background(), "TODO - user agent")
	if err != nil {
		if authErr := errors.Unwrap(err); api.IsUnauthorized(authErr) {
			logger.Info(err.Error())
			return &Poller{}, nil
		}

		return nil, err
	}

	return &Poller{
		client:    kclient,
		apiClient: api,
		log:       logger,
	}, nil
}

func (p *Poller) Start(ch <-chan struct{}) error {
	if p.apiClient == nil {
		return nil
	}

	ctx := context.Background()

	query := applications.ActivityFeedQuery{}
	query.SetType(applications.TagScan, applications.TagRun)
	subscriber, err := p.apiClient.SubscribeActivity(ctx, query)
	if err != nil {
		p.log.Error(err, "unable to connect to application service")
		return nil
	}

	activityCh := make(chan applications.ActivityItem)
	go subscriber.Subscribe(ctx, activityCh)

	for {
		select {
		case <-ch:
			return nil
		case activity := <-activityCh:
			p.handleActivity(ctx, activity)
		}
	}
}

func (p *Poller) handleActivity(ctx context.Context, activity applications.ActivityItem) {
	// We always want to delete the activity after having received it
	defer func() {
		if err := p.apiClient.DeleteActivity(ctx, activity.URL); err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to delete activity", err), activity.URL)
		}
	}()

	// Ensure we actually have an action to perform
	if len(activity.Tags) != 1 {
		p.handleErrors(ctx, fmt.Errorf("%s %d", "invalid number of activity tags, expected 1 got", len(activity.Tags)), activity.URL)
		return
	}

	// Activity feed provides us with a scenario URL
	scenario, err := p.apiClient.GetScenario(ctx, activity.ExternalURL)
	if err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to get scenario", activity.ExternalURL, err), activity.URL)
		return
	}

	// Need to fetch top level application so we can get the resources
	applicationURL := scenario.Link(api.RelationUp)
	if applicationURL == "" {
		p.handleErrors(ctx, fmt.Errorf("no matching application URL for scenario"), activity.URL)
		return
	}

	templateURL := scenario.Link(api.RelationTemplate)
	if templateURL == "" {
		p.handleErrors(ctx, fmt.Errorf("no matching template URL for scenario"), activity.URL)
		return
	}

	apiApp, err := p.apiClient.GetApplication(ctx, applicationURL)
	if err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to get application", activity.URL, err), activity.URL)
		return
	}

	var assembledApp *optimizeappsv1alpha1.Application
	if assembledApp, err = server.APIApplicationToClusterApplication(apiApp, scenario); err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to assemble application", activity.URL, err), activity.URL)
		return
	}

	assembledBytes, err := p.generateApp(*assembledApp)
	if err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to generate application", activity.URL, err), activity.URL)
		return
	}

	exp := &optimizev1beta2.Experiment{}
	if err := yaml.Unmarshal(assembledBytes, exp); err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s: %w", "invalid experiment generated", err), activity.URL)
		return
	}

	switch activity.Tags[0] {
	case applications.TagScan:
		template, err := server.ClusterExperimentToAPITemplate(exp)
		if err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to convert experiment template", err), activity.URL)
			return
		}

		if err := p.apiClient.UpdateTemplate(ctx, templateURL, *template); err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to save experiment template in server", err), activity.URL)
			return
		}
	case applications.TagRun:
		// We wont compare existing scan with current scan
		// so we can preserve changes via UI

		// Get previous template
		previousTemplate, err := p.apiClient.GetTemplate(ctx, templateURL)
		if err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to get experiment template from server", err), activity.URL)
			return
		}

		// Overwrite current scan results with previous scan results
		if err = server.APITemplateToClusterExperiment(exp, &previousTemplate); err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to convert experiment template", err), activity.URL)
			return
		}

		// At this point the experiment should be good to create/deploy/run
		// so let's create all the resources and #profit

		// Create additional RBAC ( primarily for setup task )
		if err = p.createServiceAccount(ctx, assembledBytes); err != nil {
			p.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = p.createClusterRole(ctx, assembledBytes); err != nil {
			p.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = p.createClusterRoleBinding(ctx, assembledBytes); err != nil {
			p.handleErrors(ctx, err, activity.URL)
			return
		}

		// Create configmap for load test
		if err = p.createConfigMap(ctx, assembledBytes); err != nil {
			p.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = p.createExperiment(ctx, exp); err != nil {
			p.handleErrors(ctx, err, activity.URL)
			return
		}
	}

}

func (p *Poller) handleErrors(ctx context.Context, err error, activityURL string) {
	failure := applications.ActivityFailure{FailureMessage: err.Error()}

	if err := p.apiClient.PatchApplicationActivity(ctx, activityURL, failure); err != nil {
		p.log.Error(err, "unable to update application activity")
	}
}

func (p *Poller) generateApp(app optimizeappsv1alpha1.Application) ([]byte, error) {
	// Set defaults for application
	app.Default()

	g := &experiment.Generator{
		Application: app,
		FilterOptions: scan.FilterOptions{
			KubectlExecutor: p.kubectlExecFn,
		},
	}

	var output bytes.Buffer
	if err := g.Execute(kio.ByteWriter{Writer: &output}); err != nil {
		return nil, fmt.Errorf("%s: %w", "failed to generate experiment", err)
	}

	return output.Bytes(), nil

}

func (p *Poller) createServiceAccount(ctx context.Context, data []byte) error {
	serviceAccount := &corev1.ServiceAccount{}
	if err := yaml.Unmarshal(data, serviceAccount); err != nil {
		return fmt.Errorf("%s: %w", "invalid service account", err)
	}

	// Only create the service account if it does not exist
	existingServiceAccount := &corev1.ServiceAccount{}
	if err := p.client.Get(ctx, types.NamespacedName{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, existingServiceAccount); err != nil {
		if err := p.client.Create(ctx, serviceAccount); err != nil {
			return fmt.Errorf("%s: %w", "failed to create service account", err)
		}
	}

	return nil
}

func (p *Poller) createClusterRole(ctx context.Context, data []byte) error {
	clusterRole := &rbacv1.ClusterRole{}
	if err := yaml.Unmarshal(data, clusterRole); err != nil {
		return fmt.Errorf("%s: %w", "invalid cluster role", err)
	}

	// Only create the service account if it does not exist
	existingClusterRole := &rbacv1.ClusterRole{}
	if err := p.client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, existingClusterRole); err != nil {
		if err := p.client.Create(ctx, clusterRole); err != nil {
			return fmt.Errorf("%s: %w", "failed to create clusterRole", err)
		}
	}

	return nil
}

func (p *Poller) createClusterRoleBinding(ctx context.Context, data []byte) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := yaml.Unmarshal(data, clusterRoleBinding); err != nil {
		return fmt.Errorf("%s: %w", "invalid cluster role binding", err)
	}

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := p.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, existingClusterRoleBinding); err != nil {
		if err := p.client.Create(ctx, clusterRoleBinding); err != nil {
			return fmt.Errorf("%s: %w", "failed to create cluster role binding", err)
		}
	}

	return nil
}

func (p *Poller) createConfigMap(ctx context.Context, data []byte) error {
	configMap := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(data, configMap); err != nil {
		return fmt.Errorf("%s: %w", "invalid config map", err)
	}

	existingConfigMap := &corev1.ConfigMap{}
	if err := p.client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap); err != nil {
		if err := p.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("%s: %w", "failed to create config map", err)
		}
	} else {
		if err := p.client.Update(ctx, configMap); err != nil {
			return fmt.Errorf("%s: %w", "failed to update config map", err)
		}
	}

	return nil
}

func (p *Poller) createExperiment(ctx context.Context, exp *optimizev1beta2.Experiment) error {
	existingExperiment := &optimizev1beta2.Experiment{}
	if err := p.client.Get(ctx, types.NamespacedName{Name: exp.Name, Namespace: exp.Namespace}, existingExperiment); err != nil {
		if err := p.client.Create(ctx, exp); err != nil {
			return fmt.Errorf("%s: %w", "unable to create experiment in cluster", err)
		}
	} else {
		// Update the experiment ( primarily to set replicas from 0 -> 1 )
		if err := p.client.Update(ctx, exp); err != nil {
			return fmt.Errorf("%s: %w", "unable to start experiment", err)
		}
	}

	return nil
}
