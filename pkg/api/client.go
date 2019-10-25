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

package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/redskyops/k8s-experiment/pkg/version"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

func DefaultConfig() (*viper.Viper, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("oauth2.token_url", "./auth/token/")

	// Environment variables
	_ = v.BindEnv("address", "REDSKY_ADDRESS")
	_ = v.BindEnv("oauth2.client_id", "REDSKY_OAUTH2_CLIENT_ID")
	_ = v.BindEnv("oauth2.client_secret", "REDSKY_OAUTH2_CLIENT_SECRET")
	_ = v.BindEnv("oauth2.token_url", "REDSKY_OAUTH2_TOKEN_URL")

	// Configuration on disk
	// TODO ~/.config/redskyops/??? ~/.redskyops/config???
	v.SetConfigType("yaml")
	v.SetConfigFile(os.ExpandEnv("$HOME/.redsky"))

	// Read the configuration
	if err := v.ReadInConfig(); ignoreConfigFileNotFound(err) != nil {
		return nil, err
	}
	return v, nil
}

func ignoreConfigFileNotFound(err error) error {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return nil
	}
	// `SetConfigFile` bypasses the `ConfigFileNotFoundError` logic
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// TODO This should come from the externally configured round-tripper instead

var DefaultUserAgent string

func NewClient(cfg *viper.Viper, ctx context.Context, transport http.RoundTripper) (Client, error) {
	hc := &httpClient{}
	hc.client.Transport = transport
	hc.client.Timeout = 10 * time.Second

	// Parse the API endpoint address and force a trailing slash
	var err error
	if hc.endpoint, err = url.Parse(cfg.GetString("address")); err != nil {
		return nil, err
	}
	hc.endpoint.Path = strings.TrimRight(hc.endpoint.Path, "/") + "/"

	// Set the User-Agent string
	hc.userAgent = DefaultUserAgent
	if hc.userAgent == "" {
		hc.userAgent = version.GetUserAgentString("RedSky")
	}

	// Configure OAuth2
	if cfg.IsSet("oauth2.client_id") && cfg.IsSet("oauth2.client_secret") {
		// Client credential ("two-legged") token flow
		cc := clientcredentials.Config{
			ClientID:     cfg.GetString("oauth2.client_id"),
			ClientSecret: cfg.GetString("oauth2.client_secret"),
			AuthStyle:    oauth2.AuthStyleInParams,
		}

		// Resolve the token URL against the endpoint address
		tokenURL, err := hc.endpoint.Parse(cfg.GetString("oauth2.token_url"))
		if err != nil {
			return nil, err
		}
		cc.TokenURL = tokenURL.String()

		hc.client.Transport = &oauth2.Transport{Source: cc.TokenSource(ctx), Base: hc.client.Transport}
	} else if cfg.IsSet("oauth2.token") {
		// Static token flow
		sts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.GetString("oauth2.token")})

		hc.client.Transport = &oauth2.Transport{Source: sts, Base: hc.client.Transport}
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
