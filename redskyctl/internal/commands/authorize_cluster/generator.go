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

package authorize_cluster

import (
	"context"
	"strings"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/oauth2/registration"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO This should work like a kustomize secret generator for the extra env vars
// TODO We should take annotations as input (here and on the the other generators)
// TODO Add --envFile option that gets merged with the configuration environment variables
// TODO Should we get information from other secrets in other namespaces?
// TODO What about overriding the secret name to something we do not overwrite?

// GeneratorOptions are the configuration options for generating the cluster authorization secret
type GeneratorOptions struct {
	// Config is the Red Sky Configuration used to generate the authorization secret
	Config *config.RedSkyConfig
	// Printer is the resource printer used to render generated objects
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Name is the name of the secret to generate
	Name string
	// ClientName is the name of the client to register with the authorization server
	ClientName string
}

// NewGeneratorCommand creates a command for generating the cluster authorization secret
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Generate Red Sky Ops authorization",
		Long:  "Generate authorization secret for Red Sky Ops",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.WithContextE(o.complete)(cmd, args)
		},
		RunE: commander.WithContextE(o.generate),
	}

	// TODO Allow name and client name to be configurable?

	commander.SetKubePrinter(&o.Printer, cmd)
	commander.ExitOnError(cmd)
	return cmd
}

// complete fills in the default values for the generator configuration
func (o *GeneratorOptions) complete(ctx context.Context) error {
	if o.Name == "" {
		o.Name = "redsky-manager"
	}

	if o.ClientName == "" {
		kubectl, err := o.Config.Kubectl(ctx, "config", "view", "--minify", "--output", "jsonpath={.clusters[0].name}")
		if err != nil {
			return err
		}
		stdout, err := kubectl.Output()
		if err != nil {
			return err
		}
		o.ClientName = strings.TrimSpace(string(stdout))
	}

	return nil
}

func (o *GeneratorOptions) generate(ctx context.Context) error {
	// Read the initial information from the configuration
	r := o.Config.Reader()
	ctrl, err := config.CurrentController(r)
	if err != nil {
		return err
	}
	data, err := config.EnvironmentMapping(r, true)
	if err != nil {
		return err
	}

	// Create a new secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: ctrl.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	// TODO Block registration if the authorization has a default namespace claim?
	// TODO Record the client information in the local configuration for management at a later time
	// TODO If we see a record of the same name in the local config, fetch it instead of registering it!
	// We MUST do the recording before we can support output methods other then full serializations...

	// Register the client with the authorization server
	info, err := o.Config.RegisterClient(ctx, &registration.ClientMetadata{
		ClientName:    o.ClientName,
		GrantTypes:    []string{"client_credentials"},
		RedirectURIs:  []string{},
		ResponseTypes: []string{},
	})
	if err != nil {
		return err
	}

	// Overwrite the client credentials in the secret
	secret.Data["REDSKY_AUTHORIZATION_CLIENT_ID"] = []byte(info.ClientID)
	secret.Data["REDSKY_AUTHORIZATION_CLIENT_SECRET"] = []byte(info.ClientSecret)

	return o.Printer.PrintObj(secret, o.Out)
}
