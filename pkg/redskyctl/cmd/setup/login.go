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

package setup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"time"

	"github.com/pkg/browser"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	redskyclient "github.com/redskyops/k8s-experiment/redskyapi"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const (
	// loginSuccessURL is the URL where users are redirected after a successful login
	loginSuccessURL = "https://redskyops.dev/api/auth_success/"

	loginLong    = `Log into your Red Sky account`
	loginExample = ``
)

var (
	// codeChallengeMethodS256 is AuthCodeOption for specifying the S256 code challenge method
	codeChallengeMethodS256 oauth2.AuthCodeOption = oauth2.SetAuthURLParam("code_challenge_method", "S256")
)

// LoginOptions is the configuration for logging in
type LoginOptions struct {
	DisplayURL bool
	Force      bool

	ServerAddress string // NOTE: The server address must be whitelisted as a redirect URI by the authentication provider

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
	if o.ServerAddress == "" {
		o.ServerAddress = ":8085"
	}

	return nil
}

func (o *LoginOptions) Run() error {
	// Refuse to overwrite authentication unless forced
	if !o.Force {
		if config, err := redskyclient.DefaultConfig(); err == nil && config.Exists() {
			return fmt.Errorf("refusing to overwrite existing configuration without --force")
		}
	}

	// The redirect handler is responsible for handling redirects from the OAuth2 flow
	rh, err := newRedirectHandler()
	if err != nil {
		return nil
	}
	rh.OAuth2.RedirectURL = o.url().String()

	// Create a new serve mux to handle incoming requests
	router := http.NewServeMux()
	router.Handle("/", rh)

	// TODO This is bogus until we get a real auth provider
	ap := &authenticationProvider{}
	rh.OAuth2.Endpoint = *ap.register(o.url(), router)

	server := &http.Server{
		Addr:         o.ServerAddress,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// This context corresponds to the lifetime of the server, calling shutdown will cancel the context
	serve, shutdown := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// Make sure we can shutdown the server once the configuration is written out
	rh.ShutdownServer = func() {
		shutdown()
		// TODO Read back the configuration and print out something more informative
		_, _ = fmt.Fprintln(o.Out, "You are now logged in.")
	}

	// Start the server and a blocked shutdown routine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			done <- err
		}
	}()
	go func() {
		<-serve.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- server.Shutdown(ctx)
	}()

	// Add a signal handler so we shutdown cleanly on SIGINT/TERM
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		_, _ = fmt.Fprintln(o.Out)
		shutdown()
	}()

	// Try to connect to see if start up failed
	// TODO Do we need to retry this?
	conn, err := net.DialTimeout("tcp", o.ServerAddress, 2*time.Second)
	if err == nil {
		_ = conn.Close()
	}

	// Before opening the browser, check to see if there were any errors
	select {
	case err := <-done:
		return err
	default:
		if err := o.openBrowser(rh.authCodeURL()); err != nil {
			shutdown()
			return err
		}
	}
	return <-done
}

func (o *LoginOptions) url() *url.URL {
	loc := &url.URL{Scheme: "http", Host: o.ServerAddress, Path: "/"}
	if loc.Hostname() == "" {
		loc.Host = "localhost" + loc.Host
	}
	return loc
}

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

// redirectHandler is the endpoint for handling OAuth2 login redirects
type redirectHandler struct {
	// OAuth2 configuration
	OAuth2 oauth2.Config
	// ShutdownServer is called to shutdown the server at the end of the process
	ShutdownServer func()
	// state is a random value to prevent CSRF attacks
	state string
	// verifier is the PKCE code verifier generated for this login attempt
	verifier string
}

func newRedirectHandler() (*redirectHandler, error) {
	// Generate a random state for CSRF
	sb := make([]byte, 16)
	if _, err := rand.Read(sb); err != nil {
		return nil, err
	}
	s := base64.RawURLEncoding.EncodeToString(sb)

	// Generate a random verifier
	vb := make([]byte, 32)
	if _, err := rand.Read(vb); err != nil {
		return nil, err
	}
	v := base64.RawURLEncoding.EncodeToString(vb)

	return &redirectHandler{state: s, verifier: v}, nil
}

// authCodeURL returns the browser URL for the user to start the authentication flow
func (h *redirectHandler) authCodeURL() string {
	codeChallenge := fmt.Sprintf("%x", sha256.Sum256([]byte(h.verifier)))
	return h.OAuth2.AuthCodeURL(h.state, oauth2.SetAuthURLParam("code_challenge", codeChallenge), codeChallengeMethodS256)
}

// ServeHTTP will handle the `GET /?code=...` request by exchanging the authorization code for an access token and storing it to disk
func (h *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify the request (e.g. /favicon.ico)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If this is a request for redirect target, shutdown the server once we leave this method
	defer h.ShutdownServer()

	// Verify the state for CSRF
	if h.state != r.FormValue("state") {
		http.Error(w, "CSRF state mismatch", http.StatusForbidden)
		return
	}

	// Exchange the authorization code for an access token
	t, err := h.OAuth2.Exchange(context.TODO(), r.FormValue("code"), oauth2.SetAuthURLParam("code_verifier", h.verifier))
	if err != nil {
		// TODO Should we have a better error page, or maybe a redirect to a login troubleshooting doc page?
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save the token in the user's local configuration
	if err := persistToken(t); err != nil {
		http.Error(w, "Unexpected token type", http.StatusInternalServerError)
		return
	}

	// Redirect to the login success page
	http.Redirect(w, r, loginSuccessURL, http.StatusSeeOther)
}

// persistToken write the token to disk
func persistToken(t *oauth2.Token) error {
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
	config.Set("oauth2.client_id", accessToken["redsky_client_id"])
	config.Set("oauth2.client_secret", accessToken["redsky_client_secret"])

	// TODO How else do we get the endpoint identifier?

	// Write the configuration back to disk
	err = config.Write()
	if err != nil {
		return err
	}

	return nil
}
