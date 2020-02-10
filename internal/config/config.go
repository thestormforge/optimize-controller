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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/redskyops/k8s-experiment/internal/oauth2/authorizationcode"
	"github.com/redskyops/k8s-experiment/internal/oauth2/devicecode"
	"github.com/redskyops/k8s-experiment/internal/oauth2/registration"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Loader is used to initially populate a Red Sky configuration
type Loader func(cfg *RedSkyConfig) error

// Change is used to apply a configuration change that should be persisted
type Change func(cfg *Config) error

// Endpoints exposes the Red Sky API server endpoint locations as a mapping of prefixes to base URLs
type Endpoints map[string]*url.URL

// RedSkyConfig is the structure used to manage configuration data
type RedSkyConfig struct {
	// Filename is the path to the configuration file; if left blank, it will be populated using XDG base directory conventions on the next Load
	Filename string

	data        Config
	unpersisted []Change
}

// MarshalJSON ensures only the configuration data is marshalled
func (rsc *RedSkyConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(rsc.data)
}

// Load will populate the client configuration
func (rsc *RedSkyConfig) Load(extra ...Loader) error {
	var loaders []Loader
	loaders = append(loaders, fileLoader, migrationLoader)
	loaders = append(loaders, extra...)
	loaders = append(loaders, envLoader, defaultLoader)
	for i := range loaders {
		if err := loaders[i](rsc); err != nil {
			return err
		}
	}
	return nil
}

// Update will make a change to the configuration data that should be persisted on the next call to Write
func (rsc *RedSkyConfig) Update(change Change) error {
	if err := change(&rsc.data); err != nil {
		return err
	}
	rsc.unpersisted = append(rsc.unpersisted, change)
	return nil
}

// Write all unpersisted changes to disk
func (rsc *RedSkyConfig) Write() error {
	if rsc.Filename == "" || len(rsc.unpersisted) == 0 {
		return nil
	}

	f := file{}
	if err := f.read(rsc.Filename); err != nil {
		return err
	}

	for i := range rsc.unpersisted {
		if err := rsc.unpersisted[i](&f.data); err != nil {
			return err
		}
	}

	if err := f.write(rsc.Filename); err != nil {
		return err
	}

	rsc.unpersisted = nil
	return nil
}

// SystemNamespace returns the namespace where the Red Sky controller is/should be installed
func (rsc *RedSkyConfig) SystemNamespace() (string, error) {
	_, _, _, ctrl, err := contextConfig(&rsc.data)
	if err != nil {
		return "", err
	}
	return ctrl.Namespace, nil
}

// EndpointLocations returns a resolver that can generate fully qualified endpoint URLs
func (rsc *RedSkyConfig) Endpoints() (Endpoints, error) {
	srv, _, _, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}

	add := func(ep Endpoints, prefix, endpoint string) error {
		u, err := url.Parse(endpoint)
		if err != nil {
			return err
		}
		u.Path = strings.TrimSuffix(u.Path, "/") + "/"
		ep[prefix] = u
		return nil
	}

	ep := Endpoints(make(map[string]*url.URL, 2))
	if err := add(ep, "/experiments/", srv.RedSky.ExperimentsEndpoint); err != nil {
		return nil, err
	}
	if err := add(ep, "/accounts/", srv.RedSky.AccountsEndpoint); err != nil {
		return nil, err
	}
	return ep, nil
}

// Resolve returns the fully qualified URL of the specified endpoint
func (ep Endpoints) Resolve(endpoint string) *url.URL {
	for k, v := range ep {
		if strings.HasPrefix(endpoint, k) {
			u := *v
			u.Path = u.Path + strings.TrimPrefix(endpoint, k)
			return &u
		}
	}
	return nil
}

// Kubectl returns an executable command for running kubectl
func (rsc *RedSkyConfig) Kubectl(arg ...string) (*exec.Cmd, error) {
	_, _, cstr, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}

	if cstr.Context != "" {
		arg = append([]string{"--context", cstr.Context}, arg...)
	}

	return exec.Command(cstr.Bin, arg...), nil
}

// RegisterClient performs dynamic client registration
func (rsc *RedSkyConfig) RegisterClient(ctx context.Context, client *registration.ClientMetadata) (*registration.ClientInformationResponse, error) {
	// We can't use the initial token because we don't know if we have a valid token, instead we need to authorize the context client
	src, err := rsc.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	if src != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, oauth2.NewClient(ctx, src))
	}

	// Get the current server configuration for the registration endpoint address
	srv, _, _, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}
	c := registration.Config{
		RegistrationURL: srv.Authorization.RegistrationEndpoint,
	}
	return c.Register(ctx, client)
}

// NewAuthorization creates a new authorization code flow with PKCE using the current context
func (rsc *RedSkyConfig) NewAuthorization() (*authorizationcode.Config, error) {
	srv, _, _, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}

	c, err := authorizationcode.NewAuthorizationCodeFlowWithPKCE()
	if err != nil {
		return nil, err
	}

	c.Endpoint.AuthURL = srv.Authorization.AuthorizationEndpoint
	c.Endpoint.TokenURL = srv.Authorization.TokenEndpoint
	c.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	c.EndpointParams = map[string][]string{"audience": {srv.Identifier}}
	return c, nil
}

// NewDeviceAuthorization creates a new device authorization flow using the current context
func (rsc *RedSkyConfig) NewDeviceAuthorization() (*devicecode.Config, error) {
	srv, _, _, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}

	return &devicecode.Config{
		Config: clientcredentials.Config{
			TokenURL:  srv.Authorization.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		DeviceAuthorizationURL: srv.Authorization.DeviceAuthorizationEndpoint,
		EndpointParams:         map[string][]string{"audience": {srv.Identifier}},
	}, nil
}

// Authorize configures the supplied transport
func (rsc *RedSkyConfig) Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error) {
	// Get the token source and use it to wrap the transport
	src, err := rsc.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	if src != nil {
		return &oauth2.Transport{Source: src, Base: transport}, nil
	}
	return transport, nil
}

func (rsc *RedSkyConfig) tokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	// TODO We could make RedSkyConfig implement the TokenSource interface, but we need a way to handle the context
	srv, az, _, _, err := contextConfig(&rsc.data)
	if err != nil {
		return nil, err
	}

	if az.Credential.ClientCredential != nil {
		cc := clientcredentials.Config{
			ClientID:     az.Credential.ClientID,
			ClientSecret: az.Credential.ClientSecret,
			TokenURL:     srv.Authorization.TokenEndpoint,
			AuthStyle:    oauth2.AuthStyleInParams,
		}
		return cc.TokenSource(ctx), nil
	}

	if az.Credential.TokenCredential != nil {
		c := &oauth2.Config{
			Endpoint: oauth2.Endpoint{
				AuthURL:   srv.Authorization.AuthorizationEndpoint,
				TokenURL:  srv.Authorization.TokenEndpoint,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		}
		t := &oauth2.Token{
			AccessToken:  az.Credential.AccessToken,
			TokenType:    az.Credential.TokenType,
			RefreshToken: az.Credential.RefreshToken,
			Expiry:       az.Credential.Expiry,
		}
		return c.TokenSource(ctx, t), nil
	}

	return nil, nil
}

// Merge combines the supplied data with what is already present in this client configuration; unlike Update, changes
// will not be persisted on the next write
func (rsc *RedSkyConfig) Merge(data *Config) {
	mergeServers(&rsc.data, data.Servers)
	mergeAuthorizations(&rsc.data, data.Authorizations)
	mergeClusters(&rsc.data, data.Clusters)
	mergeControllers(&rsc.data, data.Controllers)
	mergeContexts(&rsc.data, data.Contexts)
	mergeString(&rsc.data.CurrentContext, data.CurrentContext)
}

// Minify returns a copy of the configuration data including only objects from the current context
func (rsc *RedSkyConfig) Minify() *Config {
	return minifyContext(&rsc.data, rsc.data.CurrentContext)
}

// contextConfig returns all of the configurations objects for the current context
//
// IMPORTANT: This can never be used in the implementation of a Loader because it may fail
func contextConfig(data *Config) (*Server, *Authorization, *Cluster, *Controller, error) {
	ctx := findContext(data.Contexts, data.CurrentContext)
	if ctx == nil {
		return nil, nil, nil, nil, fmt.Errorf("could not find context (%s)", data.CurrentContext)
	}

	srv := findServer(data.Servers, ctx.Server)
	if srv == nil {
		return srv, nil, nil, nil, fmt.Errorf("cound not find server (%s)", ctx.Server)
	}

	az := findAuthorization(data.Authorizations, ctx.Authorization)
	if az == nil {
		return srv, az, nil, nil, fmt.Errorf("could not find authorization (%s)", ctx.Authorization)
	}

	cstr := findCluster(data.Clusters, ctx.Cluster)
	if cstr == nil {
		return srv, az, cstr, nil, fmt.Errorf("could not find cluster (%s)", ctx.Cluster)
	}

	ctrl := findController(data.Controllers, cstr.Controller)
	if ctrl == nil {
		return srv, az, cstr, ctrl, fmt.Errorf("could not find controller (%s)", cstr.Controller)
	}

	return srv, az, cstr, ctrl, nil
}
