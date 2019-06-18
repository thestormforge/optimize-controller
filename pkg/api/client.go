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

	"k8s.io/apimachinery/pkg/util/yaml"
)

type Config struct {
	Address string `json:"address,omitempty"`
}

type Client interface {
	URL(endpoint string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

func DefaultConfig() (*Config, error) {
	config := &Config{}

	p := ".cordelia"
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
		config.Address = os.Getenv("CORDELIA_ADDRESS")
	}

	return config, nil
}

func NewClient(cfg Config) (Client, error) {
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return &httpClient{
		endpoint: u,
		client:   http.Client{Timeout: 10 * time.Second},
	}, nil
}

type httpClient struct {
	endpoint *url.URL
	client   http.Client
}

func (c *httpClient) URL(ep string) *url.URL {
	p := path.Join(c.endpoint.Path, ep)
	u := *c.endpoint
	u.Path = p
	return &u
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
