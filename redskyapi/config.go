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

package redskyapi

import (
	"context"
	"net/http"
	"net/url"
	"os/exec"

	"github.com/redskyops/k8s-experiment/redskyapi/config"
)

// Config exposes the information for configuring a Red Sky Client
type Config interface {
	// SystemNamespace returns the Red Sky Controller system namespace (e.g. "redsky-system").
	SystemNamespace() (string, error)

	// ExperimentsURL returns a URL to the experiments API
	ExperimentsURL(path string) (*url.URL, error)

	// Kubectl returns an executable command for running kubectl
	Kubectl(arg ...string) (*exec.Cmd, error)

	// Authorize returns a transport that applies the authorization defined by this configuration. The
	// supplied context is used for any additional requests necessary to perform authentication. If this
	// configuration does not define any authorization details, the supplied transport may be returned
	// directly.
	Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error)

}

// DefaultConfig loads the mostly commonly used configuration
func DefaultConfig() (Config, error) {
	cc := &config.ClientConfig{}

	if err := cc.Load(); err != nil {
		return nil, err
	}

	return cc, nil
}
