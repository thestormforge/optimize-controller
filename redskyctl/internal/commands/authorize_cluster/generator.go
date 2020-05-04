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

package authorize_cluster

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/oauth2/registration"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
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
	// AllowUnauthorized generates a secret with no authorization information
	AllowUnauthorized bool
	// HelmValues indicates that instead of generating a Kubernetes secret, we should generate a Helm values file
	HelmValues bool
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

	o.addFlags(cmd)

	commander.SetKubePrinter(&o.Printer, cmd)
	commander.ExitOnError(cmd)
	return cmd
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
	// TODO Allow name to be configurable?
	cmd.Flags().StringVar(&o.ClientName, "client-name", o.ClientName, "Client name to use for registration.")
	cmd.Flags().BoolVar(&o.HelmValues, "helm-values", o.HelmValues, "Generate a Helm values file instead of a secret.")
	cmd.Flags().BoolVar(&o.AllowUnauthorized, "allow-unauthorized", o.AllowUnauthorized, "Generate a secret without authorization, if necessary.")
	_ = cmd.Flags().MarkHidden("allow-unauthorized")
}

// complete fills in the default values for the generator configuration
func (o *GeneratorOptions) complete(ctx context.Context) error {
	if o.Name == "" {
		o.Name = "redsky-manager"
	}

	// TODO Should this be part of `NewGeneratorCommand` (before addFlags) so the default can appear in the help output?
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
	controllerName, ctrl, data, err := o.readConfig()
	if err != nil {
		return err
	}

	// Get the client information (either read or register)
	info, err := o.clientInfo(ctx, ctrl)
	if o.AllowUnauthorized && redskyapi.IsUnauthorized(err) {
		// Ignore the error (but do not save the changes)
		info = &registration.ClientInformationResponse{}
	} else if err != nil {
		return err
	} else {
		// Save any changes we made to the configuration (even if we didn't register, the access token might have rolled)
		_ = o.Config.Update(config.SaveClientRegistration(controllerName, info))
		if err := o.Config.Write(); err != nil {
			_, _ = fmt.Fprintln(o.ErrOut, "Could not update configuration with controller registration information")
		}
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

	// Overwrite the client credentials in the secret
	secret.Data["REDSKY_AUTHORIZATION_CLIENT_ID"] = []byte(info.ClientID)
	secret.Data["REDSKY_AUTHORIZATION_CLIENT_SECRET"] = []byte(info.ClientSecret)

	// Use an alternate printer just for Helm values
	if o.HelmValues {
		o.Printer = &helmValuesPrinter{}
	}

	return o.Printer.PrintObj(secret, o.Out)
}

func (o *GeneratorOptions) readConfig() (string, *config.Controller, map[string][]byte, error) {
	// Read the initial information from the configuration
	r := o.Config.Reader()
	controllerName, err := r.ControllerName(r.ContextName())
	if err != nil {
		return "", nil, nil, err
	}
	ctrl, err := r.Controller(controllerName)
	if err != nil {
		return "", nil, nil, err
	}
	data, err := config.EnvironmentMapping(r, true)
	if err != nil {
		return "", nil, nil, err
	}
	return controllerName, &ctrl, data, nil
}

func (o *GeneratorOptions) clientInfo(ctx context.Context, ctrl *config.Controller) (*registration.ClientInformationResponse, error) {
	// Try to read an existing client (ignore errors and just re-register)
	if ctrl.RegistrationClientURI != "" {
		if info, err := registration.Read(ctx, ctrl.RegistrationClientURI, ctrl.RegistrationAccessToken); err == nil {
			return info, nil
		}
	}

	// Register a new client
	client := &registration.ClientMetadata{
		ClientName:    o.ClientName,
		GrantTypes:    []string{"client_credentials"},
		RedirectURIs:  []string{},
		ResponseTypes: []string{},
	}
	return o.Config.RegisterClient(ctx, client)
}

type helmValuesPrinter struct {
}

func (h helmValuesPrinter) PrintObj(i interface{}, w io.Writer) error {
	if secret, ok := i.(*corev1.Secret); ok {
		vals := map[string]interface{}{
			"remoteServer": map[string]interface{}{
				"enabled":      true,
				"identifier":   string(secret.Data["REDSKY_SERVER_IDENTIFIER"]),
				"issuer":       string(secret.Data["REDSKY_SERVER_ISSUER"]),
				"clientID":     string(secret.Data["REDSKY_AUTHORIZATION_CLIENT_ID"]),
				"clientSecret": string(secret.Data["REDSKY_AUTHORIZATION_CLIENT_SECRET"]),
			},
		}

		b, err := yaml.Marshal(vals)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}
