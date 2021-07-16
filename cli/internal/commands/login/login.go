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

package login

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/user"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/mdp/qrterminal/v3"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	"github.com/thestormforge/optimize-go/pkg/oauth2/authorizationcode"
	"golang.org/x/oauth2"
)

const (
	browserPrompt = `Opening your default browser to visit:

	%s

`
	urlPrompt = `Go to the following link in your browser:

	%s

Enter verification code:

		%s

`
	qrPrompt = `Your verification code is:

		%s

If you are having problems scanning, use your browser to visit: %s
`
)

// Options is the configuration for creating new authorization entries in a configuration
type Options struct {
	// Config is the Optimize Configuration to modify
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Environment overrides the default execution environment
	Environment string
	// Server overrides the default server identifier
	Server string
	// Issuer overrides the default authorization server issuer
	Issuer string
	// DisplayURL triggers a device authorization grant with a simple verification prompt
	DisplayURL bool
	// DisplayQR triggers a device authorization grant and uses a QR code for the verification prompt
	DisplayQR bool
	// Force allows an existing authorization to be overwritten
	Force bool

	// shutdown is the context cancellation function used to shutdown the authorization code grant callback server
	shutdown context.CancelFunc
}

// NewCommand creates a new command for executing a login
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate",
		Long:  "Log into your StormForge Account.",

		PersistentPreRunE: commander.WithoutArgsE(o.LoadConfig),
		PreRun:            commander.StreamsPreRun(&o.IOStreams),
		RunE:              commander.WithContextE(o.login),
	}

	cmd.Flags().StringVar(&o.Config.Overrides.Context, "name", "", "name of the server configuration to authorize")
	_ = cmd.Flags().MarkDeprecated("name", "use --context instead")

	cmd.Flags().StringVar(&o.Environment, "env", "", "override the execution environment")
	cmd.Flags().StringVar(&o.Server, "server", "", "override the StormForge Optimize API server identifier")
	cmd.Flags().StringVar(&o.Issuer, "issuer", "", "override the authorization server identifier")
	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "display the URL instead of opening a browser")
	cmd.Flags().BoolVar(&o.DisplayQR, "qr", false, "display a QR code instead of opening a browser")
	cmd.Flags().BoolVar(&o.Force, "force", false, "overwrite existing configuration")

	_ = cmd.Flags().MarkHidden("env")
	_ = cmd.Flags().MarkHidden("server")
	_ = cmd.Flags().MarkHidden("issuer")

	return cmd
}

// complete fills in the default values
func (o *Options) complete() error {
	// Make sure the name is not blank
	if o.Config.Overrides.Context == "" {
		name := "default"
		if o.Server != "" {
			name = strings.ToLower(o.Server)
			name = strings.TrimPrefix(name, "http://")
			name = strings.TrimPrefix(name, "https://")
			name = strings.Trim(name, "/")
			name = strings.ReplaceAll(name, ".", "_")
			name = strings.ReplaceAll(name, "/", "_")
		}
		o.Config.Overrides.Context = name
	}

	// If the server is not blank, make sure it is a URL
	if o.Server != "" {
		if u, err := url.Parse(o.Server); err != nil {
			return fmt.Errorf("server must be a valid URL: %v", err)
		} else if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("server must be an 'https' URL")
		} else if u.Path == "/v1/" {
			_, _ = fmt.Fprintf(o.ErrOut, "Warning: Server URL has a path of '/v1/', StormForge API endpoints may not resolve correctly")
		}
	}

	// If the issuer is not blank, make sure it is a URL
	if o.Issuer != "" {
		if u, err := url.Parse(o.Issuer); err != nil {
			return fmt.Errorf("issuer must be a valid URL: %v", err)
		} else if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("issuer must be an 'https' URL")
		}
	}

	return nil
}

// LoadConfig is alternate configuration loader. This is a special case for the login command as it needs to inject
// new information into the configuration at load time.
func (o *Options) LoadConfig() error {
	if err := o.complete(); err != nil {
		return err
	}

	return o.Config.Load(func(cfg *config.OptimizeConfig) error {
		// Abuse "Update" to validate the configuration does not already have an authorization
		if err := cfg.Update(o.requireForceIfNameExists); err != nil {
			return err
		}

		// Set the execution environment name before saving the server roots
		if err := cfg.Update(config.SetExecutionEnvironment(o.Environment)); err != nil {
			return err
		}

		// We need to save the server in the loader so default values are loaded on top of them
		name := cfg.Overrides.Context
		srv := &config.Server{Identifier: o.Server, Authorization: config.AuthorizationServer{Issuer: o.Issuer}}
		if err := cfg.Update(config.SaveServer(name, srv, cfg.Environment())); err != nil {
			return err
		}

		// We need change the current context here to ensure the value is correct when we try to read the configuration out later
		if err := cfg.Update(config.ApplyCurrentContext(name, name, name, "")); err != nil {
			return err
		}

		return nil
	})
}

func (o *Options) login(ctx context.Context) error {
	// The user has requested we just show a URL
	if o.DisplayURL || o.DisplayQR {
		return o.runDeviceCodeFlow(ctx)
	}

	// Perform authorization using the system web browser and a local web server
	return o.runAuthorizationCodeFlow(ctx)
}

func (o *Options) runDeviceCodeFlow(ctx context.Context) error {
	az, err := o.Config.NewDeviceAuthorization()
	if err != nil {
		return err
	}
	az.Scopes = append(az.Scopes, "register:clients", "offline_access") // TODO Where or what do we want to do here?

	t, err := az.Token(ctx, o.generateValidatationRequest)
	if err != nil {
		return err
	}
	return o.takeOffline(t)
}

func (o *Options) runAuthorizationCodeFlow(ctx context.Context) error {
	// Create a new authorization code flow
	c, err := o.Config.NewAuthorization()
	if err != nil {
		return err
	}
	c.Scopes = append(c.Scopes, "register:clients", "offline_access") // TODO Where or what do we want to do here?
	c.RedirectURL = "http://127.0.0.1:8085/"

	// Create a context we can use to shutdown the server and the OAuth authorization code callback endpoint
	ctx, o.shutdown = context.WithCancel(ctx)
	handler := c.Callback(o.takeOffline, o.generateCallbackResponse)

	// Create a new server with some extra configuration
	server := commander.NewContextServer(ctx, handler,
		commander.WithServerOptions(configureCallbackServer(c)),
		commander.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		commander.HandleStart(func(string) error {
			return o.openBrowser(c.AuthCodeURLWithPKCE())
		}))

	// Start the server, this will block until someone calls 'o.shutdown' from above
	return server.ListenAndServe()
}

// requireForceIfNameExists is a configuration "change" that really just validates that there are no name conflicts
func (o *Options) requireForceIfNameExists(cfg *config.Config) error {
	if !o.Force {
		// NOTE: We do not require --force for server name conflicts so you can log into an existing configuration
		for i := range cfg.Authorizations {
			if cfg.Authorizations[i].Name == o.Config.Overrides.Context {
				az := &cfg.Authorizations[i].Authorization
				if az.Credential.TokenCredential != nil || az.Credential.ClientCredential != nil {
					return fmt.Errorf("refusing to update, use --force")
				}
			}
		}
	}
	return nil
}

// takeOffline records the token in the configuration and write the configuration to disk
func (o *Options) takeOffline(t *oauth2.Token) error {
	// Normally clients should consider the access token as opaque, however if the user does not have a namespace
	// there is nothing we can do with the access token (except get "not activated" errors) so we should at least check
	srv, err := config.CurrentServer(o.Config.Reader())
	if err != nil {
		return err
	}

	set, err := jwk.Fetch(srv.Authorization.JSONWebKeySetURI)
	if err != nil {
		return err
	}

	tokenBytes, err := jws.VerifyWithJWKSet([]byte(t.AccessToken), set, nil)
	if err != nil {
		return err
	}

	token, err := jwt.ParseBytes(
		tokenBytes,
		jwt.WithValidate(true),
	)
	if err != nil {
		return err
	}

	ns, ok := token.PrivateClaims()["https://carbonrelay.com/claims/namespace"]
	if !ok || ns.(string) == "default" || ns.(string) == "" {
		return fmt.Errorf("account is not activated")
	}

	if err := o.Config.Update(config.SaveToken(o.Config.Overrides.Context, t)); err != nil {
		return err
	}

	if err := o.Config.Write(); err != nil {
		return err
	}

	// TODO Print out something more informative e.g. "... as [xxx]." (we would need "openid" and "email" scopes to get an ID token)
	_, _ = fmt.Fprintf(o.Out, "You are now logged in.\n")

	return nil
}

// generateCallbackResponse generates an HTTP response for the OAuth callback
func (o *Options) generateCallbackResponse(w http.ResponseWriter, r *http.Request, status int, err error) {
	// We can ignore the error because we just created the server in the configuration
	srv, _ := config.CurrentServer(o.Config.Reader())

	switch status {
	case http.StatusOK:
		// Redirect the user to the successful login URL and shutdown the server
		http.Redirect(w, r, srv.Application.AuthSuccessEndpoint, http.StatusSeeOther)
		o.shutdown()
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		// Ignorable error codes, e.g. browser requests for '/favicon.ico'
		http.Error(w, http.StatusText(status), status)
	default:
		// TODO Better detection of this error
		if status == http.StatusInternalServerError && err != nil && err.Error() == "account is not activated" {
			http.Redirect(w, r, srv.Application.BaseURL, http.StatusSeeOther)
			_, _ = fmt.Fprintf(o.Out, "Your account is not activated.\n")
			o.shutdown()
			return
		}

		// TODO Redirect to a troubleshooting URL? Use the snake cased status text as the fragment (e.g. '...#internal-server-error')?
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = http.StatusText(status)
		}
		http.Error(w, msg, status)

		// TODO Print the actual error message?
		_, _ = fmt.Fprintf(o.Out, "An error occurred, please try again.\n")

		o.shutdown()
	}
}

// generateValidatationRequest generates a validation request to the command output stream
func (o *Options) generateValidatationRequest(userCode, verificationURI, verificationURIComplete string) {
	if o.DisplayQR {
		qrterminal.Generate(verificationURIComplete, qrterminal.L, o.Out)
		_, _ = fmt.Fprintf(o.Out, qrPrompt, userCode, verificationURI)
		return
	}

	_, _ = fmt.Fprintf(o.Out, urlPrompt, verificationURI, userCode)
}

// openBrowser prints the supplied URL and possibly opens a web browser pointing to that URL
func (o *Options) openBrowser(loc string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Do not open the browser for root, but assume they still have one
	if u.Uid == "0" {
		_, _ = fmt.Fprintf(o.Out, "%s\n", loc)
		return nil
	}

	// Print the URL and open it
	_, _ = fmt.Fprintf(o.Out, browserPrompt, loc)
	if err := browser.OpenURL(loc); err != nil {
		return fmt.Errorf("failed to open browser, use 'stormforge login --url' instead")
	}

	return nil
}

// configureCallbackServer configures an HTTP server using the supplied callback redirect URL for the listen address
func configureCallbackServer(c *authorizationcode.Config) func(srv *http.Server) {
	return func(srv *http.Server) {
		// Try to make the server listen on the same host as the callback
		if addr, err := c.CallbackAddr(); err == nil {
			srv.Addr = addr
		}

		// Adjust timeouts
		srv.ReadTimeout = 5 * time.Second
		srv.WriteTimeout = 10 * time.Second
		srv.IdleTimeout = 15 * time.Second
	}
}
