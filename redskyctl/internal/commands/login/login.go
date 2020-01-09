/*
Copyright 2019 GramLabs, Inc.

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

package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os/user"
	"time"

	"github.com/pkg/browser"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	redskyclient "github.com/redskyops/k8s-experiment/redskyapi"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	// OAuthProfile controls with OAuth configuration to use
	// TODO As a temporary hack, we actually use the callback server as an OAuth provider
	OAuthProfile = "http://localhost:8085/"

	// LoginSuccessURL is the URL where users are redirected after a successful login
	LoginSuccessURL = "https://redskyops.dev/api/auth_success/"
)

const (
	loginLong    = `Log into your Red Sky account`
	loginExample = ``
)

// LoginOptions is the configuration for logging in
type LoginOptions struct {
	DisplayURL bool
	Force      bool

	cmdutil.IOStreams
}

// NewLoginOptions returns a new login options struct
func NewLoginOptions(ioStreams cmdutil.IOStreams) *LoginOptions {
	return &LoginOptions{
		IOStreams: ioStreams,
	}
}

func NewLoginCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewLoginOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Authenticate",
		Long:    loginLong,
		Example: loginExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "Display the URL instead of opening a browser.")
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite existing configuration")

	return cmd
}

func (o *LoginOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	return nil
}

func (o *LoginOptions) Run() error {
	// Refuse to overwrite authentication unless forced
	if !o.Force {
		if config, err := redskyclient.DefaultConfig(); err == nil && (config.OAuth2.ClientID != "" || config.OAuth2.ClientSecret != "") {
			return fmt.Errorf("refusing to overwrite existing configuration without --force")
		}
	}

	// Use the OAuth configuration to create a new authorization flow with PKCE
	config := NewOAuthConfig(OAuthProfile)
	if config == nil {
		return fmt.Errorf("unknown OAuth profile: %s", OAuthProfile)
	}
	config.RedirectURL = "http://localhost:8085/"
	config.Scopes = []string{"offline_access"}
	flow, err := NewAuthorizationCodeFlowWithPKCE(config)
	if err != nil {
		return err
	}

	// Configure the flow to persist the access token for offline use and generate the appropriate callback responses
	flow.Audience = "https://api.carbonrelay.dev/"
	flow.HandleToken = o.takeOffline
	flow.GenerateResponse = o.generateCallbackResponse

	// TODO The flow for not launching a browser is actually somewhat different then just showing the URL:
	// 1. We do not need a local webserver at all
	// 2. We need to interactively prompt for the authorization code
	// 3. The backend needs a special redirect URL that displays the authorization code in the browser instead of redirecting
	// IS THIS AN AUTH0 "Device Flow" (e.g. a TV app authorization)

	// TODO This is bogus until we get a real auth provider
	ap := &authenticationProvider{}

	// Create a new server that will be shutdown when the authorization flow completes
	server := cmdutil.NewContextServer(serverShutdownContext(flow), ap.handler(flow.Callback),
		cmdutil.WithServerOptions(configureCallbackServer(flow)),
		cmdutil.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		cmdutil.HandleStart(func(string) error {
			return o.openBrowser(flow.AuthCodeURL())
		}))

	return server.ListenAndServe()
}

// takeOffline caches the access token for offline use
func (o *LoginOptions) takeOffline(t *oauth2.Token) error {
	// For now, the token is kind of a dummy.
	if t.TokenType != "dummy" {
		return fmt.Errorf("unexpected token type")
	}

	// The dummy access token is just the base64 encoded JSON with the client ID and secret
	data, err := base64.URLEncoding.DecodeString(t.AccessToken)
	if err != nil {
		return err
	}
	accessToken := make(map[string]string)
	if err := json.Unmarshal(data, &accessToken); err != nil {
		return err
	}

	// Load the Red Sky configuration to persist the changes
	config, err := redskyclient.DefaultConfig()
	if err != nil {
		return err
	}
	_ = config.Set("oauth2.client_id", accessToken["redsky_client_id"])
	_ = config.Set("oauth2.client_secret", accessToken["redsky_client_secret"])

	// Write the configuration back to disk
	err = config.Write()
	if err != nil {
		return err
	}

	// TODO Print out something more informative
	_, _ = fmt.Fprintln(o.Out, "You are now logged in.")

	return nil
}

// generateCallbackResponse generates an HTTP response for the OAuth callback
func (o *LoginOptions) generateCallbackResponse(w http.ResponseWriter, r *http.Request, message string, code int) {
	// TODO Redirect to a troubleshooting page for internal server errors
	if code == http.StatusOK {
		http.Redirect(w, r, LoginSuccessURL, http.StatusSeeOther)
	} else if message == "" {
		http.Error(w, http.StatusText(code), code)
	} else {
		http.Error(w, message, code)
	}
}

// openBrowser prints the supplied URL and possibly opens a web browser pointing to that URL
func (o *LoginOptions) openBrowser(loc string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Do not open the browser for root
	if o.DisplayURL || u.Uid == "0" {
		_, _ = fmt.Fprintf(o.Out, "%s\n", loc)
		return nil
	}

	_, _ = fmt.Fprintf(o.Out, "Opening your default browser to visit:\n\n\t%s\n\n", loc)
	return browser.OpenURL(loc)
}

// configureCallbackServer configures an HTTP server using the supplied callback redirect URL for the listen address
func configureCallbackServer(f *AuthorizationCodeFlowWithPKCE) func(srv *http.Server) {
	return func(srv *http.Server) {
		// Try to make the server listen on the same host as the callback
		if addr, err := f.CallbackAddr(); err == nil {
			srv.Addr = addr
		}

		// Adjust timeouts
		srv.ReadTimeout = 5 * time.Second
		srv.WriteTimeout = 10 * time.Second
		srv.IdleTimeout = 15 * time.Second
	}
}

// serverShutdownContext wraps the response generator of the supplied flow to cancel the returned context.
// This is effectively the code that shuts down the HTTP server once the OAuth callback is hit.
func serverShutdownContext(f *AuthorizationCodeFlowWithPKCE) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	// Wrap GenerateResponse so it cancels the context on success or server failure
	genResp := f.GenerateResponse
	f.GenerateResponse = func(w http.ResponseWriter, r *http.Request, message string, code int) {
		if genResp != nil {
			genResp(w, r, message, code)
		}
		if code == http.StatusOK || code == http.StatusInternalServerError {
			cancel()
		}
	}

	return ctx
}
