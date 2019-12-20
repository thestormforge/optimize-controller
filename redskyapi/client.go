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

package redskyapi

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/redskyops/k8s-experiment/pkg/version"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

// ConfigureOAuth2 checks the supplied to configuration to see if the (possibly nil) transport needs to be wrapped to
// perform authentication. The context is used for token management if necessary.
func ConfigureOAuth2(cfg *Config, ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error) {

	// Client credential ("two-legged") token flow
	if cfg.OAuth2.ClientID != "" && cfg.OAuth2.ClientSecret != "" {
		cc := clientcredentials.Config{
			ClientID:     cfg.OAuth2.ClientID,
			ClientSecret: cfg.OAuth2.ClientSecret,
			AuthStyle:    oauth2.AuthStyleInParams,
		}

		// Resolve the token URL against the endpoint address
		endpoint, err := url.Parse(cfg.Address)
		if err != nil {
			return nil, err
		}
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/"
		tokenURL, err := endpoint.Parse(cfg.OAuth2.TokenURL)
		if err != nil {
			return nil, err
		}
		cc.TokenURL = tokenURL.String()

		return &oauth2.Transport{Source: cc.TokenSource(ctx), Base: transport}, nil
	}

	// Static token flow
	if cfg.OAuth2.Token != "" {
		sts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.OAuth2.Token})
		return &oauth2.Transport{Source: sts, Base: transport}, nil
	}

	// No OAuth
	return transport, nil
}

// GetAddress returns the URL representation of the address configuration parameter
func GetAddress(cfg *Config) (*url.URL, error) {
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	return u, nil
}

// TODO This should come from the externally configured round-tripper instead

var DefaultUserAgent string

func NewClient(cfg *Config, ctx context.Context, transport http.RoundTripper) (Client, error) {
	hc := &httpClient{}
	hc.client.Timeout = 10 * time.Second

	// Parse the API endpoint address and force a trailing slash
	var err error
	if hc.endpoint, err = url.Parse(cfg.Address); err != nil {
		return nil, err
	}
	hc.endpoint.Path = strings.TrimRight(hc.endpoint.Path, "/") + "/"

	// Configure the OAuth2 transport
	hc.client.Transport, err = ConfigureOAuth2(cfg, ctx, transport)
	if err != nil {
		return nil, err
	}

	// Set the User-Agent string
	hc.userAgent = DefaultUserAgent
	if hc.userAgent == "" {
		hc.userAgent = version.GetUserAgentString("RedSky")
	}

	return hc, nil
}

type httpClient struct {
	endpoint  *url.URL
	client    http.Client
	userAgent string
}

func (c *httpClient) URL(ep string) *url.URL {
	u := *c.endpoint
	u.Path = path.Join(u.Path, ep)
	return &u
}

func (c *httpClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var body []byte
	done := make(chan struct{})
	go func() {
		body, err = ioutil.ReadAll(resp.Body)
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		err = resp.Body.Close()
		if err == nil {
			err = ctx.Err()
		}
	case <-done:
	}

	return resp, body, err
}
