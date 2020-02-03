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

// Package authorizationcode implements the OAuth 2.0 Authorization Code Grant with the PKCE extension.
//
// See https://tools.ietf.org/html/rfc7636
package authorizationcode

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

// AuthorizationCodeFlowWithPKCE implements an authorization code flow with proof key for code exchange.
type AuthorizationCodeFlowWithPKCE struct {
	// Config is the OAuth2 configuration to use for this authorization flow
	oauth2.Config

	// Audience is the URI identifying the target API
	Audience string
	// HandleToken receives the access token from the authorization flow
	HandleToken func(*oauth2.Token) error
	// GenerateResponse produces an HTTP response for the given status code
	GenerateResponse func(w http.ResponseWriter, r *http.Request, message string, code int)

	// state is a random value to prevent CSRF attacks
	state string
	// verifier is the PKCE code verifier generated for this login attempt
	verifier string
}

// NewAuthorizationCodeFlowWithPKCE creates a new authorization flow using the supplied OAuth2 configuration
func NewAuthorizationCodeFlowWithPKCE() (*AuthorizationCodeFlowWithPKCE, error) {
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

	return &AuthorizationCodeFlowWithPKCE{state: s, verifier: v}, nil
}

// AuthCodeURLWithPKCE returns the browser URL for the user to start the authorization flow
func (f *AuthorizationCodeFlowWithPKCE) AuthCodeURLWithPKCE() string {
	audience := oauth2.SetAuthURLParam("audience", f.Audience)
	sum256 := sha256.Sum256([]byte(f.verifier))
	codeChallenge := oauth2.SetAuthURLParam("code_challenge", base64.RawURLEncoding.EncodeToString(sum256[:]))
	codeChallengeMethod := oauth2.SetAuthURLParam("code_challenge_method", "S256")
	return f.AuthCodeURL(f.state, audience, codeChallenge, codeChallengeMethod)
}

// ExchangeWithPKCE returns the access token for the authorization flow
func (f *AuthorizationCodeFlowWithPKCE) ExchangeWithPKCE(ctx context.Context, code string) (*oauth2.Token, error) {
	codeVerifier := oauth2.SetAuthURLParam("code_verifier", f.verifier)
	return f.Exchange(ctx, code, codeVerifier)
}

// CallbackAddr returns the address of the callback server (i.e. the host of the OAuth redirect URL)
func (f *AuthorizationCodeFlowWithPKCE) CallbackAddr() (string, error) {
	u, err := url.Parse(f.RedirectURL)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

// Callback implements an HTTP handler for the target of the OAuth2 redirect URL
func (f *AuthorizationCodeFlowWithPKCE) Callback(w http.ResponseWriter, r *http.Request) {
	// Make sure this request matches the configuration
	if status, txt := f.validateRequest(r); status != http.StatusOK {
		f.respond(w, r, txt, status)
		return
	}

	// Exchange the authorization code for an access token
	token, err := f.ExchangeWithPKCE(context.TODO(), r.FormValue("code"))
	if err != nil {
		f.respond(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle the token
	if f.HandleToken != nil {
		if err := f.HandleToken(token); err != nil {
			f.respond(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Report success
	f.respond(w, r, "", http.StatusOK)
}

func (f *AuthorizationCodeFlowWithPKCE) validateRequest(r *http.Request) (int, string) {
	// Validate the request URL matches the configured redirect URL
	u, err := url.Parse(f.RedirectURL)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	if r.URL.Path != u.Path { // TODO Equality check on the whole URL?
		return http.StatusNotFound, ""
	}

	// If the path is correct, we only support the GET method
	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, ""
	}

	// Check the CSRF state
	if f.state != r.FormValue("state") {
		return http.StatusForbidden, "CSRF state mismatch"
	}

	// Check for errors
	if errorCode := r.FormValue("error"); errorCode != "" {
		if ed := r.FormValue("error_description"); ed != "" {
			errorCode = ed
		}
		return http.StatusInternalServerError, errorCode
	}

	return http.StatusOK, ""
}

func (f *AuthorizationCodeFlowWithPKCE) respond(w http.ResponseWriter, r *http.Request, message string, code int) {
	if f.GenerateResponse != nil {
		f.GenerateResponse(w, r, message, code)
	} else {
		http.Error(w, message, code)
	}
}
