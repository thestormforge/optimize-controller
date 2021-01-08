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

package check

import (
	"context"
	"crypto/sha1"
	"fmt"

	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// ConfigOptions are the options for checking a Red Sky Configuration
type ConfigOptions struct {
	// Config is the Red Sky Configuration to check
	Config *config.RedSkyConfig
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// TODO Verbose? Skip server check?
}

// NewConfigCommand creates a new command for checking the Red Sky Configuration
func NewConfigCommand(o *ConfigOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Check the configuration",
		Long:  "Check the Red Sky Configuration",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			// TODO We should have an option to overwrite the configuration using stdin (e.g. to test connections using the controller config)
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: commander.WithContextE(o.checkConfig),
	}

	return cmd
}

// checkConfig runs sanity checks on the configuration
func (o *ConfigOptions) checkConfig(ctx context.Context) error {
	r := o.Config.Reader()

	// Verify we can minify the configuration
	if _, err := config.Minify(r); err != nil {
		return err
	}

	// Verify we can connect using the current configuration
	if _, err := o.ExperimentsAPI.Options(ctx); err != nil {
		return err
	}

	// Print out a success message that includes the tenant identifier
	tenant, err := tenantID(r)
	if err != nil {
		return err
	}
	if tenant != "" {
		_, _ = fmt.Fprintf(o.Out, "Success, configuration is valid for tenant '%s'.\n", tenant)
	} else {
		_, _ = fmt.Fprintf(o.Out, "Success.\n")
	}
	return nil
}

func tenantID(r config.Reader) (string, error) {
	az, err := config.CurrentAuthorization(r)
	if err != nil {
		return "", err
	}
	if az.Credential.TokenCredential == nil {
		return "", nil
	}
	mc := jwt.MapClaims{}
	if _, _, err := new(jwt.Parser).ParseUnverified(az.Credential.TokenCredential.AccessToken, mc); err != nil {
		return "", err
	}
	if tenant, ok := mc["https://carbonrelay.com/claims/namespace"].(string); ok {
		th := sha1.Sum([]byte(tenant))
		return fmt.Sprintf("%x", th[0:4]), nil
	}
	return "", fmt.Errorf("unable to determine tenant identifier")
}
