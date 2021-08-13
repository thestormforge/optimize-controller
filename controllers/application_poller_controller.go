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
	"errors"
	"fmt"
	"os/exec"

	"github.com/go-logr/logr"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Poller struct {
	client        client.Client
	log           logr.Logger
	apiClient     applications.API
	kubectlExecFn func(cmd *exec.Cmd) ([]byte, error)
}

func NewPoller(kclient client.Client, logger logr.Logger) (*Poller, error) {
	appAPI, err := server.NewApplicationAPI(context.Background(), version.GetInfo().String())
	if err != nil {
		if authErr := errors.Unwrap(err); api.IsUnauthorized(authErr) {
			logger.Info(err.Error())
			return &Poller{log: logger}, nil
		}

		return &Poller{log: logger}, err
	}

	return &Poller{
		client:    kclient,
		apiClient: appAPI,
		log:       logger,
	}, nil
}

func (p *Poller) Start(ch <-chan struct{}) error {
	if p.apiClient == nil {
		<-ch
		return nil
	}
	p.log.Info("Starting application poller")

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
			// We'll skip any janky activities that may come through
			// TODO we should figure out the why for this
			if activity.ID == "" {
				continue
			}

			p.handleActivity(ctx, activity)
		}
	}
}

// handleActivity performs the task required for each activity.
// When an ActivityItem is tagged with scan, the generation workflow is used to generate an experiment and the result
// is converted into an api.Template consisting of parameters and metrics.
// When an ActivityItem is tagged with run, the previous scanned template results are merged with
// the results of an experiment generation workflow. Following this, the generated resources are applied/created
// in the cluster.
// note, rbac defined in cli/internal/commands/grant_permissions/generator
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

	p.log.Info("starting activity task", "task", activity.Tags[0], "activity", activity.URL)

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

	generatedResources, err := p.generateApp(*assembledApp)
	if err != nil {
		p.handleErrors(ctx, fmt.Errorf("%s (%s): %w", "failed to generate application", activity.URL, err), activity.URL)
		return
	}

	var exp *optimizev1beta2.Experiment
	for i := range generatedResources {
		if expObj, ok := generatedResources[i].(*optimizev1beta2.Experiment); ok {
			exp = expObj
			break
		}
	}

	if exp == nil {
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

		p.log.Info("successfully completed resource scan", "activity", activity.URL)
	case applications.TagRun:
		// We wont compare existing scan with current scan
		// so we can preserve changes via UI

		// Get previous template
		previousTemplate, err := p.apiClient.GetTemplate(ctx, templateURL)
		if err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to get experiment template from server, a 'scan' task must be completed first", err), activity.URL)
			return
		}

		// Overwrite current scan results with previous scan results
		if err = server.APITemplateToClusterExperiment(exp, &previousTemplate); err != nil {
			p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to convert experiment template", err), activity.URL)
			return
		}

		// At this point the experiment should be good to create/deploy/run
		// so let's create all the resources and #profit

		// TODO
		// try to clean up on failure ( might be a simple / blind p.client.Delete(ctx,generatedResources[i])
		for i := range generatedResources {
			// TODO generatedResource ( experiment ) does not contain the namespace
			// not sure why yet
			objKey, err := client.ObjectKeyFromObject(generatedResources[i])
			if err != nil {
				p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to get object key", err), activity.URL)
				return
			}

			holder := &unstructured.Unstructured{}
			holder.SetGroupVersionKind(generatedResources[i].GetObjectKind().GroupVersionKind())
			err = p.client.Get(ctx, objKey, holder)
			switch {
			case apierrors.IsNotFound(err):
				if err := p.client.Create(ctx, generatedResources[i]); err != nil {
					p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to create object", err), activity.URL)
					return
				}
			case err == nil:
				// TODO This might need to be a patch instead of an update
				// {"error":"failed to update object: experiments.optimize.stormforge.io \"01fcj078y60j74m11tnm7ga0yw-2a1d7d90\" is invalid: metadata.resourceVersion: Invalid value: 0x0: must be specified for an update"}
				if err := p.client.Update(ctx, generatedResources[i]); err != nil {
					p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to update object", err), activity.URL)
					return
				}
			default:
				// Assume this should be a hard error
				p.handleErrors(ctx, fmt.Errorf("%s: %w", "failed to get object", err), activity.URL)
				return
			}
		}

		p.log.Info("successfully created in cluster resources", "activity", activity.URL)
	}
}

func (p *Poller) handleErrors(ctx context.Context, err error, activityURL string) {
	failure := applications.ActivityFailure{FailureMessage: err.Error()}

	if err := p.apiClient.PatchApplicationActivity(ctx, activityURL, failure); err != nil {
		p.log.Error(err, "unable to update application activity")
	}
}

func (p *Poller) generateApp(app optimizeappsv1alpha1.Application) ([]runtime.Object, error) {
	// Set defaults for application
	app.Default()

	// TODO hack from above issue ( missing namespace )
	if app.Namespace == "" {
		app.Namespace = "default"
	}

	g := &experiment.Generator{
		Application: app,
		FilterOptions: scan.FilterOptions{
			KubectlExecutor: p.kubectlExecFn,
		},
	}

	objList := sfio.ObjectList{}
	if err := g.Execute(&objList); err != nil {
		return nil, fmt.Errorf("%s: %w", "failed to generate experiment", err)
	}

	runtimeObjs := make([]runtime.Object, 0, len(objList.Items))
	for i := range objList.Items {
		runtimeObjs = append(runtimeObjs, objList.Items[i].Object)
	}

	return runtimeObjs, nil
}
