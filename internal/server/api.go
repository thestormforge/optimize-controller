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

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/thestormforge/optimize-controller/v2/internal/version"
	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
)

const audience = "https://api.carbonrelay.io/v1/"

func NewExperimentAPI(ctx context.Context, uaComment string) (experimentsv1alpha1.API, error) {
	client, err := newClientFromConfig(ctx, uaComment)
	if err != nil {
		return nil, err
	}

	expAPI := experimentsv1alpha1.NewAPI(client)

	// An unauthorized error means we will never be able to connect without changing the credentials and restarting
	if _, err := expAPI.CheckEndpoint(ctx); api.IsUnauthorized(err) {
		return nil, fmt.Errorf("experiments API is unavailable, skipping setup: %s", err.Error())
	}

	return expAPI, nil
}

func NewApplicationAPI(ctx context.Context, uaComment string) (applications.API, error) {
	client, err := newClientFromConfig(ctx, uaComment)
	if err != nil {
		return nil, err
	}

	appAPI := applications.NewAPI(client)

	if _, err := appAPI.CheckEndpoint(ctx); api.IsUnauthorized(err) {
		return nil, fmt.Errorf("applications API is unavailable, skipping setup: %s", err.Error())
	}

	return appAPI, nil
}

func newClientFromConfig(ctx context.Context, uaComment string) (api.Client, error) {
	// Load the configuration
	cfg := &config.OptimizeConfig{}
	cfg.AuthorizationParameters = map[string][]string{
		"audience": {audience},
	}

	if err := cfg.Load(); err != nil {
		return nil, err
	}

	// Get the Experiments API endpoint from the configuration
	// NOTE: The current version of the configuration has an explicit configuration for the
	// experiments endpoint which would duplicate the "/experiments/" path segment
	srv, err := config.CurrentServer(cfg.Reader())
	if err != nil {
		return nil, err
	}

	address := strings.TrimSuffix(srv.API.ExperimentsEndpoint, "/v1/experiments/")

	rt, err := cfg.Authorize(ctx, version.UserAgent("optimize-controller", uaComment, nil))
	if err != nil {
		return nil, err
	}

	// Create a new API client
	c, err := api.NewClient(address, rt)
	if err != nil {
		return nil, err
	}

	return c, nil
}
