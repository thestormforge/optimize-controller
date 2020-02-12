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

package configuration

import (
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// TODO Like the version command, support dumping the default configuration from the manager
// `kubectl exec -n redsky-system -c manager $(kubectl get pods -n redsky-system -o name) /manager config`
// TODO Add an option to output a Helm values.yaml for our chart
// TODO We should have a "decode token" option that decodes JWT tokens

// ViewOptions are the options for viewing a configuration file
type ViewOptions struct {
	// Config is the Red Sky Configuration to view
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// TODO Minify?
	// TODO Output format (e.g. json,yaml,env)? Templating?
}

// NewViewCommand creates a new command for viewing the configuration
func NewViewCommand(o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the configuration file",
		Long:  "View the Red Sky Ops configuration file",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.view),
	}

	commander.ExitOnError(cmd)
	return cmd
}

func (o *ViewOptions) view() error {
	output, err := yaml.Marshal(o.Config)
	if err != nil {
		return err
	}

	_, err = o.Out.Write(output)
	return err
}
