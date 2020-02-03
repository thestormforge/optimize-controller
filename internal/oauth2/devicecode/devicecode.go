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

// Package devicecode implements the OAuth 2.0 Device Authorization Grant.
//
// See https://tools.ietf.org/html/rfc8628
package devicecode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// DeviceFlow describes a device authorization grant (also known as a "device flow").
type DeviceFlow struct {
	// Config is the OAuth2 configuration to use for this authorization flow
	clientcredentials.Config

	// DeviceAuthorizationURL is the location of the device authorization endpoint
	DeviceAuthorizationURL string
	// Audience is the URI identifying the target API
	Audience string
	// HandleToken receives the access token from the authorization flow
	HandleToken func(*oauth2.Token) error
	// GenerateResponse is used to display the verification information to the user
	GenerateResponse func(userCode, verificationURI, verificationURIComplete string)
}

type deviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval,omitempty"`
}

type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// TODO Should this be `Token(context.Context, func(string,string,string)) (*Token, error)` instead?

// Authorize uses the device flow to retrieve a token.
func (df *DeviceFlow) Authorize(ctx context.Context) error {
	// Get the device code
	v := url.Values{
		"client_id": {df.ClientID},
		"audience":  {df.Audience}, // TODO Should audience come in via the endpoint parameters?
	}
	if len(df.Scopes) > 0 {
		v.Set("scope", strings.Join(df.Scopes, " "))
	}
	for k, p := range df.EndpointParams {
		v[k] = p
	}
	req, err := newDeviceAuthorizationRequest(df.DeviceAuthorizationURL, v)
	if err != nil {
		return err
	}
	dar, err := doDeviceAuthorizationRoundTrip(ctx, req)
	if err != nil {
		return err
	}

	// Request verification from the user
	df.GenerateResponse(dar.UserCode, dar.VerificationURI, dar.VerificationURIComplete)

	// Wait for the response to come back
	t, err := requestToken(ctx, df.Config, dar.DeviceCode, time.Duration(dar.Interval)*time.Second)
	if err != nil {
		return err
	}

	// Invoke the token handler
	return df.HandleToken(t)
}

func newDeviceAuthorizationRequest(deviceAuthorizationURL string, v url.Values) (*http.Request, error) {
	req, err := http.NewRequest("POST", deviceAuthorizationURL, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func doDeviceAuthorizationRoundTrip(ctx context.Context, req *http.Request) (*deviceAuthorizationResponse, error) {
	r, err := ctxhttp.Do(ctx, oauth2.NewClient(ctx, nil), req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("device: cannot fetch device authorization: %v", err)
	}
	if code := r.StatusCode; code < 200 || code > 299 {
		return nil, fmt.Errorf("device: cannot fetch device authorization: %v", code)
	}

	dar := &deviceAuthorizationResponse{}
	if err := json.Unmarshal(body, dar); err != nil {
		return nil, err
	}
	if dar.DeviceCode == "" {
		return nil, fmt.Errorf("device: server response missing device_code")
	}
	if dar.UserCode == "" {
		return nil, fmt.Errorf("device: server response missing user_code")
	}
	if dar.VerificationURI == "" {
		return nil, fmt.Errorf("device: server response missing verification_uri")
	}
	return dar, nil
}

func requestToken(ctx context.Context, cfg clientcredentials.Config, deviceCode string, interval time.Duration) (*oauth2.Token, error) {
	if cfg.EndpointParams == nil {
		cfg.EndpointParams = url.Values{}
	}
	cfg.EndpointParams.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	cfg.EndpointParams.Set("device_code", deviceCode)
	ts := cfg.TokenSource(ctx)
	for {
		t, err := ts.Token()
		if err != nil {
			if err := handleDeviceAccessTokenResponse(err, &interval); err != nil {
				return nil, err
			}
			continue
		}
		return t, nil
	}
}

func handleDeviceAccessTokenResponse(err error, interval *time.Duration) error {
	if rErr, ok := err.(*oauth2.RetrieveError); ok {
		errResp := &errorResponse{}
		if err := json.Unmarshal(rErr.Body, errResp); err != nil {
			return err
		}

		if errResp.Error == "slow_down" {
			*interval = *interval + (5 * time.Second)
		}

		if errResp.Error == "authorization_pending" || errResp.Error == "slow_down" {
			time.Sleep(*interval)
			return nil
		}
	}
	return err
}
