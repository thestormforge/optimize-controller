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
	"fmt"
	"os/exec"

	"github.com/go-logr/logr"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	redskyv1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Runner struct {
	client        client.Client
	apiClient     applications.API
	log           logr.Logger
	kubectlExecFn func(cmd *exec.Cmd) ([]byte, error)
}

func New(kclient client.Client, logger logr.Logger) (*Runner, error) {
	api, err := server.NewApplicationAPI(context.Background(), "TODO - user agent")
	if err != nil {
		return nil, err
	}

	return &Runner{
		client:    kclient,
		apiClient: api,
		log:       logger,
	}, nil
}

// This doesnt necessarily need to live here, but seemed to make sense
func (r *Runner) Run(ctx context.Context) {
	query := applications.ActivityFeedQuery{}
	query.SetType(applications.TagScan, applications.TagRun)
	subscriber, err := r.apiClient.SubscribeActivity(ctx, query)
	if err != nil {
		r.log.Error(err, "unable to connect to application service")
		return
	}

	activityCh := make(chan applications.ActivityItem)
	go subscriber.Subscribe(ctx, activityCh)

	for {
		select {
		case <-ctx.Done():
			return
		case activity := <-activityCh:
			r.handleActivity(ctx, activity)
		}
	}
}

func (r *Runner) handleActivity(ctx context.Context, activity applications.ActivityItem) {
	// We always want to delete the activity after having received it
	defer func() {
		if err := r.apiClient.DeleteActivity(ctx, activity.URL); err != nil {
			r.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to delete activity", err), activity.URL)
		}
	}()

	// Ensure we actually have an action to perform
	if len(activity.Tags) != 1 {
		r.handleErrors(ctx, fmt.Errorf("%s %d", "invalid number of activity tags, expected 1 got", len(activity.Tags)), activity.URL)
		return
	}

	// Activity feed provides us with a scenario URL
	scenario, err := r.apiClient.GetScenario(ctx, activity.ExternalURL)
	if err != nil {
		r.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to get scenario", activity.ExternalURL, err), activity.URL)
		return
	}

	// Need to fetch top level application so we can get the resources
	applicationURL := scenario.Link(api.RelationUp)
	if applicationURL == "" {
		r.handleErrors(ctx, fmt.Errorf("no matching application URL for scenario"), activity.URL)
		return
	}

	templateURL := scenario.Link(api.RelationTemplate)
	if templateURL == "" {
		r.handleErrors(ctx, fmt.Errorf("no matching template URL for scenario"), activity.URL)
		return
	}

	apiApp, err := r.apiClient.GetApplication(ctx, applicationURL)
	if err != nil {
		r.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to get application", activity.URL, err), activity.URL)
		return
	}

	var assembledApp *redskyappsv1alpha1.Application
	if assembledApp, err = r.scan(apiApp, scenario); err != nil {
		r.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to assemble application", activity.URL, err), activity.URL)
		return
	}

	assembledBytes, err := r.generateApp(*assembledApp)
	if err != nil {
		r.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to generate application", activity.URL, err), activity.URL)
		return
	}

	exp := &redskyv1beta2.Experiment{}
	if err := yaml.Unmarshal(assembledBytes, exp); err != nil {
		r.handleErrors(ctx, fmt.Errorf("%s: %w", "invalid experiment generated", err), activity.URL)
		return
	}

	switch activity.Tags[0] {
	case applications.TagScan:
		template, err := server.ClusterExperimentToAPITemplate(exp)
		if err != nil {
			r.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to convert experiment template", err), activity.URL)
			return
		}

		if err := r.apiClient.UpdateTemplate(ctx, templateURL, *template); err != nil {
			r.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to save experiment template in server", err), activity.URL)
			return
		}
	case applications.TagRun:
		// We wont compare existing scan with current scan
		// so we can preserve changes via UI

		// Get previous template
		previousTemplate, err := r.apiClient.GetTemplate(ctx, templateURL)
		if err != nil {
			r.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to get experiment template from server", err), activity.URL)
			return
		}

		// Overwrite current scan results with previous scan results
		if err = server.APITemplateToClusterExperiment(exp, &previousTemplate); err != nil {
			r.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to convert experiment template", err), activity.URL)
			return
		}

		// At this point the experiment should be good to create/deploy/run
		// so let's create all the resources and #profit

		// Create additional RBAC ( primarily for setup task )
		if err = r.createServiceAccount(ctx, assembledBytes); err != nil {
			r.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = r.createClusterRole(ctx, assembledBytes); err != nil {
			r.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = r.createClusterRoleBinding(ctx, assembledBytes); err != nil {
			r.handleErrors(ctx, err, activity.URL)
			return
		}

		// Create configmap for load test
		if err = r.createConfigMap(ctx, assembledBytes); err != nil {
			r.handleErrors(ctx, err, activity.URL)
			return
		}

		if err = r.createExperiment(ctx, exp); err != nil {
			r.handleErrors(ctx, err, activity.URL)
			return
		}
	}

}

func (r *Runner) handleErrors(ctx context.Context, err error, activityURL string) {
	failure := applications.ActivityFailure{FailureMessage: err.Error()}

	if err := r.apiClient.PatchApplicationActivity(ctx, activityURL, failure); err != nil {
		r.log.Error(err, "unable to update application activity")
	}
}

func (r *Runner) createServiceAccount(ctx context.Context, data []byte) error {
	serviceAccount := &corev1.ServiceAccount{}
	if err := yaml.Unmarshal(data, serviceAccount); err != nil {
		return fmt.Errorf("%s: %w", "invalid service account", err)
	}

	// Only create the service account if it does not exist
	existingServiceAccount := &corev1.ServiceAccount{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, existingServiceAccount); err != nil {
		if err := r.client.Create(ctx, serviceAccount); err != nil {
			return fmt.Errorf("%s: %w", "failed to create service account", err)
		}
	}

	return nil
}

func (r *Runner) createClusterRole(ctx context.Context, data []byte) error {
	clusterRole := &rbacv1.ClusterRole{}
	if err := yaml.Unmarshal(data, clusterRole); err != nil {
		return fmt.Errorf("%s: %w", "invalid cluster role", err)
	}

	// Only create the service account if it does not exist
	existingClusterRole := &rbacv1.ClusterRole{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, existingClusterRole); err != nil {
		if err := r.client.Create(ctx, clusterRole); err != nil {
			return fmt.Errorf("%s: %w", "failed to create clusterRole", err)
		}
	}

	return nil
}

func (r *Runner) createClusterRoleBinding(ctx context.Context, data []byte) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := yaml.Unmarshal(data, clusterRoleBinding); err != nil {
		return fmt.Errorf("%s: %w", "invalid cluster role binding", err)
	}

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, existingClusterRoleBinding); err != nil {
		if err := r.client.Create(ctx, clusterRoleBinding); err != nil {
			return fmt.Errorf("%s: %w", "failed to create cluster role binding", err)
		}
	}

	return nil
}

func (r *Runner) createConfigMap(ctx context.Context, data []byte) error {
	configMap := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(data, configMap); err != nil {
		return fmt.Errorf("%s: %w", "invalid config map", err)
	}

	existingConfigMap := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap); err != nil {
		if err := r.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("%s: %w", "failed to create config map", err)
		}
	} else {
		if err := r.client.Update(ctx, configMap); err != nil {
			return fmt.Errorf("%s: %w", "failed to update config map", err)
		}
	}

	return nil
}

func (r *Runner) createExperiment(ctx context.Context, exp *redskyv1beta2.Experiment) error {
	existingExperiment := &redskyv1beta2.Experiment{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: exp.Name, Namespace: exp.Namespace}, existingExperiment); err != nil {
		if err := r.client.Create(ctx, exp); err != nil {
			return fmt.Errorf("%s: %w", "unable to create experiment in cluster", err)
		}
	} else {
		// Update the experiment ( primarily to set replicas from 0 -> 1 )
		if err := r.client.Update(ctx, exp); err != nil {
			return fmt.Errorf("%s: %w", "unable to start experiment", err)
		}
	}

	return nil
}
