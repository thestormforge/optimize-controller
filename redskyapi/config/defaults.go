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
	"net/url"
	"os/exec"
	"strings"
)

// The default loader must NEVER make changes via ClientConfig.Update or ClientConfig.unpersisted

var (
	// DefaultServerIdentifier is the default entrypoint to the remote application
	DefaultServerIdentifier = "https://api.carbonrelay.io/v1/"
)

func defaultLoader(cfg *ClientConfig) error {
	defaultServerName := "default"
	defaultClusterName := clusterName()
	defaultControllerName := defaultClusterName
	defaultContextName := "default"

	if len(cfg.data.Servers) == 0 {
		cfg.data.Servers = append(cfg.data.Servers, NamedServer{Name: defaultServerName})
	}

	if len(cfg.data.Authorizations) == 0 {
		cfg.data.Authorizations = append(cfg.data.Authorizations, NamedAuthorization{Name: defaultServerName})
	}

	if len(cfg.data.Clusters) == 0 {
		cfg.data.Clusters = append(cfg.data.Clusters, NamedCluster{Name: defaultClusterName})
	}

	if len(cfg.data.Controllers) == 0 {
		cfg.data.Controllers = append(cfg.data.Controllers, NamedController{Name: defaultControllerName})
	}

	if len(cfg.data.Contexts) == 0 {
		cfg.data.Contexts = append(cfg.data.Contexts, NamedContext{Name: defaultContextName})
	}

	for i := range cfg.data.Servers {
		if err := defaultServer(&cfg.data.Servers[i].Server); err != nil {
			return err
		}
	}

	// No defaults for authorizations

	for i := range cfg.data.Clusters {
		// TODO This is wrong if there are multiple objects, none of which have a default name
		if err := defaultCluster(&cfg.data.Clusters[i].Cluster, &cfg.data, defaultClusterName); err != nil {
			return err
		}
	}

	for i := range cfg.data.Controllers {
		if err := defaultController(&cfg.data.Controllers[i].Controller); err != nil {
			return err
		}
	}

	for i := range cfg.data.Contexts {
		// TODO This is wrong if there are multiple objects, none of which have a default name
		if err := defaultContext(&cfg.data.Contexts[i].Context, &cfg.data, defaultServerName, defaultClusterName); err != nil {
			return err
		}
	}

	// TODO This is wrong if there are multiple objects, none of which have a default name
	if len(cfg.data.Contexts) == 1 {
		mergeString(&cfg.data.CurrentContext, cfg.data.Contexts[0].Name)
	}
	mergeString(&cfg.data.CurrentContext, defaultContextName)

	return nil
}

func defaultServer(srv *Server) error {
	mergeString(&srv.Identifier, DefaultServerIdentifier)

	// TODO We should try discovery, e.g. fetch "{srv.Identifier without path}/.well-known/oauth-authorization-server[{srv.Identifier path}]

	// Hard coded defaults for the default server
	if srv.Identifier == DefaultServerIdentifier {
		mergeString(&srv.RedSky.ExperimentsEndpoint, "https://api.carbonrelay.io/v1/experiments")
		mergeString(&srv.RedSky.AccountsEndpoint, "https://api.carbonrelay.io/v1/accounts")
		mergeString(&srv.Authorization.AuthorizationEndpoint, "https://login.carbonrelay.io/authorize")
		mergeString(&srv.Authorization.TokenEndpoint, "https://login.carbonrelay.io/oauth/token")
		mergeString(&srv.Authorization.RegistrationEndpoint, "https://api.carbonrelay.io/v1/accounts/clients/register")
		return nil
	}

	// Try to generate defaults based on the server identifier
	u, err := url.Parse(srv.Identifier)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/")
	base := u.String()

	mergeString(&srv.RedSky.ExperimentsEndpoint, base+"/v1/experiments")
	mergeString(&srv.RedSky.AccountsEndpoint, base+"/v1/accounts")
	mergeString(&srv.Authorization.AuthorizationEndpoint, base+"/authorize")
	mergeString(&srv.Authorization.TokenEndpoint, base+"/oauth/token")
	mergeString(&srv.Authorization.RegistrationEndpoint, base+"/oauth/register")
	return nil
}

func defaultCluster(cstr *Cluster, cfg *Config, defaultClusterName string) error {
	if len(cfg.Clusters) == 1 {
		mergeString(&cstr.Controller, cfg.Clusters[0].Name)
	}

	mergeString(&cstr.Bin, "kubectl")
	mergeString(&cstr.Controller, defaultClusterName)
	return nil
}

func defaultController(ctrl *Controller) error {
	mergeString(&ctrl.Namespace, "redsky-system")
	return nil
}

func defaultContext(ctx *Context, cfg *Config, defaultServerName, defaultClusterName string) error {
	if len(cfg.Servers) == 1 {
		mergeString(&ctx.Server, cfg.Servers[0].Name)
	}
	if len(cfg.Authorizations) == 1 {
		mergeString(&ctx.Authorization, cfg.Authorizations[0].Name)
	}
	if len(cfg.Clusters) == 1 {
		mergeString(&ctx.Cluster, cfg.Clusters[0].Name)
	}

	mergeString(&ctx.Server, defaultServerName)
	mergeString(&ctx.Authorization, defaultServerName)
	mergeString(&ctx.Cluster, defaultClusterName)
	return nil
}

// clusterName returns the current cluster name from kubeconfig
func clusterName() string {
	// This constitutes a "bootstrap" invocation of "kubectl", we can't use the configuration because we are actually creating it
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={.clusters[0].name}")
	stdout, err := cmd.Output()
	if err != nil {
		return "default"
	}
	return strings.TrimSpace(string(stdout))
}
