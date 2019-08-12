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
	"bufio"
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/redskyops/k8s-experiment/pkg/version"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type OAuth2 struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`

	Token string `json:"token,omitempty"`
}

type Config struct {
	Address string  `json:"address,omitempty"`
	OAuth2  *OAuth2 `json:"oauth2,omitempty"`
}

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

func DefaultConfig() (*Config, error) {
	config := &Config{}

	p := ".redsky"
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home != "" {
		p = filepath.Join(home, p)
	}
	f, err := os.Open(p)
	if err == nil {
		if err = yaml.NewYAMLOrJSONDecoder(bufio.NewReader(f), 4096).Decode(config); err != nil {
			return nil, err
		}
		if err = f.Close(); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if config.Address == "" {
		config.Address = os.Getenv("REDSKY_ADDRESS")
	}

	if config.OAuth2 == nil {
		oauth2 := OAuth2{
			ClientID:     os.Getenv("REDSKY_OAUTH2_CLIENT_ID"),
			ClientSecret: os.Getenv("REDSKY_OAUTH2_CLIENT_SECRET"),
		}
		if oauth2.ClientID != "" && oauth2.ClientSecret != "" {
			config.OAuth2 = &oauth2
		}
	}

	if config.OAuth2 != nil && config.OAuth2.TokenURL == "" && config.OAuth2.ClientID != "" && config.OAuth2.ClientSecret != "" {
		config.OAuth2.TokenURL = os.Getenv("REDSKY_OAUTH2_TOKEN_URL")
		if config.OAuth2.TokenURL == "" {
			config.OAuth2.TokenURL = "./auth/token/"
		}
	}

	return config, nil
}

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

	return &httpClient{
		endpoint:  u,
		client:    *hc,
		userAgent: "RedSky/" + strings.TrimLeft(version.Version, "v"),
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
