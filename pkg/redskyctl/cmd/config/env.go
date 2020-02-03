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

	"github.com/redskyops/k8s-experiment/internal/config"
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
	cfg := &config.RedSkyConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	env, err := config.LegacyEnvMapping(cfg, o.Manager)
	if err != nil {
		return err
	}

	// Serialize the environment map to a ".env" format
	var keys []string
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(o.Out, "%s=%s\n", k, string(env[k]))
	}

	return nil
}
