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
	"github.com/redskyops/k8s-experiment/internal/config"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	viewLong    = `View the Red Sky Ops configuration file`
	viewExample = ``
)

// TODO Like the version command, support dumping the default configuration from the manager
// `kubectl exec -n redsky-system -c manager $(kubectl get pods -n redsky-system -o name) /manager config`
// TODO Add a --helm-values option to output the configuration as a Helm values file

type ConfigViewOptions struct {
	cmdutil.IOStreams
}

func NewConfigViewOptions(ioStreams cmdutil.IOStreams) *ConfigViewOptions {
	return &ConfigViewOptions{
		IOStreams: ioStreams,
	}
}

func NewConfigViewCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigViewOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "view",
		Short:   "View the configuration file",
		Long:    viewLong,
		Example: viewExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigViewOptions) Run() error {
	cfg := &config.RedSkyConfig{}

	if err := cfg.Load(); err != nil {
		return err
	}

	output, err := cfg.Marshal()
	if err != nil {
		return err
	}

	_, err = o.Out.Write(output)
	return err
}
