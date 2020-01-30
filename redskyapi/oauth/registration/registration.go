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

package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
)

// This is a partial Dynamic Client Registration (RFC 7591 + 7592) implementation

type Config struct {
	// RegistrationURL is the URL of the client registration endpoint
	RegistrationURL string
	// InitialToken is an optional token used to make registration requests. If not specified, it may be necessary
	// to configure the HTTP client to perform the appropriate authorization out of the scope of this configuration.
	InitialToken *oauth2.Token
}

type ClientMetadata struct {
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types"`
	ResponseTypes []string `json:"response_types"`
	ClientName    string   `json:"client_name"`
}

type ClientInformationResponse struct {
	RegistrationClientURI   string `json:"registration_client_uri"`
	RegistrationAccessToken string `json:"registration_access_token"`
	ClientID                string `json:"client_id"`
	ClientSecret            string `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64  `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64  `json:"client_secret_expires_at,omitempty"`
	ClientMetadata
}

type ClientRegistrationErrorResponse struct {
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *ClientRegistrationErrorResponse) Error() string {
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	return e.ErrorCode
}

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
	// TODO Check content type?
	if code := r.StatusCode; code > 399 || code < 500 {
		responseError := &ClientRegistrationErrorResponse{}
		if err := json.Unmarshal(body, responseError); err != nil {
			return nil, err
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
