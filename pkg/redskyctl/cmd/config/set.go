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
	"strings"

	"github.com/redskyops/k8s-experiment/internal/config"
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
	cfg := &config.RedSkyConfig{}

	if err := cfg.Load(); err != nil {
		return err
	}

	if err := cfg.Update(config.SetProperty(o.Key, o.Value)); err != nil {
		return err
	}

	if err := cfg.Write(); err != nil {
		return err
	}

	return nil
}
