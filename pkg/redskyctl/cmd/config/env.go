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
	"sort"

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

	cmd.Flags().BoolVar(&o.Manager, "manager", false, "Generate the manager environment.")

	return cmd
}

func (o *ConfigOptions) runEnv() error {
	env := make(map[string]string)

	env["REDSKY_ADDRESS"] = o.Config.Address

	if o.Config.OAuth2 != nil {
		env["REDSKY_OAUTH2_CLIENT_ID"] = o.Config.OAuth2.ClientID
		env["REDSKY_OAUTH2_CLIENT_SECRET"] = o.Config.OAuth2.ClientSecret
		env["REDSKY_OAUTH2_TOKEN_URL"] = o.Config.OAuth2.TokenURL

		// Outside the context of the manager it is easier to have the resolved URL
		if !o.Manager {
			if b, err := url.Parse(o.Config.Address); err == nil {
				if r, err := b.Parse(o.Config.OAuth2.TokenURL); err == nil {
					env["REDSKY_OAUTH2_TOKEN_URL"] = r.String()
				}
			}
		}
	}

	if o.Config.Manager != nil && o.Manager {
		for _, v := range o.Config.Manager.Environment {
			env[v.Name] = v.Value
		}
	}

	// Serialize the environment map to a ".env" format
	var keys []string
	for k, v := range env {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(o.Out, "%s=%s\n", k, env[k])
	}

	return nil
}
