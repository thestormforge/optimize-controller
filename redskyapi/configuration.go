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
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/yaml"
)

// OAuth2 is the authentication configuration for the Red Sky API
type OAuth2 struct {
	// ClientID is used to authenticate for a client credentials grant
	ClientID string `json:"client_id,omitempty"`
	// ClientSecret is used to authenticate for a client credentials grant
	ClientSecret string `json:"client_secret,omitempty"`
	// TokenURL is used to obtain an access token (defaults to `./auth/token/` relative to the server address)
	TokenURL string `json:"token_url,omitempty"`

	// Token is the static access token to use instead of the client credential grant
	Token string `json:"token,omitempty"`
}

// ManagerEnvVar is a single environment variable to expose to the manager
type ManagerEnvVar struct {
	// Name is the case-sensitive name of the environment variable
	Name string `json:"name"`
	// Value is the environment variable value
	Value string `json:"value"`
}

// Manager is additional configuration for the Red Sky Manager program
type Manager struct {
	// Environment is list of additional environment variables to expose to the manager
	Environment []ManagerEnvVar `json:"env,omitempty"`
}

// Config is the client configuration information
type Config struct {
	// Filename is the full path to the configuration file (defaults to ~/.redsky)
	Filename string `json:"-"`
	// Address is the fully qualified URL of the Red Sky API
	Address string `json:"address,omitempty"`
	// OAuth2 is the authentication configuration for the Red Sky API
	OAuth2 OAuth2 `json:"oauth2,omitempty"`
	// Manager is additional configuration for the Red Sky Manager program
	Manager Manager `json:"manager,omitempty"`
}

// DefaultConfig creates a new configuration
func DefaultConfig() (*Config, error) {
	c := &Config{}

	c.Filename = filepath.Join(os.Getenv("HOME"), ".redsky")

	if err := c.Read(); err != nil {
		return nil, err
	}

	c.LoadEnv()

	if err := c.Complete(); err != nil {
		return nil, err
	}

	return c, nil
}

// Read unmashals the configuration from disk using the filename field
func (c *Config) Read() error {
	if c.Filename == "" {
		return nil
	}

	f, err := os.Open(c.Filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err = yaml2.NewYAMLOrJSONDecoder(bufio.NewReader(f), 4096).Decode(c); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	return nil
}

// Write marshals the configuration to disk using the filename field
func (c *Config) Write() error {
	if c.Filename == "" {
		return nil
	}

	output, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(c.Filename, output, 0644)
	return err
}

// LoadEnv overwrites values in the configuration using environment variables
func (c *Config) LoadEnv() {
	if v, ok := os.LookupEnv("REDSKY_ADDRESS"); ok {
		c.Address = v
	}

	if v, ok := os.LookupEnv("REDSKY_OAUTH2_CLIENT_ID"); ok {
		c.OAuth2.ClientID = v
	}

	if v, ok := os.LookupEnv("REDSKY_OAUTH2_CLIENT_SECRET"); ok {
		c.OAuth2.ClientSecret = v
	}

	if v, ok := os.LookupEnv("REDSKY_OAUTH2_TOKEN_URL"); ok {
		c.OAuth2.TokenURL = v
	}
}

// Complete fills in default values and normalizes values
func (c *Config) Complete() error {
	// Force a trailing slash on the address
	u, err := url.Parse(c.Address)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	c.Address = u.String()

	// Provide a default token URL (relative to address)
	if c.OAuth2.TokenURL == "" {
		c.OAuth2.TokenURL = "./auth/token/"
	}

	return nil
}

// Set attempts to set a configuration value using a JSON Path expression
func (c *Config) Set(key, value string) error {
	// Special handling for the manager environment variables so they look like a name/value map
	if strings.HasPrefix(key, "manager.env.") {
		name := strings.TrimPrefix(key, "manager.env.")
		c.Manager.Environment = setEnvVar(c.Manager.Environment, name, value)
		return nil
	}

	// Special handling for the client ID to extract the host name
	if key == "oauth2.client_id" {
		parts := strings.SplitN(value, ".", 2)
		if len(parts) > 1 {
			value = parts[0]
			c.Address = fmt.Sprintf("https://%s/", parts[1])
		}
	}

	jp := jsonpath.New("config")
	// TODO Instead of always adding the braces and dot, check to see if they are needed
	if err := jp.Parse(fmt.Sprintf("{.%s}", key)); err != nil {
		return err
	}

	fullResults, err := jp.FindResults(c)
	if err != nil {
		return err
	}
	if len(fullResults) != 1 || len(fullResults[0]) != 1 {
		return fmt.Errorf("%s could not be set", key)

	}

	fullResults[0][0].Set(reflect.ValueOf(value))
	return nil
}

func setEnvVar(env []ManagerEnvVar, name, value string) []ManagerEnvVar {
	for i := range env {
		if env[i].Name == name {
			if value == "" {
				return append(env[:i], env[i+1:]...)
			}
			env[i].Value = value
			return env
		}
	}
	if value == "" {
		return env
	}
	return append(env, ManagerEnvVar{Name: name, Value: value})
}
