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

package config

import (
	"fmt"
	"net/url"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	envLong    = `View the Red Sky Ops configuration file as environment variables`
	envExample = ``
)

func NewEnvCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runEnv

	cmd := &cobra.Command{
		Use:     "env",
		Short:   "Generate environment variables from configuration",
		Long:    envLong,
		Example: envExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigOptions) runEnv() error {
	env := make(map[string]string)

	env["REDSKY_ADDRESS"] = o.Config.Address
	if o.Config.OAuth2 != nil {
		env["REDSKY_OAUTH2_CLIENT_ID"] = o.Config.OAuth2.ClientID
		env["REDSKY_OAUTH2_CLIENT_SECRET"] = o.Config.OAuth2.ClientSecret

		// TODO Make this behavior optional
		env["REDSKY_OAUTH2_TOKEN_URL"] = resolveTokenURL(o.Config)
	}
	if o.Config.Manager != nil {
		for _, v := range o.Config.Manager.Environment {
			env[v.Name] = v.Value
		}
	}

	// Serialize the environment map to a ".env" format
	for k, v := range env {
		if v != "" {
			_, _ = fmt.Fprintf(o.Out, "%s=%s\n", k, v)
		}
	}

	return nil
}

func resolveTokenURL(cfg *api.Config) string {
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return cfg.OAuth2.TokenURL
	}
	t, err := url.Parse(cfg.OAuth2.TokenURL)
	if err != nil {
		return cfg.OAuth2.TokenURL
	}
	return u.ResolveReference(t).String()
}
