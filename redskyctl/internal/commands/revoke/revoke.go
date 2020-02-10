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

package revoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/redskyops/k8s-experiment/internal/config"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commander"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commands/login"
	"github.com/spf13/cobra"
	"golang.org/x/net/context/ctxhttp"
)

// Options is the configuration for removing authorization entries in a configuration
type Options struct {
	// Config is the Red Sky Configuration to modify
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Name is the key of the authorization in the configuration to remove
	Name string
}

// NewCommand creates a new command for executing a logout
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke an authorization",
		Long:  "Log out of your Red Sky Account.",

		PreRun: commander.StreamsPreRun(&o.IOStreams),

		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			commander.CheckErr(cmd, err)
		},
	}

	cmd.Flags().StringVar(&o.Name, "name", "", "Name of the server configuration to de-authorize.")

	return cmd
}

// Run executes the revocation
func (o *Options) Run() error {
	return o.Config.Update(func(cfg *config.Config) error {
		// Default the name to the current authorization
		name := o.authorizationName(cfg)

		// Find the authorization record and the corresponding revocation endpoint
		i, endpoint, err := indexOfAuthorizationServer(cfg, name)
		if err != nil {
			return err
		}

		// Revoke the credential
		az := &cfg.Authorizations[i].Authorization
		if az.Credential.TokenCredential != nil {
			if err := revokeToken(context.Background(), endpoint, login.ClientID, az.Credential.RefreshToken); err != nil {
				return err
			}
		}
		if az.Credential.ClientCredential != nil {
			_, _ = fmt.Fprintf(o.Out, "Unable to revoke client credential, removing reference from configuration")
		}

		// Remove the authorization record from the configuration
		cfg.Authorizations = append(cfg.Authorizations[:i], cfg.Authorizations[i+1:]...)
		return nil
	})
}

// authorizationName returns the name of the authorization to revoke
func (o *Options) authorizationName(cfg *config.Config) string {
	if o.Name != "" {
		return o.Name
	}
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Name == cfg.CurrentContext {
			return cfg.Contexts[i].Context.Authorization
		}
	}
	return "default"
}

// indexOfAuthorizationServer locates a named authorization record and the corresponding revocation endpoint
func indexOfAuthorizationServer(cfg *config.Config, name string) (int, string, error) {
	// First, make sure the authorization exists
	az := -1
	for i := range cfg.Authorizations {
		if cfg.Authorizations[i].Name == name {
			az = i
			break
		}
	}
	if az < 0 {
		return -1, "", fmt.Errorf("authorization does not exist: %s", name)
	}

	// We must find a context that explicitly associates it with a server: WE DO NOT WANT TO BE SENDING TOKENS TO THE WRONG PLACE.
	serverName := ""
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Context.Authorization == name {
			serverName = cfg.Contexts[i].Context.Server
			break
		}
	}
	if serverName == "" {
		return -1, "", fmt.Errorf("unable to find server for named authorization (no context association): %s", name)
	}

	// Find the revocation endpoint for the named server
	endpoint := ""
	for i := range cfg.Servers {
		if cfg.Servers[i].Name == serverName {
			endpoint = cfg.Servers[i].Server.Authorization.RevocationEndpoint
			break
		}
	}
	if endpoint == "" {
		return -1, "", fmt.Errorf("could not find server for authorization: %s", serverName)
	}

	return az, endpoint, nil
}

// revokeToken sends a POST request to the revocation endpoint with the supplied client ID and refresh token
func revokeToken(ctx context.Context, endpoint, clientID, refreshToken string) error {
	b, err := json.Marshal(&struct {
		ClientID string `json:"client_id"`
		Token    string `json:"token"`
	}{ClientID: clientID, Token: refreshToken})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctxhttp.Do(ctx, nil, req)
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("cannot read revocation response: %v", err)
	}

	if code := resp.StatusCode; code != http.StatusOK && code != http.StatusNoContent {
		return fmt.Errorf("revocation returned unexpected status: %v", code)
	}
	return nil
}
