/*
Copyright 2022 GramLabs, Inc.

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

package reset

import (
	"context"
	"net/http"

	"github.com/thestormforge/optimize-go/pkg/config"
	"golang.org/x/oauth2"
)

// deleteControllerClient deletes the client used by the current controller
// from the remote registration service and the local configuration.
func deleteControllerClient(ctx context.Context, cfg *config.OptimizeConfig) error {
	// Check to see if the current controller has a registered client
	ctrl, err := config.CurrentController(cfg.Reader())
	if err != nil {
		return err
	}
	if ctrl.RegistrationClientURI == "" {
		return nil
	}

	// Authorize an HTTP client
	c := oauth2.NewClient(ctx, nil)
	c.Transport, err = cfg.Authorize(ctx, c.Transport)
	if err != nil {
		return err
	}

	// Issue a DELETE to client registration URI
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, ctrl.RegistrationClientURI, nil)
	if err != nil {
		return err
	}
	_, err = c.Do(req)
	if err != nil {
		return err
	}

	// Remove all references to the client from the configuration
	if err := cfg.Update(deleteClientRegistration(ctrl.RegistrationClientURI)); err != nil {
		return err
	}
	return cfg.Write()
}

// deleteClientRegistration removes all references to a client given it's registration URL.
func deleteClientRegistration(u string) config.Change {
	return func(cfg *config.Config) error {
		for i := range cfg.Controllers {
			if cfg.Controllers[i].Controller.RegistrationClientURI != u {
				continue
			}

			cfg.Controllers[i].Controller.RegistrationClientURI = ""
			cfg.Controllers[i].Controller.RegistrationAccessToken = ""
		}
		return nil
	}
}
