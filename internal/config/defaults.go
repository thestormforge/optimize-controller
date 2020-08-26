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
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/redskyops/redskyops-controller/internal/oauth2/discovery"
)

// The default loader must NEVER make changes via RedSkyConfig.Update or RedSkyConfig.unpersisted

func defaultLoader(cfg *RedSkyConfig) error {
	// NOTE: Any errors reported here are effectively fatal errors for a program that needs configuration since they will
	// not be able to load the configuration. Errors should be limited to unusable configurations.

	d := &defaults{cfg: &cfg.data, env: cfg.Environment(), clusterName: "default"}

	// This constitutes a "bootstrap" invocation of "kubectl", we can't use the configuration because we are actually creating it
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={.clusters[0].name}")
	if stdout, err := cmd.Output(); err == nil {
		d.clusterName = strings.TrimSpace(string(stdout))
	}

	d.addDefaultObjects()
	if err := d.applyServerDefaults(); err != nil {
		return err
	}
	// No defaults for authorizations
	if err := d.applyClusterDefaults(); err != nil {
		return err
	}
	if err := d.applyControllerDefaults(); err != nil {
		return err
	}
	if err := d.applyContextDefaults(); err != nil {
		return err
	}
	return nil
}

// defaultString overwrites an empty s1 with the value of s2
func defaultString(s1 *string, s2 string) {
	if *s1 == "" {
		*s1 = s2
	}
}

func defaultServerRoots(env string, srv *Server) error {
	// The environment corresponds to deployment details of the proprietary backend
	switch env {
	case "production":
		defaultString(&srv.Identifier, "https://api.carbonrelay.io/v1/")
		defaultString(&srv.Authorization.Issuer, "https://auth.carbonrelay.io/")
	case "development":
		defaultString(&srv.Identifier, "https://api.carbonrelay.dev/v1/")
		defaultString(&srv.Authorization.Issuer, "https://auth.carbonrelay.dev/")
	default:
		return fmt.Errorf("unknown environment: '%s'", env)
	}
	return nil
}

func defaultServerEndpoints(srv *Server) error {
	// Determine the default base URLs
	api, err := discovery.IssuerURL(srv.Identifier)
	if err != nil {
		return err
	}
	issuer, err := discovery.IssuerURL(srv.Authorization.Issuer)
	if err != nil {
		return err
	}

	// Apply the Red Sky defaults
	defaultString(&srv.RedSky.ExperimentsEndpoint, api+"/experiments/")
	defaultString(&srv.RedSky.AccountsEndpoint, api+"/accounts/")

	// Apply the authorization defaults
	// TODO We should try discovery, e.g. fetch `discovery.WellKnownURI(issuer, "oauth-authorization-server")` and _merge_ (not _default_ since the server reported values win)
	defaultString(&srv.Authorization.AuthorizationEndpoint, issuer+"/authorize")
	defaultString(&srv.Authorization.TokenEndpoint, issuer+"/oauth/token")
	defaultString(&srv.Authorization.RevocationEndpoint, issuer+"/oauth/revoke")
	// defaultString(&srv.Authorization.RegistrationEndpoint, issuer+"/oauth/register")
	defaultString(&srv.Authorization.DeviceAuthorizationEndpoint, issuer+"/oauth/device/code")
	defaultString(&srv.Authorization.JSONWebKeySetURI, discovery.WellKnownURI(issuer, "jwks.json"))

	// Special case for the registration service which is actually part of the accounts API
	if u, err := url.Parse(srv.RedSky.AccountsEndpoint); err != nil {
		defaultString(&srv.Authorization.RegistrationEndpoint, api+"/accounts/clients")
	} else {
		u.Path = strings.TrimRight(u.Path, "/") + "/clients"
		defaultString(&srv.Authorization.RegistrationEndpoint, u.String())
	}

	return nil
}

type defaults struct {
	cfg         *Config
	env         string
	clusterName string
}

func (d *defaults) addDefaultObjects() {
	if len(d.cfg.Servers) == 0 {
		d.cfg.Servers = append(d.cfg.Servers, NamedServer{Name: "default"})
	}

	if len(d.cfg.Authorizations) == 0 {
		d.cfg.Authorizations = append(d.cfg.Authorizations, NamedAuthorization{Name: "default"})
	}

	if len(d.cfg.Clusters) == 0 {
		d.cfg.Clusters = append(d.cfg.Clusters, NamedCluster{Name: d.clusterName})
	}

	if len(d.cfg.Controllers) == 0 {
		d.cfg.Controllers = append(d.cfg.Controllers, NamedController{Name: d.clusterName})
	}

	if len(d.cfg.Contexts) == 0 {
		d.cfg.Contexts = append(d.cfg.Contexts, NamedContext{Name: "default"})
	}
}

func (d *defaults) applyServerDefaults() error {
	for i := range d.cfg.Servers {
		srv := &d.cfg.Servers[i].Server

		if err := defaultServerRoots(d.env, srv); err != nil {
			return err
		}

		if err := defaultServerEndpoints(srv); err != nil {
			return err
		}
	}
	return nil
}

func (d *defaults) applyClusterDefaults() error {
	for i := range d.cfg.Clusters {
		cstr := &d.cfg.Clusters[i].Cluster

		defaultString(&cstr.Bin, "kubectl")

		if err := d.defaultControllerName(&cstr.Controller, d.cfg.Clusters[i].Name); err != nil {
			return err
		}
	}
	return nil
}

func (d *defaults) applyControllerDefaults() error {
	for i := range d.cfg.Controllers {
		ctrl := &d.cfg.Controllers[i].Controller

		defaultString(&ctrl.Namespace, "redsky-system")
	}
	return nil
}

func (d *defaults) applyContextDefaults() error {
	for i := range d.cfg.Contexts {
		ctx := &d.cfg.Contexts[i].Context
		name := d.cfg.Contexts[i].Name

		if err := d.defaultServerName(&ctx.Server, name); err != nil {
			return err
		}

		if err := d.defaultAuthorizationName(&ctx.Authorization, name, ctx.Server); err != nil {
			return err
		}

		if err := d.defaultClusterName(&ctx.Cluster, name); err != nil {
			return err
		}
	}

	if err := d.defaultContextName(&d.cfg.CurrentContext); err != nil {
		return err
	}

	return nil
}

// Default name functions attempt to resolve a default name

func (d *defaults) defaultServerName(s *string, name string) error {
	if findServer(d.cfg.Servers, name) != nil {
		defaultString(s, name)
		return nil
	}
	if len(d.cfg.Servers) == 1 {
		defaultString(s, d.cfg.Servers[0].Name)
		return nil
	}
	if findServer(d.cfg.Servers, "default") != nil {
		defaultString(s, "default")
		return nil
	}
	if *s != "" {
		return nil
	}
	return fmt.Errorf("could not imply default server name for context: %s", name)
}

func (d *defaults) defaultAuthorizationName(s *string, name, server string) error {
	if findAuthorization(d.cfg.Authorizations, name) != nil {
		defaultString(s, name)
		return nil
	}
	if findAuthorization(d.cfg.Authorizations, server) != nil {
		defaultString(s, server)
		return nil
	}
	if len(d.cfg.Authorizations) == 1 {
		defaultString(s, d.cfg.Authorizations[0].Name)
		return nil
	}
	if findAuthorization(d.cfg.Authorizations, "default") != nil {
		defaultString(s, "default")
		return nil
	}
	if *s != "" {
		return nil
	}
	return fmt.Errorf("could not imply default authorization name for context: %s", name)
}

func (d *defaults) defaultClusterName(s *string, name string) error {
	if findCluster(d.cfg.Clusters, name) != nil {
		defaultString(s, name)
		return nil
	}
	if len(d.cfg.Clusters) == 1 {
		defaultString(s, d.cfg.Clusters[0].Name)
		return nil
	}
	if findCluster(d.cfg.Clusters, d.clusterName) != nil {
		defaultString(s, d.clusterName)
		return nil
	}
	if *s != "" {
		return nil
	}
	return fmt.Errorf("could not imply default cluster name for context: %s", name)
}

func (d *defaults) defaultControllerName(s *string, name string) error {
	if findController(d.cfg.Controllers, name) != nil {
		defaultString(s, name)
		return nil
	}
	if len(d.cfg.Controllers) == 1 {
		defaultString(s, d.cfg.Controllers[0].Name)
		return nil
	}
	if findController(d.cfg.Controllers, d.clusterName) != nil {
		defaultString(s, d.clusterName)
		return nil
	}
	if *s != "" {
		return nil
	}
	return fmt.Errorf("could not imply default controller name for cluster: %s", name)
}

func (d *defaults) defaultContextName(s *string) error {
	if len(d.cfg.Contexts) == 1 {
		defaultString(s, d.cfg.Contexts[0].Name)
		return nil
	}
	if findContext(d.cfg.Contexts, "default") != nil {
		defaultString(s, "default")
		return nil
	}
	if *s != "" {
		return nil
	}
	return fmt.Errorf("could not imply default current context")
}
