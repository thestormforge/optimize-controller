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
	"time"

	"github.com/redskyops/k8s-experiment/redskyapi/config"
)

// Config exposes the information for configuring a Red Sky Client
type Config interface {
	// ExperimentsURL returns a URL to the experiments API
	ExperimentsURL(path string) (*url.URL, error)

	// Authorize returns a transport that applies the authorization defined by this configuration. The
	// supplied context is used for any additional requests necessary to perform authentication. If this
	// configuration does not define any authorization details, the supplied transport may be returned
	// directly.
	Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error)

	// Marshal will write the data out
	Marshal() ([]byte, error)
	// TODO This should be in the redskyctl configuration interface
}

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

// TODO Get rid of this...
// DefaultConfig loads the mostly commonly used configuration
func DefaultConfig() (Config, error) {
	cc := &config.ClientConfig{}
	if err := cc.Load(); err != nil {
		return nil, err
	}
	return cc, nil
}

// NewClient returns a new client for accessing Red Sky APIs; the supplied context is used for authentication/authorization
// requests and the supplied transport (which may be nil in the case of the default transport) is used for all requests made
// to the API server.
func NewClient(cfg Config, ctx context.Context, transport http.RoundTripper) (Client, error) {
	var err error

	hc := &httpClient{config: cfg}
	hc.client.Timeout = 10 * time.Second

	// Configure the OAuth2 transport
	hc.client.Transport, err = cfg.Authorize(ctx, transport)
	if err != nil {
		return nil, err
	}

	// Make sure that we can ignore the error from ExperimentsURL
	_, err = cfg.ExperimentsURL("")
	if err != nil {
		return nil, err
	}

	return hc, nil
}

type httpClient struct {
	config Config
	client http.Client
}

func (c *httpClient) URL(ep string) *url.URL {
	u, _ := c.config.ExperimentsURL(ep)
	return u
}

func (c *httpClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
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
