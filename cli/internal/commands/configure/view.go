/*
Copyright 2020 GramLabs, Inc.

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

package configure

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	"sigs.k8s.io/yaml"
)

// TODO Like the version command, support dumping the default configuration from the manager
// `kubectl exec -n stormforge-system -c manager $(kubectl get pods -n stormforge-system -o name) /manager config`
// TODO Add an option to output a Helm values.yaml for our chart
// TODO Output format (e.g. json,yaml,env)? Templating?

// ViewOptions are the options for viewing a configuration file
type ViewOptions struct {
	// Config is the Optimize Configuration to view
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Output is the format to output data in
	Output string
	// FileOnly causes view to just dump the configuration file to out
	FileOnly bool
	// Minify causes the configuration to be evaluated and reduced to only the current effective configuration
	Minify bool
}

// NewViewCommand creates a new command for viewing the configuration
func NewViewCommand(o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the configuration file",
		Long:  "View the StormForge Optimize configuration file",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.view),
	}

	cmd.Flags().StringVarP(&o.Output, "output", "o", "yaml", "output `format`. One of: yaml|json")
	cmd.Flags().BoolVar(&o.FileOnly, "raw", false, "display the raw configuration file without merging")
	cmd.Flags().BoolVar(&o.Minify, "minify", false, "reduce information to effective values")
	cmd.Flags().BoolVar(&config.DecodeJWT, "decode-jwt", false, "display JWT claims instead of raw token strings")
	_ = cmd.Flags().MarkHidden("decode-jwt")

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

	// Reduce using the Reader
	if o.Minify {
		mini, err := config.Minify(o.Config.Reader())
		if err != nil {
			return err
		}
		output, err := yaml.Marshal(mini)
		if err == nil {
			_, err = o.Out.Write(output)
		}
		return err
	}

	// Marshal the configuration as YAML and write it out
	switch strings.ToLower(o.Output) {
	case "yaml", "":
		output, err := yaml.Marshal(o.Config)
		if err != nil {
			return err
		}
		if _, err := o.Out.Write(output); err != nil {
			return err
		}
	case "json":
		output, err := json.Marshal(o.Config)
		if err != nil {
			return err
		}
		if _, err := o.Out.Write(output); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported output format: %s", o.Output)
	}

	return nil
}
