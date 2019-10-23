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

type OAuth2 struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`

	Token string `json:"token,omitempty"`
}

type Manager struct {
	Environment map[string]string `json:"env,omitempty"`
	// TODO Have an option for not including the local config?
}

type Config struct {
	Address string   `json:"address,omitempty"`
	OAuth2  *OAuth2  `json:"oauth2,omitempty"`
	Manager *Manager `json:"manager,omitempty"`
}

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

func NewConfig(v *viper.Viper) *Config {
	config := &Config{
		Address: v.GetString("address"),
	}

	if v.IsSet("oauth2.client_id") || v.IsSet("oauth2.client_secret") {
		config.OAuth2 = &OAuth2{
			ClientID:     v.GetString("oauth2.client_id"),
			ClientSecret: v.GetString("oauth2.client_secret"),
			TokenURL:     v.GetString("oauth2.token_url"),
		}
	}

	if v.IsSet("manager.env") {
		config.Manager = &Manager{
			Environment: v.GetStringMapString("manager.env"),
		}
	}

	return config
}

func DefaultConfig() (*Config, error) {
	v := viper.New()
	v.SetDefault("oauth2.token_url", "./auth/token")

	// Get configuration from environment variables
	// TODO Switch to explicit binding
	v.SetEnvPrefix("redsky")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Get configuration from disk
	// TODO ~/.config/redskyops/??? ~/.redskyops/config???
	v.SetConfigType("yaml")
	v.SetConfigFile(os.ExpandEnv("$HOME/.redsky"))

	// Read the configuration and convert to a typed configuration object
	// TODO Get rid of the structs and just pass the Viper around
	if err := v.ReadInConfig(); ignoreConfigFileNotFound(err) != nil {
		return nil, err
	}
	return NewConfig(v), nil
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

// TODO Should this be `NewClient(*viper.Viper, context.Context, http.RoundTripper)`?

func NewClient(cfg Config) (Client, error) {
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}

	// Force a trailing slash before calling URL.ResolveReference to better meet user expectations
	u.Path = strings.TrimRight(u.Path, "/") + "/"

	var hc *http.Client
	if cfg.OAuth2 != nil {
		ctx := context.TODO()
		if cfg.OAuth2.TokenURL != "" && cfg.OAuth2.ClientID != "" && cfg.OAuth2.ClientSecret != "" {
			// Client credential ("two-legged") token flow
			t, err := url.Parse(cfg.OAuth2.TokenURL)
			if err != nil {
				return nil, err
			}

			c := clientcredentials.Config{
				ClientID:     cfg.OAuth2.ClientID,
				ClientSecret: cfg.OAuth2.ClientSecret,
				TokenURL:     u.ResolveReference(t).String(),
				AuthStyle:    oauth2.AuthStyleInParams,
			}

			hc = c.Client(ctx)
		} else if cfg.OAuth2.Token != "" {
			// Static token flow
			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.OAuth2.Token})
			hc = oauth2.NewClient(ctx, ts)
		}
	}

	// Default client
	if hc == nil {
		hc = &http.Client{}
	}

	// Use an explicit timeout
	hc.Timeout = 10 * time.Second

	// Strip the trailing slash back out so it isn't displayed
	u.Path = strings.TrimRight(u.Path, "/")

	// Make sure we have a User-Agent string
	ua := DefaultUserAgent
	if ua == "" {
		ua = version.GetUserAgentString("RedSky")
	}

	return &httpClient{
		endpoint:  u,
		client:    *hc,
		userAgent: ua,
	}, nil
}

type httpClient struct {
	endpoint  *url.URL
	client    http.Client
	userAgent string
}

func (c *httpClient) URL(ep string) *url.URL {
	p := path.Join(c.endpoint.Path, ep)
	u := *c.endpoint
	u.Path = p
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
