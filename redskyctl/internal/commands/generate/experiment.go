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

package generate

import (
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/api/filesys"
)

type ExperimentOptions struct {
	experiments.Options

	Filename  string
	Resources []string
}

func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Generate an experiment",
		Long:  "Generate an experiment using magik",

		Annotations: map[string]string{
			"KustomizePluginKind":           "Application",
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
			commander.PrinterHideStatus:     "true",
			commander.PrinterStreamList:     "true",
		},

		PreRun: func(cmd *cobra.Command, args []string) {
			// Handle the case when we are invoked as a Kustomize exec plugin
			if cmd.CalledAs() == cmd.Annotations["KustomizePluginKind"] && len(args) == 1 {
				o.Filename = args[0]
			}
			commander.SetStreams(&o.IOStreams, cmd)
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "File that contains the experiment configuration.")
	cmd.Flags().StringArrayVarP(&o.Resources, "resources", "r", nil, "Additional resources to consider.")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	commander.SetKubePrinter(&o.Printer, cmd, nil)
	return cmd
}

func (o *ExperimentOptions) generate() error {
	// Read the experiment configuration
	app, err := o.readConfig()
	if err != nil {
		return err
	}

	// Create a new scanner
	s := &experiment.Scanner{App: app, FileSystem: filesys.MakeFsOnDisk()}

	// Configure how we filter the application resources when looking for requests/limits
	// TODO This is kind of a hack: we are just adding labels (if present) to the default selectors
	s.ContainerResourcesSelector = experiment.DefaultContainerResourcesSelectors()
	if app.Parameters != nil && app.Parameters.ContainerResources != nil {
		ls := labels.Set(app.Parameters.ContainerResources.Labels).String()
		for i := range s.ContainerResourcesSelector {
			s.ContainerResourcesSelector[i].LabelSelector = ls
		}
	}

	// Scan the resources and add the results into the experiment
	list := &corev1.List{}
	if err := s.ScanInto(list); err != nil {
		return err
	}

	// TODO Do some sanity checks to make sure everything is valid

	return o.Printer.PrintObj(list, o.Out)
}

func (o *ExperimentOptions) readConfig() (*experiment.Application, error) {
	app := &experiment.Application{}

	// Read the configuration from disk if specified
	if o.Filename != "" {
		r, err := o.IOStreams.OpenFile(o.Filename)
		if err != nil {
			return nil, err
		}

		rr := commander.NewResourceReader()
		_ = experiment.AddToScheme(rr.Scheme)
		if err := rr.ReadInto(r, app); err != nil {
			return nil, err
		}
	}

	// Add additional resources (this allows addition manifests to be added when invoking the CLI)
	app.Resources = append(app.Resources, o.Resources...)

	return app, nil
}
