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
	"io"
	"os"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// TODO Like the version command, support dumping the default configuration from the manager
// `kubectl exec -n redsky-system -c manager $(kubectl get pods -n redsky-system -o name) /manager config`
// TODO Add an option to output a Helm values.yaml for our chart
// TODO Have a "--minify" flag to just show the effective values available through the Reader
// TODO Output format (e.g. json,yaml,env)? Templating?

// ViewOptions are the options for viewing a configuration file
type ViewOptions struct {
	// Config is the Red Sky Configuration to view
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// FileOnly causes view to just dump the configuration file to out
	FileOnly bool
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

	cmd.Flags().BoolVar(&o.FileOnly, "raw", false, "Display the raw configuration file without merging.")
	cmd.Flags().BoolVar(&config.DecodeJWT, "decode-jwt", false, "Display JWT claims instead of raw token strings.")
	_ = cmd.Flags().MarkHidden("decode-jwt")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *ViewOptions) view() error {
	// Dump the raw config file bytes to the console
	if o.FileOnly {
		f, err := os.Open(o.Config.Filename)
		if err == nil {
			_, err = io.Copy(o.Out, f)
		}
		return err
	}

	// Marshal the configuration as YAML and write it out
	output, err := yaml.Marshal(o.Config)
	if err == nil {
		_, err = o.Out.Write(output)
	}
	return err
}
