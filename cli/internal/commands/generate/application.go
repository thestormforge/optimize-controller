/*
Copyright 2021 GramLabs, Inc.

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

package generate

import (
	"os"

	"github.com/spf13/cobra"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/application"
	"github.com/thestormforge/optimize-go/pkg/config"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type ApplicationOptions struct {
	// Config is the Optimize Configuration used to generate the application
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Generator       application.Generator
	Resources       []string
	DefaultResource konjurev1beta2.Kubernetes
}

func NewApplicationCommand(o *ApplicationOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "application",
		Aliases: []string{"app"},
		Short:   "Generate an application",
		Long:    "Generate an application descriptor",

		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.Generator.DefaultReader = cmd.InOrStdin()
			o.Generator.WorkingDirectory, err = os.Getwd()
			return
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVar(&o.Generator.Name, "name", "", "set the application `name`")
	cmd.Flags().StringSliceVar(&o.Generator.Goals, "goals", nil, "specify the application optimization objective")
	cmd.Flags().BoolVar(&o.Generator.Documentation.Disabled, "no-comments", false, "suppress documentation comments on output")
	cmd.Flags().StringVar(&o.Generator.ScenarioFile, "test-case-file", "", "specify either a StormForger (.js) or Locust (.py) test case `file`")
	cmd.Flags().StringArrayVarP(&o.Resources, "resources", "r", nil, "additional resources to consider")
	cmd.Flags().StringArrayVar(&o.DefaultResource.Namespaces, "namespace", nil, "select resources from a specific namespace")
	cmd.Flags().StringVar(&o.DefaultResource.NamespaceSelector, "ns-selector", "", "`sel`ect resources from labeled namespaces")
	cmd.Flags().StringVarP(&o.DefaultResource.Selector, "selector", "l", "", "`sel`ect only labeled resources")

	_ = cmd.MarkFlagFilename("test-case-file", "js", "py")

	return cmd
}

func (o *ApplicationOptions) generate() error {
	if len(o.Resources) > 0 {
		// Add explicitly requested resources
		o.Generator.Resources = append(o.Generator.Resources, konjure.NewResource(o.Resources...))
	} else if o.DefaultResource.Selector == "" && o.Generator.Name != "" {
		// Add a default the label selector based on the name
		o.DefaultResource.Selector = "app.kubernetes.io/name=" + o.Generator.Name
	}

	// Only include the default resource if it has values
	if !o.isDefaultResourceEmpty() {
		o.Generator.Resources = append(o.Generator.Resources, konjure.Resource{Kubernetes: &o.DefaultResource})
	}

	// Generate the application
	return o.Generator.Execute(&kio.ByteWriter{Writer: o.Out})
}

func (o *ApplicationOptions) isDefaultResourceEmpty() bool {
	// TODO Should we check that `kubectl` is available and can return something meaningful?
	return len(o.DefaultResource.Namespaces) == 0 &&
		o.DefaultResource.NamespaceSelector == "" &&
		len(o.DefaultResource.Types) == 0 &&
		o.DefaultResource.Selector == ""
}
