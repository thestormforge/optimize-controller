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
	"sigs.k8s.io/yaml"
)

// Loader is used to initially populate a client configuration
type Loader func(cfg *ClientConfig) error

// Change is used to apply a configuration change that should be persisted
type Change func(cfg *Config) error

// ClientConfig is the structure used to manage configuration data
type ClientConfig struct {
	// Filename is the path to the configuration file; if left blank, it will be populated using XDG base directory conventions on the next Load
	Filename string

	data        Config
	unpersisted []Change
}

// Load will populate the client configuration
func (cc *ClientConfig) Load(extra ...Loader) error {
	var loaders []Loader
	loaders = append(loaders, fileLoader, migrationLoader)
	loaders = append(loaders, extra...)
	loaders = append(loaders, defaultLoader, envLoader)
	for i := range loaders {
		if err := loaders[i](cc); err != nil {
			return err
		}
	}
	return nil
}

// Update will make a change to the configuration data that should be persisted on the next call to Write
func (cc *ClientConfig) Update(change Change) error {
	if err := change(&cc.data); err != nil {
		return err
	}
	cc.unpersisted = append(cc.unpersisted, change)
	return nil
}

// Write all unpersisted changes to disk
func (cc *ClientConfig) Write() error {
	if cc.Filename == "" || len(cc.unpersisted) == 0 {
		return nil
	}

	f := file{}
	if err := f.read(cc.Filename); err != nil {
		return err
	}

	for i := range cc.unpersisted {
		if err := cc.unpersisted[i](&f.data); err != nil {
			return err
		}
	}

	if err := f.write(cc.Filename); err != nil {
		return err
	}

	cc.unpersisted = nil
	return nil
}

// Marshal will write the data out
func (cc *ClientConfig) Marshal() ([]byte, error) {
	return yaml.Marshal(cc.data)
}

// SystemNamespace returns the namespace where the Red Sky controller is/should be installed
func (cc *ClientConfig) SystemNamespace() (string, error) {
	_, _, _, ctrl, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return "", err
	}
	return ctrl.Namespace, nil
}

// ExperimentsURL returns the path to the experiments API endpoint
func (cc *ClientConfig) ExperimentsURL(p string) (*url.URL, error) {
	svr, _, _, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(svr.RedSky.ExperimentsEndpoint)
	if err != nil {
		return nil, err
	}

	// Path notes:
	// 1. We can't use path.Join because it will strip trailing slashes
	// 2. We don't know if the configured path has a "/"
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(p, "/experiments/")

	return u, nil
}

// Kubectl returns an executable command for running kubectl
func (cc *ClientConfig) Kubectl(arg ...string) (*exec.Cmd, error) {
	_, _, cstr, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return nil, err
	}

	if cstr.Context != "" {
		arg = append([]string{"--context", cstr.Context}, arg...)
	}

	return exec.Command(cstr.Bin, arg...), nil
}

// RegisterClient performs dynamic client registration
func (cc *ClientConfig) RegisterClient(ctx context.Context, client *registration.ClientMetadata) (*registration.ClientInformationResponse, error) {
	// We can't use the initial token because we don't know if we have a valid token, instead we need to authorize the context client
	src, err := cc.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	if src != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, oauth2.NewClient(ctx, src))
	}

	// Get the current server configuration for the registration endpoint address
	srv, _, _, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return nil, err
	}
	c := registration.Config{
		RegistrationURL: srv.Authorization.RegistrationEndpoint,
	}
	return c.Register(ctx, client)
}

// NewAuthorization creates a new authorization code flow with PKCE using the current context
func (cc *ClientConfig) NewAuthorization() (*authorizationcode.AuthorizationCodeFlowWithPKCE, error) {
	srv, _, _, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return nil, err
	}

	az, err := authorizationcode.NewAuthorizationCodeFlowWithPKCE()
	if err != nil {
		return nil, err
	}

	az.Audience = srv.Identifier
	az.Endpoint.AuthURL = srv.Authorization.AuthorizationEndpoint
	az.Endpoint.TokenURL = srv.Authorization.TokenEndpoint
	az.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	return az, nil
}

// NewDeviceAuthorization creates a new device authorization flow using the current context
func (cc *ClientConfig) NewDeviceAuthorization() (*devicecode.DeviceFlow, error) {
	srv, _, _, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
	if err != nil {
		return nil, err
	}

	return &devicecode.DeviceFlow{
		Config: clientcredentials.Config{
			TokenURL:  srv.Authorization.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		DeviceAuthorizationURL: srv.Authorization.DeviceAuthorizationEndpoint,
		EndpointParams:         map[string][]string{"audience": {srv.Identifier}},
	}, nil
}

// Authorize configures the supplied transport
func (cc *ClientConfig) Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error) {
	// Get the token source and use it to wrap the transport
	src, err := cc.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	if src != nil {
		return &oauth2.Transport{Source: src, Base: transport}, nil
	}
	return transport, nil
}

func (cc *ClientConfig) tokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	// TODO We could make ClientConfig implement the TokenSource interface, but we need a way to handle the context
	srv, az, _, _, err := contextConfig(&cc.data, cc.data.CurrentContext)
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
func (cc *ClientConfig) Merge(data *Config) {
	mergeServers(&cc.data, data.Servers)
	mergeAuthorizations(&cc.data, data.Authorizations)
	mergeClusters(&cc.data, data.Clusters)
	mergeControllers(&cc.data, data.Controllers)
	mergeContexts(&cc.data, data.Contexts)
	mergeString(&cc.data.CurrentContext, data.CurrentContext)
}

// contextConfig returns all of the configurations objects for the named context
func contextConfig(data *Config, name string) (*Server, *Authorization, *Cluster, *Controller, error) {
	ctx := findContext(data.Contexts, name)
	if ctx == nil {
		return nil, nil, nil, nil, fmt.Errorf("could not find context (%s)", name)
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
