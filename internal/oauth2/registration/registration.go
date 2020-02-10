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

// Package registration implements the OAuth 2.0 Dynamic Client Registration Protocol.
//
// This should be used when a client needs to dynamically or programmatically register
// with the authorization server.
//
// See https://tools.ietf.org/html/rfc7591
package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"

	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
)

// Config describes a client registration process, including the initial authorization and the server's endpoint URL.
type Config struct {
	// RegistrationURL is the URL of the client registration endpoint
	RegistrationURL string
	// InitialToken is an optional token used to make registration requests. If not specified, it may be necessary
	// to configure the HTTP client to perform the appropriate authorization out of the scope of this configuration.
	InitialToken *oauth2.Token
}

// ClientMetadata is the set of metadata values associated with the client.
type ClientMetadata struct {
	// RedirectURIs is the array of redirection URIs for use in redirect-based flows such as the authorization code and implicit flows.
	RedirectURIs []string `json:"redirect_uris"`
	// GrantTypes is the array of OAuth 2.0 grant type strings that the client case use at the token endpoint.
	GrantTypes []string `json:"grant_types"`
	// ResponseTypes is the array of OAuth 2.0 response type strings that the client use at the authorization endpoint.
	ResponseTypes []string `json:"response_types"`
	// ClientName is the human-readable string name of the client to be presented to the end-user during authorization.
	ClientName string `json:"client_name"`
}

// ClientInformationResponse contains the client identifier as well as the client secret, if the client is confidential client.
type ClientInformationResponse struct {
	// RegistrationClientURI is the string containing the fully qualified URL of the client configuration endpoint for this client.
	RegistrationClientURI string `json:"registration_client_uri"`
	// RegistrationAccessToken is the string containing the access token to be used at the client configuration endpoint.
	RegistrationAccessToken string `json:"registration_access_token"`

	// ClientID is the OAuth 2.0 client identifier string.
	ClientID string `json:"client_id"`
	// ClientSecret is the OAuth 2.0 client secret string.
	ClientSecret string `json:"client_secret,omitempty"`
	// ClientIDIssuedAt is the time at which the client identifier was issued.
	ClientIDIssuedAt int64 `json:"client_id_issued_at,omitempty"`
	// ClientSecretExpiresAt is the time at which the client secret will expire or 0 if it will not expire.
	ClientSecretExpiresAt int64 `json:"client_secret_expires_at,omitempty"`

	// ClientMetadata contains all of the registered metadata about the client.
	ClientMetadata
}

// ClientRegistrationErrorResponse is returned when an OAuth 2.0 error condition occurs.
type ClientRegistrationErrorResponse struct {
	// ErrorCode is the single ASCII error code string.
	ErrorCode string `json:"error"`
	// ErrorDescription is the human-readable ASCII test description of the error used for debugging.
	ErrorDescription string `json:"error_description,omitempty"`
}

// TODO Should we use the standard OAuth package errors instead?

// Error returns the string representation of the error response for debugging.
func (e *ClientRegistrationErrorResponse) Error() string {
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	return e.ErrorCode
}

// Register uses the initial access token to register a client.
func (c *Config) Register(ctx context.Context, client *ClientMetadata) (*ClientInformationResponse, error) {
	req, err := newRegistrationRequest(c.RegistrationURL, client)
	if err != nil {
		return nil, err
	}

	var src oauth2.TokenSource
	if c.InitialToken != nil {
		src = oauth2.StaticTokenSource(c.InitialToken)
	}

	info, err := doRegistrationRoundTrip(ctx, req, src)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// TODO Read, Update and Delete functions for RFC 7592

func newRegistrationRequest(registrationURL string, client *ClientMetadata) (*http.Request, error) {
	// TODO This should accept a `map[string]interface{}`
	b, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", registrationURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func doRegistrationRoundTrip(ctx context.Context, req *http.Request, src oauth2.TokenSource) (*ClientInformationResponse, error) {
	r, err := ctxhttp.Do(ctx, oauth2.NewClient(ctx, src), req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("registration: cannot fetch client information: %v", err)
	}

	if code := r.StatusCode; code < 200 || code > 299 {
		responseError := &ClientRegistrationErrorResponse{}
		if err := json.Unmarshal(body, responseError); err != nil {
			if t, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && t != "application/json" {
				return nil, fmt.Errorf("registration: invalid response body type; expected JSON, got: %s", t)
			}
			return nil, fmt.Errorf("registration: invalid response: %v", err)
		}
		if responseError.ErrorCode == "" {
			return nil, fmt.Errorf("registration: server error response missing error")
		}
		return nil, responseError
	} else if code != 201 {
		return nil, fmt.Errorf("registration: server response had unexpected status: %v", code)
	}

	// TODO We should separate out the JSON representation like the OAuth 2.0 codes does with tokenJSON
	info := &ClientInformationResponse{}
	if err := json.Unmarshal(body, info); err != nil {
		return nil, err
	}
	if info.ClientID == "" {
		return nil, fmt.Errorf("registration: server response missing client_id")
	}
	return info, nil
}
