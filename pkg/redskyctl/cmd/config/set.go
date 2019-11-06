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
	"os"
	"strings"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	setLong    = `Modify the Red Sky Ops configuration file`
	setExample = `Names are: address, oauth2.token, oauth2.token_url, oauth2.client_id, oauth2.client_secret

# Set the remote server address
redskyctl config set address http://example.carbonrelay.io`
)

type ConfigSetOptions struct {
	Key   string
	Value string

	cmdutil.IOStreams
}

func NewConfigSetOptions(ioStreams cmdutil.IOStreams) *ConfigSetOptions {
	return &ConfigSetOptions{
		IOStreams: ioStreams,
	}
}

func NewConfigSetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigSetOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "set NAME [VALUE]",
		Short:   "Modify the configuration file",
		Long:    setLong,
		Example: setExample,
		Args:    cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigSetOptions) Complete(args []string) error {
	if len(args) > 0 {
		o.Key = args[0]
	}
	if len(args) > 1 {
		o.Value = args[1]
	} else if strings.Contains(o.Key, "=") {
		s := strings.SplitN(o.Key, "=", 2)
		o.Key = s[0]
		o.Value = s[1]
	}
	return nil
}

func (o *ConfigSetOptions) Run() error {
	cfg, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	if strings.HasPrefix(o.Key, "manager.env.") {
		// TODO No idea if this is the best way to do this
		var mgrEnv []ManagerEnvVar
		if err := cfg.UnmarshalKey("manager.env", &mgrEnv); err != nil {
			return err
		}
		mgrEnv = setEnvVar(mgrEnv, strings.TrimPrefix(o.Key, "manager.env."), o.Value)
		cfg.Set("manager.env", mgrEnv)
	} else {
		cfg.Set(o.Key, o.Value)
	}

	// Viper is frustratingly buggy. We can't just use WriteConfig because it won't honor the explicit configuration type.
	if err := cfg.WriteConfigAs(os.ExpandEnv("${HOME}/redsky.yaml")); err != nil {
		return err
	}
	if err := os.Rename(os.ExpandEnv("${HOME}/redsky.yaml"), os.ExpandEnv("${HOME}/.redsky")); err != nil {
		return err
	}
	return nil
}

func setEnvVar(mgrEnv []ManagerEnvVar, key, value string) []ManagerEnvVar {
	for i := range mgrEnv {
		if mgrEnv[i].Name == key {
			if value == "" {
				return append(mgrEnv[:i], mgrEnv[i+1:]...)
			}
			mgrEnv[i].Value = value
			return mgrEnv
		}
	}
	if value == "" {
		return mgrEnv
	}
	return append(mgrEnv, ManagerEnvVar{Name: key, Value: value})
}
