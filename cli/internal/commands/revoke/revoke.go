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

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	"golang.org/x/net/context/ctxhttp"
)

// Options is the configuration for removing authorization entries in a configuration
type Options struct {
	// Config is the Optimize Configuration to modify
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

// NewCommand creates a new command for executing a logout
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "revoke",
		Short:   "Revoke an authorization",
		Long:    "Log out of your StormForge Account.",
		Aliases: []string{"logout"},

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.revoke),
	}

	cmd.Flags().StringVar(&o.Config.Overrides.Context, "name", "", "name of the server configuration to de-authorize")
	_ = cmd.Flags().MarkDeprecated("name", "use --context instead")

	return cmd
}

// Run executes the revocation
func (o *Options) revoke(ctx context.Context) error {
	ri, err := o.Config.RevocationInfo()
	if err != nil {
		return err
	}

	if ri.Authorization.Credential.TokenCredential != nil {
		if err := revokeToken(ctx, ri.RevocationURL, ri.ClientID, ri.Authorization.Credential.RefreshToken); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(o.Out, "Revoked credential '%s'.\n", ri)
	}
	if ri.Authorization.Credential.ClientCredential != nil {
		_, _ = fmt.Fprintf(o.Out, "Unable to revoke client credential '%s', removing reference from configuration", ri)
	}

	if err := o.Config.Update(ri.RemoveAuthorization()); err != nil {
		return err
	}

	if err := o.Config.Write(); err != nil {
		return err
	}

	return nil
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
