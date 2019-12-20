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

package generate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	redskyclient "github.com/redskyops/k8s-experiment/redskyapi"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

const (
	generateSecretLong    = `Generate authorization secret for Red Sky Ops`
	generateSecretExample = ``
)

// TODO This should work like a kustomize secret generator for the extra env vars

type GenerateSecretOptions struct {
	Name      string
	Namespace string

	cmdutil.IOStreams
}

func NewGenerateSecretOptions(ioStreams cmdutil.IOStreams) *GenerateSecretOptions {
	return &GenerateSecretOptions{
		Name:      "redsky-manager",
		Namespace: "redsky-system",
		IOStreams: ioStreams,
	}
}

func NewGenerateSecretCmd(ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGenerateSecretOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "secret",
		Short:   "Generate Red Sky Ops authorization",
		Long:    generateSecretLong,
		Example: generateSecretExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}

	// This won't show up in the help, but it will still get populated
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "The namespace to be used by the manager.")

	return cmd
}

func (o *GenerateSecretOptions) Complete() error {
	return nil
}

func (o *GenerateSecretOptions) Run() error {
	secret := &corev1.Secret{}
	secret.Name = o.Name
	secret.Namespace = o.Namespace
	secret.Type = corev1.SecretTypeOpaque

	// TODO This should login to the server and obtain a new secret; for now just copy from the default config
	cfg, err := redskyclient.DefaultConfig()
	if err != nil {
		return err
	}
	secret.Data = make(map[string][]byte)
	secret.Data["REDSKY_ADDRESS"] = []byte(cfg.Address)
	secret.Data["REDSKY_OAUTH2_CLIENT_ID"] = []byte(cfg.OAuth2.ClientID)
	secret.Data["REDSKY_OAUTH2_CLIENT_SECRET"] = []byte(cfg.OAuth2.ClientSecret)
	secret.Data["REDSKY_OAUTH2_TOKEN_URL"] = []byte(cfg.OAuth2.TokenURL)
	for _, v := range cfg.Manager.Environment {
		secret.Data[v.Name] = []byte(v.Value)
	}

	// Remove empty values
	for k, v := range secret.Data {
		if string(v) == "" {
			delete(secret.Data, k)
		}
	}

	// TODO ULTRA HACK. Update the name based on what the hash name should be; this is only exposed to callers in code...
	o.Name, err = hashSecretName(secret)
	if err != nil {
		return err
	}

	return serialize(secret, o.Out)
}

// Mimic Kustomize
func hashSecretName(sec *corev1.Secret) (string, error) {
	data, err := json.Marshal(map[string]interface{}{"kind": "Secret", "type": sec.Type, "name": sec.Name, "data": sec.Data})
	if err != nil {
		return "", err
	}
	hex := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
	enc := []rune(hex[:10])
	for i := range enc {
		switch enc[i] {
		case '0':
			enc[i] = 'g'
		case '1':
			enc[i] = 'h'
		case '3':
			enc[i] = 'k'
		case 'a':
			enc[i] = 'm'
		case 'e':
			enc[i] = 't'
		}
	}
	return fmt.Sprintf("%s-%s", sec.Name, string(enc)), nil
}
