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
	"strings"

	"golang.org/x/oauth2"
)

// SaveServer is a configuration change that persists the supplied server configuration. If the server exists,
// it is overwritten; otherwise a new named server is created.
func SaveServer(name string, srv *Server) Change {
	return func(cfg *Config) error {
		mergeServers(cfg, []NamedServer{{Name: name, Server: *srv}})
		mergeAuthorizations(cfg, []NamedAuthorization{{Name: name}})

		// Make sure we capture the current value of the default server identifier
		defaultString(&findServer(cfg.Servers, name).Identifier, DefaultServerIdentifier)
		return nil
	}
}

// SaveToken is a configuration change that persists the supplied token as a named authorization. If the authorization
// exists, it is overwritten; otherwise a new named authorization is created.
func SaveToken(name string, t *oauth2.Token) Change {
	return func(cfg *Config) error {
		az := findAuthorization(cfg.Authorizations, name)
		if az == nil {
			cfg.Authorizations = append(cfg.Authorizations, NamedAuthorization{Name: name})
			az = &cfg.Authorizations[len(cfg.Authorizations)-1].Authorization
		}

		az.Credential.ClientCredential = nil
		az.Credential.TokenCredential = &TokenCredential{
			AccessToken:  t.AccessToken,
			TokenType:    t.TokenType,
			RefreshToken: t.RefreshToken,
			Expiry:       t.Expiry,
		}
		return nil
	}
}

// ApplyCurrentContext is a configuration change that updates the values of a context and sets that context as the
// current context. If the context exists, non-empty values will overwrite; otherwise a new named context is created.
func ApplyCurrentContext(contextName, serverName, authorizationName, clusterName string) Change {
	return func(cfg *Config) error {
		ctx := findContext(cfg.Contexts, contextName)
		if ctx == nil {
			cfg.Contexts = append(cfg.Contexts, NamedContext{Name: contextName})
			ctx = &cfg.Contexts[len(cfg.Contexts)-1].Context
		}

		mergeString(&cfg.CurrentContext, contextName)
		mergeString(&ctx.Server, serverName)
		mergeString(&ctx.Authorization, authorizationName)
		mergeString(&ctx.Cluster, clusterName)
		return nil
	}
}

// SetProperty is a configuration change that updates a single property using a dotted name notation.
func SetProperty(name, value string) Change {
	// TODO This is a giant hack. Consider not even supporting `redskyctl config set` generically
	return func(cfg *Config) error {
		path := strings.Split(name, ".")
		switch path[0] {
		case "current-context":
			cfg.CurrentContext = value
			return nil
		case "cluster":
			if len(path) == 3 {
				cstr := findCluster(cfg.Clusters, path[1])
				if cstr == nil {
					return fmt.Errorf("unknown cluster: %s", path[1])
				}
				switch path[2] {
				case "context":
					cstr.Context = value
					return nil
				case "bin":
					cstr.Bin = value
					return nil
				case "controller":
					cstr.Controller = value
					return nil
				}
			}
		case "controller":
			if len(path) == 4 && path[2] == "env" {
				mergeControllers(cfg, []NamedController{{
					Name:       path[1],
					Controller: Controller{Env: []ControllerEnvVar{{Name: path[3], Value: value}}},
				}})
				return nil
			}
		case "context":
			if len(path) == 3 {
				ctx := findContext(cfg.Contexts, path[1])
				if ctx == nil {
					return fmt.Errorf("unknown context: %s", path[1])
				}
				switch path[2] {
				case "server":
					if findServer(cfg.Servers, value) == nil {
						return fmt.Errorf("unknown %s reference: %s", path[2], value)
					}
					ctx.Server = value
					return nil
				case "authorization":
					if findAuthorization(cfg.Authorizations, value) == nil {
						return fmt.Errorf("unknown %s reference: %s", path[2], value)
					}
					ctx.Authorization = value
					return nil
				case "cluster":
					if findCluster(cfg.Clusters, value) == nil {
						return fmt.Errorf("unknown %s reference: %s", path[2], value)
					}
					ctx.Cluster = value
					return nil
				}
			}
		}
		return fmt.Errorf("unknown config property: %s", name)
	}
}
