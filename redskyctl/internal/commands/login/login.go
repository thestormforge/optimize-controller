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
	"fmt"
	"net/http"
	"os/user"
	"strings"
	"time"

	"github.com/pkg/browser"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/redskyapi/config"
	"github.com/redskyops/k8s-experiment/redskyapi/oauth"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// TODO Configure these via LDFLAGS appropriate for dev/prod
var (
	// SuccessURL is the URL where users are redirected after a successful login
	SuccessURL = "https://redskyops.dev/api/auth_success/"

	// ClientID is the identifier for the Red Sky Control application
	ClientID = ""
)

const (
	loginLong    = `Log into your Red Sky account`
	loginExample = ``
)

// LoginOptions is the configuration for logging in
type LoginOptions struct {
	DisplayURL bool
	Force      bool
	Name       string
	Server     string

	cfg config.ClientConfig
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
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite existing configuration.")

	cmd.Flags().StringVar(&o.Name, "name", "", "Name of the server configuration to authorize.")
	cmd.Flags().StringVar(&o.Server, "server", "", "Override the Red Sky API server identifier.")

	cmd.Flags().StringVar(&o.cfg.Filename, "redskyconfig", "", "Set the file used to store the Red Sky client configuration.")

	return cmd
}

func (o *LoginOptions) Complete(cmdutil.Factory, *cobra.Command, []string) error {
	if o.Name == "" {
		o.Name = "default"
		if o.Server != "" {
			o.Name = strings.ToLower(o.Server)
			o.Name = strings.TrimPrefix(o.Name, "http://")
			o.Name = strings.TrimPrefix(o.Name, "https://")
			o.Name = strings.ReplaceAll(o.Name, "/", "_")
		}
	}

	return nil
}

func (o *LoginOptions) Run() error {
	// Load the exiting client configuration
	if err := o.cfg.Load(o.loginConfig); err != nil {
		return err
	}

	// Create a new authorization code flow
	az, err := o.cfg.NewAuthorization()
	if err != nil {
		return err
	}
	az.HandleToken = o.takeOffline
	az.GenerateResponse = o.generateCallbackResponse
	az.ClientID = ClientID
	az.Scopes = append(az.Scopes, "offline_access") // TODO Where or what do we want to do here?
	az.RedirectURL = "http://localhost:8085/"

	// TODO Device code flow does not require server...
	if o.DisplayURL {
		//az.RedirectURL = "" // TODO This should be the device code URI
	}

	// Create a new server that will be shutdown when the authorization flow completes
	server := cmdutil.NewContextServer(serverShutdownContext(az), http.HandlerFunc(az.Callback),
		cmdutil.WithServerOptions(configureCallbackServer(az)),
		cmdutil.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		cmdutil.HandleStart(func(string) error {
			return o.openBrowser(az.AuthCodeURLWithPKCE())
		}))
	return server.ListenAndServe()
}

// loginConfig applies the login configuration
func (o *LoginOptions) loginConfig(cfg *config.ClientConfig) error {
	if err := cfg.Update(o.requireForceIfNameExists); err != nil {
		return err
	}
	if err := cfg.Update(config.SaveServer(o.Name, &config.Server{Identifier: o.Server})); err != nil {
		return err
	}
	if err := cfg.Update(config.ApplyCurrentContext(o.Name, o.Name, o.Name, "")); err != nil {
		return err
	}
	return nil
}

// requireForceIfNameExists is a configuration "change" that really just validates that there are no name conflicts
func (o *LoginOptions) requireForceIfNameExists(cfg *config.Config) error {
	if !o.Force {
		// NOTE: We do not require --force for server name conflicts so you can log into an existing configuration
		for i := range cfg.Authorizations {
			if cfg.Authorizations[i].Name == o.Name {
				return fmt.Errorf("refusing to update, use --force")
			}
		}
	}
	return nil
}

// takeOffline records the token in the configuration and write the configuration to disk
func (o *LoginOptions) takeOffline(t *oauth2.Token) error {
	if err := o.cfg.Update(config.SaveToken(o.Name, t)); err != nil {
		return err
	}
	if err := o.cfg.Write(); err != nil {
		return err
	}
	return nil
}

// generateCallbackResponse generates an HTTP response for the OAuth callback
func (o *LoginOptions) generateCallbackResponse(w http.ResponseWriter, r *http.Request, message string, status int) {
	// TODO Redirect to a troubleshooting page for internal server errors
	if status == http.StatusOK {
		http.Redirect(w, r, SuccessURL, http.StatusSeeOther)
		// TODO Print out something more informative
		_, _ = fmt.Fprintln(o.Out, "You are now logged in.")
	} else {
		msg := message
		if msg == "" {
			msg = http.StatusText(status)
		}
		http.Error(w, message, status)
		if isStatusTerminal(status) {
			_, _ = fmt.Fprintln(o.Out, "An error occurred, please try again.")
		}
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
func configureCallbackServer(f *oauth.AuthorizationCodeFlowWithPKCE) func(srv *http.Server) {
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
func serverShutdownContext(f *oauth.AuthorizationCodeFlowWithPKCE) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	// Wrap GenerateResponse so it cancels the context on success or server failure
	genResp := f.GenerateResponse
	f.GenerateResponse = func(w http.ResponseWriter, r *http.Request, message string, status int) {
		if genResp != nil {
			genResp(w, r, message, status)
		}

		if isStatusTerminal(status) {
			cancel()
		}
	}

	return ctx
}

// isStatusTerminal checks to see if the status indicates that it is time to shutdown the server
func isStatusTerminal(status int) bool {
	return status != http.StatusNotFound && status != http.StatusMethodNotAllowed
}
