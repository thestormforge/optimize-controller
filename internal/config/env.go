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

import "os"

func envLoader(cfg *RedSkyConfig) error {
	(&env{
		serverIdentifier:          os.Getenv("REDSKY_SERVER_IDENTIFIER"),
		serverIssuer:              os.Getenv("REDSKY_SERVER_ISSUER"),
		authorizationClientID:     os.Getenv("REDSKY_AUTHORIZATION_CLIENT_ID"),
		authorizationClientSecret: os.Getenv("REDSKY_AUTHORIZATION_CLIENT_SECRET"),
	}).applyConfig(&cfg.data)

	return nil
}

type env struct {
	serverIdentifier          string
	serverIssuer              string
	authorizationClientID     string
	authorizationClientSecret string
}

func (e *env) applyConfig(cfg *Config) {
	// Find the current or only context
	ctx := findContext(cfg.Contexts, cfg.CurrentContext)
	if ctx == nil && len(cfg.Contexts) == 1 {
		ctx = &cfg.Contexts[0].Context
	}

	// Determine the names for the server and authorization
	serverName := "default"
	authorizationName := ""
	if ctx != nil {
		mergeString(&serverName, ctx.Server)
		mergeString(&authorizationName, ctx.Authorization)
	}
	defaultString(&authorizationName, serverName)

	// Apply the environment configuration
	if e.serverIdentifier != "" || e.serverIssuer != "" {
		e.applyServer(cfg, serverName)
	}
	if e.authorizationClientID != "" && e.authorizationClientSecret != "" {
		e.applyAuthorization(cfg, authorizationName)
	}
}

func (e *env) applyServer(cfg *Config, name string) {
	// Find or create the server
	srv := findServer(cfg.Servers, name)
	if srv == nil {
		cfg.Servers = append(cfg.Servers, NamedServer{Name: name})
		srv = &cfg.Servers[len(cfg.Servers)-1].Server
	}

	// Overwrite the API server identifier and authorization server issuer
	mergeString(&srv.Identifier, e.serverIdentifier)
	mergeString(&srv.Authorization.Issuer, e.serverIssuer)
}

func (e *env) applyAuthorization(cfg *Config, name string) {
	// Find or create the authorization
	az := findAuthorization(cfg.Authorizations, name)
	if az == nil {
		cfg.Authorizations = append(cfg.Authorizations, NamedAuthorization{Name: name})
		az = &cfg.Authorizations[len(cfg.Authorizations)-1].Authorization
	}

	// Overwrite the authorization credential
	mergeAuthorization(az, &Authorization{Credential: Credential{ClientCredential: &ClientCredential{
		ClientID:     e.authorizationClientID,
		ClientSecret: e.authorizationClientSecret,
	}}})
}

// LegacyEnvMapping produces a map of environment variables generated from a configuration
func LegacyEnvMapping(cfg *RedSkyConfig, includeController bool) (map[string][]byte, error) {
	srv, az, _, ctrl, err := contextConfig(&cfg.data)
	if err != nil {
		return nil, err
	}

	env := make(map[string][]byte)

	// Record the server information
	env["REDSKY_SERVER_IDENTIFIER"] = []byte(srv.Identifier)
	env["REDSKY_SERVER_ISSUER"] = []byte(srv.Authorization.Issuer)

	// Record the authorization information
	if az.Credential.ClientCredential != nil {
		env["REDSKY_AUTHORIZATION_CLIENT_ID"] = []byte(az.Credential.ClientID)
		env["REDSKY_AUTHORIZATION_CLIENT_SECRET"] = []byte(az.Credential.ClientSecret)
	}

	// Optionally record environment variables from the controller configuration
	if includeController {
		for i := range ctrl.Env {
			env[ctrl.Env[i].Name] = []byte(ctrl.Env[i].Value)
		}
	}

	// Strip out blanks
	for k, v := range env {
		if len(v) == 0 {
			delete(env, k)
		}
	}
	return env, nil
}
