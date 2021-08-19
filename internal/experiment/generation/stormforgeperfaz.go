/*
Copyright 2021 GramLabs, Inc.

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

package generation

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/thestormforge/optimize-go/pkg/config"
)

// DefaultOptimizeConfig is the mega-hack-sky-hook used to inject configuration
// this low into the code. This should not be necessary, as generation should not
// be accessing the configuration directly.
var DefaultOptimizeConfig *config.OptimizeConfig

// StormForgePerformanceAuthorization is a helper for obtaining StormForge Performance
// access tokens (JWTs). Normally, low level code like this does not have access to
// the application layer configuration so this is a bit of a hack to ensure we can
// talk to the appropriate APIs.
type StormForgePerformanceAuthorization struct {
	// The configuration to get authorization information from, leave nil to get the default.
	Config *config.OptimizeConfig
	// The function used to lookup environment variables, leave nil to get the default.
	LookupEnvFunc func(string) (string, bool)
	// The label to use to describe the Performance Service Account (if needed)
	ServiceAccountLabel func() string
}

// AccessToken returns a StormForge Performance access token for the optional organization.
// If the organization is empty, we use the private space that depends on shared authorization
// between the two products.
func (az *StormForgePerformanceAuthorization) AccessToken(ctx context.Context, org string) (string, error) {
	if err := az.setDefaults(); err != nil {
		return "", err
	}

	if org != "" {
		return az.accessTokenForOrganization(ctx, org)
	}

	return az.accessTokenForOptimize(ctx)
}

func (az *StormForgePerformanceAuthorization) accessTokenForOrganization(ctx context.Context, org string) (string, error) {
	// Check for an organization specific environment variable (this will never conflict with "real forge")
	key := fmt.Sprintf("STORMFORGER_%s_JWT", strings.Map(toEnv, org))
	if at, ok := az.lookupEnv(key); ok {
		return at, nil
	}

	// Register a new service account
	sa, err := exec.CommandContext(ctx, "forge", "serviceaccount", "create", org, az.ServiceAccountLabel()).Output()
	if err != nil {
		return "", err
	}

	// The last line of the output (that contains the two dots) is the JWT
	var accessToken string
	tokenScanner := bufio.NewScanner(bytes.NewBuffer(sa))
	for tokenScanner.Scan() {
		if strings.Count(tokenScanner.Text(), ".") == 2 {
			accessToken = tokenScanner.Text()
		}
	}

	// Return the access token, persist the value in the configuration so we don't need to create it again
	return accessToken, az.storeEnv(key, accessToken)
}

func (az *StormForgePerformanceAuthorization) accessTokenForOptimize(ctx context.Context) (string, error) {
	// Check to see if the standard environment variable is set
	if at, ok := az.lookupEnv("STORMFORGER_JWT"); ok {
		return at, nil
	}

	// Configure a token exchange
	src, err := az.Config.PerformanceAuthorization(ctx)
	if err != nil {
		return "", err
	}

	// Swap the Optimize token for a Performance token (don't bother to save it, we can always exchange again)
	t, err := src.Token()
	if err != nil {
		return "", err
	}

	return t.AccessToken, nil
}

// setDefaults ensures the authorization helper has default values set.
func (az *StormForgePerformanceAuthorization) setDefaults() error {
	// Populate the config from the global sky hook: this is mostly for the CLI
	if az.Config == nil {
		az.Config = DefaultOptimizeConfig
	}

	// If we still need a configuration, load a fresh instance
	if az.Config == nil {
		az.Config = &config.OptimizeConfig{
			AuthorizationParameters: map[string][]string{"audience": {"https://api.carbonrelay.io/v1/"}},
		}
		if err := az.Config.Load(); err != nil {
			return err
		}
	}

	// Usually we want to defer to the OS
	if az.LookupEnvFunc == nil {
		az.LookupEnvFunc = os.LookupEnv
	}

	return nil
}

// lookupEnv is used to resolve environment variables.
func (az *StormForgePerformanceAuthorization) lookupEnv(key string) (string, bool) {
	// Check the actual current environment, if configured
	if az.LookupEnvFunc != nil {
		return az.LookupEnvFunc(key)
	}

	// Check to see if the environment variable is persisted in the configuration file
	if ctrl, err := config.CurrentController(az.Config.Reader()); err == nil {
		for _, envVar := range ctrl.Env {
			if envVar.Name == key {
				return envVar.Value, true
			}
		}
	}

	return "", false
}

// storeEnv persists an environment variable in the configuration.
func (az *StormForgePerformanceAuthorization) storeEnv(key, value string) error {
	r := az.Config.Reader()

	controllerName, err := r.ControllerName(r.ContextName())
	if err != nil {
		return err
	}

	if err := az.Config.Update(func(cfg *config.Config) error {
		for i := range cfg.Controllers {
			if cfg.Controllers[i].Name != controllerName {
				continue
			}

			// Update the environment variable and return
			for j := range cfg.Controllers[i].Controller.Env {
				if cfg.Controllers[i].Controller.Env[j].Name == key {
					cfg.Controllers[i].Controller.Env[j].Value = value
					return nil
				}
			}

			// Add the new environment variable and return
			cfg.Controllers[i].Controller.Env = append(cfg.Controllers[i].Controller.Env, config.ControllerEnvVar{
				Name:  key,
				Value: value,
			})
			return nil
		}

		return nil
	}); err != nil {
		return err
	}

	return az.Config.Write()
}

// toEnv is a helper to map StormForge Performance organization names to environment variables.
func toEnv(r rune) rune {
	switch r {
	case '-':
		return '_'
	default:
		return unicode.ToUpper(r)
	}
}
