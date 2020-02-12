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

// NewEnvOverrides returns a new collection of overrides based on the current environment
func NewEnvOverrides() *Overrides {
	return &Overrides{
		ServerIdentifier: os.Getenv("REDSKY_SERVER_IDENTIFIER"),
		ServerIssuer:     os.Getenv("REDSKY_SERVER_ISSUER"),
		Credential: ClientCredential{
			ClientID:     os.Getenv("REDSKY_AUTHORIZATION_CLIENT_ID"),
			ClientSecret: os.Getenv("REDSKY_AUTHORIZATION_CLIENT_SECRET"),
		},
	}
}

// EnvironmentMapping returns an environment variable map from the specified configuration reader
func EnvironmentMapping(r Reader, includeController bool) (map[string][]byte, error) {
	env := make(map[string][]byte)

	// Record the server information
	srv, err := CurrentServer(r)
	if err != nil {
		return nil, err
	}
	env["REDSKY_SERVER_IDENTIFIER"] = []byte(srv.Identifier)
	env["REDSKY_SERVER_ISSUER"] = []byte(srv.Authorization.Issuer)

	// Record the authorization information
	az, err := CurrentAuthorization(r)
	if err != nil {
		return nil, err
	}
	if az.Credential.ClientCredential != nil {
		env["REDSKY_AUTHORIZATION_CLIENT_ID"] = []byte(az.Credential.ClientID)
		env["REDSKY_AUTHORIZATION_CLIENT_SECRET"] = []byte(az.Credential.ClientSecret)
	}

	// Optionally record environment variables from the controller configuration
	if includeController {
		ctrl, err := CurrentController(r)
		if err != nil {
			return nil, err
		}

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
