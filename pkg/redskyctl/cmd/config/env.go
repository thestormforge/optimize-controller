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
	"sort"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	envLong    = `View the Red Sky Ops configuration file as environment variables`
	envExample = ``
)

type ConfigEnvOptions struct {
	Manager bool

	cmdutil.IOStreams
}

func NewConfigEnvOptions(ioStreams cmdutil.IOStreams) *ConfigEnvOptions {
	return &ConfigEnvOptions{
		IOStreams: ioStreams,
	}
}

func NewConfigEnvCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigEnvOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "env",
		Short:   "Generate environment variables from configuration",
		Long:    envLong,
		Example: envExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Manager, "manager", false, "Generate the manager environment.")

	return cmd
}

func (o *ConfigEnvOptions) Run() error {
	// It would be nice if we could just access the bindings stored in Viper
	cfg, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	// Create an environment map, we will ignore empty strings later
	env := make(map[string]string)
	env["REDSKY_ADDRESS"] = cfg.GetString("address")
	env["REDSKY_OAUTH2_CLIENT_ID"] = cfg.GetString("oauth2.client_id")
	env["REDSKY_OAUTH2_CLIENT_SECRET"] = cfg.GetString("oauth2.client_secret")

	// No good way to detect defaults
	if cfg.IsSet("oauth2.client_id") || cfg.IsSet("oauth2.client_secret") {
		env["REDSKY_OAUTH2_TOKEN_URL"] = cfg.GetString("oauth2.token_url")

		// When we are not targeting the manager, resolve the full URL (e.g. so you can use it in cURL)
		if !o.Manager {
			if b, err := api.GetAddress(cfg); err == nil {
				if r, err := b.Parse(cfg.GetString("oauth2.token_url")); err == nil {
					env["REDSKY_OAUTH2_TOKEN_URL"] = r.String()
				}
			}
		}
	}

	// Add manager specific environment variables
	if o.Manager {
		var mgrEnv []ManagerEnvVar
		if err := cfg.UnmarshalKey("manager.env", &mgrEnv); err != nil {
			return err
		}
		for _, v := range mgrEnv {
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
