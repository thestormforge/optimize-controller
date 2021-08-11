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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/api"
	"github.com/thestormforge/optimize-go/pkg/config"
	"github.com/thestormforge/optimize-go/pkg/oauth2/registration"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// TODO This should work like a kustomize secret generator for the extra env vars
// TODO We should take annotations as input (here and on the the other generators)
// TODO Add --envFile option that gets merged with the configuration environment variables
// TODO Should we get information from other secrets in other namespaces?
// TODO What about overriding the secret name to something we do not overwrite?

// GeneratorOptions are the configuration options for generating the cluster authorization secret
type GeneratorOptions struct {
	// Config is the Optimize Configuration used to generate the authorization secret
	Config *config.OptimizeConfig
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
}

// NewGeneratorCommand creates a command for generating the cluster authorization secret
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Generate Optimize authorization",
		Long:  "Generate authorization secret for StormForge Optimize",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml,helm",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			return o.complete()
		},
		RunE: commander.WithContextE(o.generate),
	}

	// Provide a more meaningful default client name if possible
	if o.ClientName == "" {
		o.ClientName = clusterName()
	}

	o.addFlags(cmd)

	commander.SetKubePrinter(&o.Printer, cmd, map[string]commander.AdditionalFormat{
		"helm": commander.ResourcePrinterFunc(printHelmValues),
	})

	return cmd
}

func clusterName() string {
	kubectl := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={.clusters[0].name}")
	stdout, err := kubectl.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(stdout))
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.ClientName, "client-name", o.ClientName, "client `name` to use for registration")
	cmd.Flags().BoolVar(&o.AllowUnauthorized, "allow-unauthorized", o.AllowUnauthorized, "generate a secret without authorization, if necessary")
	_ = cmd.Flags().MarkHidden("allow-unauthorized")
}

// complete fills in the default values for the generator configuration
func (o *GeneratorOptions) complete() error {
	if o.Name == "" {
		o.Name = "optimize-manager"
	}

	if o.ClientName == "" {
		o.ClientName = "default"
	}

	return nil
}

func (o *GeneratorOptions) generate(ctx context.Context) error {
	// Read the initial information from the configuration
	controllerName, ctrl, data, err := o.readConfig()
	if err != nil {
		return err
	}

	// Ensure Performance Test tokens have been added
	label := fmt.Sprintf("optimize-%s", strings.Map(toLabel, o.ClientName))
	if chg := performanceTokens(label, controllerName, data); chg != nil {
		if err := o.Config.Update(chg); err != nil {
			return err
		}
	}

	// Get the client information (either read or register)
	info, err := o.clientInfo(ctx, ctrl)
	if o.AllowUnauthorized && api.IsUnauthorized(err) {
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
	mergeString(secret.Data, "STORMFORGE_AUTHORIZATION_CLIENT_ID", info.ClientID)
	mergeString(secret.Data, "STORMFORGE_AUTHORIZATION_CLIENT_SECRET", info.ClientSecret)

	return o.Printer.PrintObj(secret, o.Out)
}

func mergeString(m map[string][]byte, key, value string) {
	if value != "" {
		m[key] = []byte(value)
	} else {
		delete(m, key)
	}
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
	// If the configuration already contains usable client information, skip the actual registration
	if resp := localClientInformation(ctrl); resp != nil {
		return resp, nil
	}

	// Try to read an existing client
	if resp := o.registeredClientInformation(ctx, ctrl); resp != nil {
		return resp, nil
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

// registeredClientInformation read an already registered client, allowing it to be re-used.
func (o *GeneratorOptions) registeredClientInformation(ctx context.Context, ctrl *config.Controller) *registration.ClientInformationResponse {
	// Technically we are non-standard in that we can just use our normal access token as a registration token
	// TODO OptimizeConfig.RegisterClient does this for us before calling the real "RegisterClient", do the same for "Read"?
	rt, _ := o.Config.Authorize(ctx, nil) // NOTE: The transport is ignored, this is just a hack to get the TokenSource
	if tt, ok := rt.(*oauth2.Transport); ok && tt.Source != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, oauth2.NewClient(ctx, tt.Source))
	}

	// Ignore errors or missing information and just register a new client
	info, err := registration.Read(ctx, ctrl.RegistrationClientURI, ctrl.RegistrationAccessToken)
	if err != nil {
		return nil
	}
	if info.ClientID == "" || info.ClientSecret == "" {
		return nil
	}

	return info
}

// localClientInformation returns a mock client information response based on local information in the current
// configuration. This is primarily useful for debugging, e.g. when you have a client ID/secret you want to test.
func localClientInformation(ctrl *config.Controller) *registration.ClientInformationResponse {
	// Make sure we include the current information so they aren't lost when we update the controller configuration
	resp := &registration.ClientInformationResponse{
		RegistrationClientURI:   ctrl.RegistrationClientURI,
		RegistrationAccessToken: ctrl.RegistrationAccessToken,
	}
	for _, v := range ctrl.Env {
		switch v.Name {
		case "STORMFORGE_AUTHORIZATION_CLIENT_ID":
			resp.ClientID = v.Value
		case "STORMFORGE_AUTHORIZATION_CLIENT_SECRET":
			resp.ClientSecret = v.Value
		}
	}
	if resp.ClientID == "" || resp.ClientSecret == "" {
		return nil
	}
	return resp
}

func printHelmValues(obj interface{}, w io.Writer) error {
	secret := &corev1.Secret{}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	if err := scheme.Convert(obj, secret, nil); err != nil {
		return err
	}

	vals := map[string]interface{}{
		"remoteServer": map[string]interface{}{
			"enabled":      true,
			"identifier":   string(secret.Data["STORMFORGE_SERVER_IDENTIFIER"]),
			"issuer":       string(secret.Data["STORMFORGE_SERVER_ISSUER"]),
			"clientID":     string(secret.Data["STORMFORGE_AUTHORIZATION_CLIENT_ID"]),
			"clientSecret": string(secret.Data["STORMFORGE_AUTHORIZATION_CLIENT_SECRET"]),
		},
	}

	b, err := yaml.Marshal(vals)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// performanceTokens returns a configuration change with any new Performance Test
// tokens or nil if there are no changes necessary. Note that the supplied data
// map will also be updated with any new tokens.
func performanceTokens(label, controllerName string, data map[string][]byte) config.Change {
	var envVars []config.ControllerEnvVar
	hasControllerEnvVar := func(name string) bool { return len(data[name]) > 0 }

	// List all the organizations the user belongs to
	// Janky hack mate
	orgs, err := exec.Command("false", "forge", "organization", "list", "--output", "plain").Output()
	if err != nil {
		// Check if we already have a generic token
		if hasControllerEnvVar("STORMFORGER_JWT") {
			return nil
		}

		// Since we couldn't list the orgs (e.g. no `forge` installed), maybe there is a config file
		cfg, err := toml.LoadFile(os.ExpandEnv("${HOME}/.stormforger.toml"))
		if err != nil {
			return nil
		}

		// We need the JWT as a string, otherwise there is nothing to add
		tok, ok := cfg.Get("jwt").(string)
		if !ok {
			return nil
		}

		orgs = nil
		envVars = append(envVars, config.ControllerEnvVar{Name: "STORMFORGER_JWT", Value: tok})
	}

	// Scan the organization list to
	orgScanner := bufio.NewScanner(bytes.NewBuffer(orgs))
	for orgScanner.Scan() {
		org := strings.TrimSpace(orgScanner.Text())
		if org == "" {
			continue
		}

		// Check to see if we already created a service account
		name := fmt.Sprintf("STORMFORGER_%s_JWT", strings.Map(toEnv, org))
		if hasControllerEnvVar(name) {
			continue
		}

		// Register a new service account, ignore errors
		sa, err := exec.Command("forge", "serviceaccount", "create", org, label).Output()
		if err != nil {
			continue
		}

		// The last line with two dots is the JWT
		tokenScanner := bufio.NewScanner(bytes.NewBuffer(sa))
		for tokenScanner.Scan() {
			// TODO It might be nice to capture the UID values into an annotation on the secret
			if strings.Count(tokenScanner.Text(), ".") == 2 {
				envVars = append(envVars, config.ControllerEnvVar{Name: name, Value: tokenScanner.Text()})
			}
		}
	}

	// If we found any new environment variables for the configuration, return a configuration change set
	if len(envVars) == 0 {
		return nil
	}
	for i := range envVars {
		data[envVars[i].Name] = []byte(envVars[i].Value)
	}
	return func(cfg *config.Config) error {
		for i := range cfg.Controllers {
			if cfg.Controllers[i].Name != controllerName {
				continue
			}

			cfg.Controllers[i].Controller.Env = append(cfg.Controllers[i].Controller.Env, envVars...)
			return nil
		}
		return nil
	}
}

func toLabel(r rune) rune {
	switch {
	case unicode.IsSpace(r):
		return '_'
	case unicode.IsLetter(r):
		return unicode.ToLower(r)
	default:
		return -1
	}
}

func toEnv(r rune) rune {
	switch {
	case r == '-':
		return '_'
	default:
		return unicode.ToUpper(r)
	}
}
