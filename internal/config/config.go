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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/redskyops/redskyops-controller/internal/oauth2/authorizationcode"
	"github.com/redskyops/redskyops-controller/internal/oauth2/devicecode"
	"github.com/redskyops/redskyops-controller/internal/oauth2/registration"
	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// audience is the logical identifier of the Red Sky API
const audience = "https://api.carbonrelay.io/v1/"

// Loader is used to initially populate a Red Sky configuration
type Loader func(cfg *RedSkyConfig) error

// Change is used to apply a configuration change that should be persisted
type Change func(cfg *Config) error

// ClientIdentity is a mapping function that returns an OAuth 2.0 `client_id` given an authorization server issuer identifier
type ClientIdentity func(string) string

// Endpoints exposes the Red Sky API server endpoint locations as a mapping of prefixes to base URLs
type Endpoints map[string]*url.URL

// RedSkyConfig is the structure used to manage configuration data
type RedSkyConfig struct {
	// Filename is the path to the configuration file; if left blank, it will be populated using XDG base directory conventions on the next Load
	Filename string
	// Overrides to the standard configuration
	Overrides Overrides
	// ClientIdentity is used to determine the OAuth 2.0 client identifier
	ClientIdentity ClientIdentity

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
	loaders = append(loaders, fileLoader, envLoader, migrationLoader)
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
	return &overrideReader{overrides: &rsc.Overrides, delegate: &defaultReader{cfg: &rsc.data}}
}

// SystemNamespace returns the namespace where the Red Sky controller is/should be installed
func (rsc *RedSkyConfig) SystemNamespace() (string, error) {
	ctrl, err := CurrentController(rsc.Reader())
	if err != nil {
		return "", nil
	}
	return ctrl.Namespace, nil
}

// Endpoints returns a resolver that can generate fully qualified endpoint URLs
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
func (rsc *RedSkyConfig) Kubectl(ctx context.Context, arg ...string) (*exec.Cmd, error) {
	cstr, err := CurrentCluster(rsc.Reader())
	if err != nil {
		return nil, err
	}

	var globals []string

	if cstr.KubeConfig != "" {
		globals = appendIfNotPresent(globals, arg, "--kubeconfig", cstr.KubeConfig)
	}

	if cstr.Context != "" {
		globals = appendIfNotPresent(globals, arg, "--context", cstr.Context)
	}

	if cstr.Namespace != "" {
		globals = appendIfNotPresent(globals, arg, "--namespace", cstr.Namespace)
	}

	return exec.CommandContext(ctx, cstr.Bin, append(globals, arg...)...), nil
}

// appendIfNotPresent is meant to allow args coming to override globals rather then relying on unspecified behavior
func appendIfNotPresent(s []string, arg []string, flag, value string) []string {
	// This won't catch things like a global --namespace and a -n arg
	for i := range arg {
		if arg[i] == flag {
			return s
		}
	}
	return append(s, flag, value)
}

// RevocationInformation contains the information necessary to revoke an authorization credential
type RevocationInformation struct {
	// RevocationURL is the URL of the authorization server's revocation endpoint
	RevocationURL string
	// ClientID is the client identifier for the authorization
	ClientID string
	// Authorization is the credential that needs to be revoked
	Authorization Authorization

	// authorization name is used internally so revocation information can be a change
	authorizationName string
}

// String returns a string representation of this revocation
func (ri *RevocationInformation) String() string {
	return ri.authorizationName
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
		ClientID:          rsc.clientID(srv),
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

	c.ClientID = rsc.clientID(&srv)
	c.Endpoint.AuthURL = srv.Authorization.AuthorizationEndpoint
	c.Endpoint.TokenURL = srv.Authorization.TokenEndpoint
	c.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	c.EndpointParams = map[string][]string{"audience": {audience}}
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
			ClientID:  rsc.clientID(&srv),
			TokenURL:  srv.Authorization.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		DeviceAuthorizationURL: srv.Authorization.DeviceAuthorizationEndpoint,
		EndpointParams:         map[string][]string{"audience": {audience}},
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
	azName, err := r.AuthorizationName(r.ContextName())
	if err != nil {
		return nil, err
	}
	az, err := r.Authorization(azName)
	if err != nil {
		return nil, err
	}

	if az.Credential.ClientCredential != nil {
		cc := clientcredentials.Config{
			ClientID:       az.Credential.ClientID,
			ClientSecret:   az.Credential.ClientSecret,
			TokenURL:       srv.Authorization.TokenEndpoint,
			EndpointParams: url.Values{"audience": []string{audience}},
			AuthStyle:      oauth2.AuthStyleInParams,
		}
		return cc.TokenSource(ctx), nil
	}

	if az.Credential.TokenCredential != nil {
		c := &oauth2.Config{
			ClientID: rsc.clientID(&srv),
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
		return &updateTokenSource{
			src: c.TokenSource(ctx, t),
			cfg: rsc,
			az:  azName,
		}, nil
	}

	return nil, nil
}

// PublicKey attempts to fetch a public key with the specified identifier from the JWKS of the current server
func (rsc *RedSkyConfig) PublicKey(ctx context.Context, keyID interface{}) (interface{}, error) {
	// Make sure the key identifier is a string
	kid, ok := keyID.(string)
	if !ok {
		return nil, fmt.Errorf("expected string key identifier")
	}

	// Get the server configuration for the JWKS URL
	srv, err := CurrentServer(rsc.Reader())
	if err != nil {
		return nil, err
	}

	// Use the OAuth HTTP client to fetch the JWKS
	req, err := http.NewRequest(http.MethodGet, srv.Authorization.JSONWebKeySetURI, nil)
	if err != nil {
		return nil, err
	}
	r, err := ctxhttp.Do(ctx, oauth2.NewClient(ctx, nil), req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("jwks: cannot fetch key set: %v", err)
	}
	if code := r.StatusCode; code < 200 || code > 299 {
		return nil, fmt.Errorf("jwks: cannot fetch key set: %v", code)
	}

	// Decode the JWKS and try to locate a key
	set, err := jwk.ParseBytes(body)
	if err != nil {
		return nil, err
	}
	keys := set.LookupKeyID(kid)
	for i := range keys {
		if key, err := keys[i].Materialize(); err == nil {
			return key, nil
		}
	}
	return nil, fmt.Errorf("jwks: unable to find key: %v", keyID)
}

func (rsc *RedSkyConfig) clientID(srv *Server) (id string) {
	if rsc.ClientIdentity != nil {
		return rsc.ClientIdentity(srv.Authorization.Issuer)
	}

	switch srv.Authorization.Issuer {
	case "https://auth.carbonrelay.io/":
		id = "pE3kMKdrMTdW4DOxQHesyAuFGNOWaEke"
	case "https://carbonrelay-dev.auth0.com/":
		id = "fmbRPm2zoQJ64hb37CUJDJVmRLHhE04Y"
	case "":
		id = ""
	default:
		// OAuth specifications warning against mix-ups, instead of using a fixed environment variable name, the name
		// should be derived from the issuer: this helps ensure we do not send the client identifier to the wrong server.

		// PRECONDITION: issuer identifiers must be https:// URIs with no query or fragment
		prefix := strings.ReplaceAll(strings.TrimPrefix(srv.Authorization.Issuer, "https://"), "//", "/")
		prefix = strings.ReplaceAll(strings.TrimRight(prefix, "/"), "/", "//") + "/"
		prefix = strings.Map(func(r rune) rune {
			switch {
			case r >= 'A' && r <= 'Z':
				return r
			case r == '.' || r == '/':
				return '_'
			}
			return -1
		}, strings.ToUpper(prefix))

		id = os.Getenv(prefix + "CLIENT_ID")
	}

	return id
}

type updateTokenSource struct {
	src oauth2.TokenSource
	cfg *RedSkyConfig
	az  string
}

func (u *updateTokenSource) Token() (*oauth2.Token, error) {
	t, err := u.src.Token()
	if err != nil {
		return nil, err
	}
	if az, err := u.cfg.Reader().Authorization(u.az); err == nil {
		if az.Credential.TokenCredential != nil && az.Credential.AccessToken != t.AccessToken {
			_ = u.cfg.Update(SaveToken(u.az, t))
			_ = u.cfg.Write()
		}
	}
	return t, nil
}
