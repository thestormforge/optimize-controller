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

	"github.com/redskyops/redskyops-controller/internal/oauth2/authorizationcode"
	"github.com/redskyops/redskyops-controller/internal/oauth2/devicecode"
	"github.com/redskyops/redskyops-controller/internal/oauth2/registration"
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
	// Overrides to the standard configuration
	Overrides *Overrides

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
	loaders = append(loaders, defaultLoader)
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

// Reader returns a configuration reader for accessing information from the configuration
func (rsc *RedSkyConfig) Reader() Reader {
	r := &defaultReader{cfg: &rsc.data}
	if rsc.Overrides != nil {
		return &overrideReader{overrides: rsc.Overrides, delegate: r}
	}
	return r
}

// SystemNamespace returns the namespace where the Red Sky controller is/should be installed
func (rsc *RedSkyConfig) SystemNamespace() (string, error) {
	ctrl, err := CurrentController(rsc.Reader())
	if err != nil {
		return "", nil
	}
	return ctrl.Namespace, nil
}

// EndpointLocations returns a resolver that can generate fully qualified endpoint URLs
func (rsc *RedSkyConfig) Endpoints() (Endpoints, error) {
	srv, err := CurrentServer(rsc.Reader())
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
	cstr, err := CurrentCluster(rsc.Reader())
	if err != nil {
		return nil, err
	}

	var globals []string

	if cstr.KubeConfig != "" {
		globals = append(globals, "--kubeconfig", cstr.KubeConfig)
	}

	if cstr.Context != "" {
		globals = append(globals, "--context", cstr.Context)
	}

	// TODO Use CommandContext and accept a context
	return exec.Command(cstr.Bin, append(globals, arg...)...), nil
}

// RevocationInformation contains the information necessary to revoke an authorization credential
type RevocationInformation struct {
	// RevocationURL is the URL of the authorization server's revocation endpoint
	RevocationURL string
	// Authorization is the credential that needs to be revoked
	Authorization Authorization

	// authorization name is used internally so revocation information can be a change
	authorizationName string
}

// RemoveAuthorization returns a configuration change to remove the authorization associated with the revocation information
func (ri *RevocationInformation) RemoveAuthorization() Change {
	return func(cfg *Config) error {
		for i := range cfg.Contexts {
			if cfg.Contexts[i].Context.Authorization == ri.authorizationName {
				cfg.Contexts[i].Context.Authorization = ""
			}
		}
		for i := range cfg.Authorizations {
			if cfg.Authorizations[i].Name == ri.authorizationName {
				cfg.Authorizations = append(cfg.Authorizations[:i], cfg.Authorizations[i+1:]...)
				return nil
			}
		}
		return nil
	}
}

// RevocationInfo returns the information necessary to revoke an authorization entry from the configuration
func (rsc *RedSkyConfig) RevocationInfo(name string) (*RevocationInformation, error) {
	var authorizationName, serverName string
	for i := range rsc.data.Contexts {
		// Allow a blank name for the authorization to revoke the current context authorization
		if (name != "" && name == rsc.data.Contexts[i].Context.Authorization) ||
			(name == "" && rsc.data.Contexts[i].Name == rsc.data.CurrentContext) {
			authorizationName = rsc.data.Contexts[i].Context.Authorization
			serverName = rsc.data.Contexts[i].Context.Server
		}
	}

	az := findAuthorization(rsc.data.Authorizations, authorizationName)
	if authorizationName == "" || az == nil {
		return nil, fmt.Errorf("unknown authorization: %s", name)
	}

	srv := findServer(rsc.data.Servers, serverName)
	if serverName == "" || srv == nil {
		return nil, fmt.Errorf("unable to find server for authorization: %s", authorizationName)
	}

	return &RevocationInformation{
		RevocationURL:     srv.Authorization.RevocationEndpoint,
		Authorization:     *az,
		authorizationName: authorizationName,
	}, nil
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
	srv, err := CurrentServer(rsc.Reader())
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
	srv, err := CurrentServer(rsc.Reader())
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
	srv, err := CurrentServer(rsc.Reader())
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
	r := rsc.Reader()
	srv, err := CurrentServer(r)
	if err != nil {
		return nil, err
	}
	az, err := CurrentAuthorization(r)
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
