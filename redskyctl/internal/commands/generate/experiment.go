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
	"fmt"
	"strings"

	"github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/api/filesys"
)

type ExperimentOptions struct {
	experiments.Options

	Filename   string
	Resources  []string
	Scenario   string
	Objectives []string
}

func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Generate an experiment",
		Long:  "Generate an experiment from an application descriptor",

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

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "file that contains the experiment configuration")
	cmd.Flags().StringArrayVarP(&o.Resources, "resources", "r", nil, "additional resources to consider")
	cmd.Flags().StringVarP(&o.Scenario, "scenario", "s", o.Scenario, "the application scenario to generate an experiment for")
	cmd.Flags().StringArrayVar(&o.Objectives, "objectives", o.Objectives, "the application objectives to generate an experiment for")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	commander.SetKubePrinter(&o.Printer, cmd, nil)
	return cmd
}

func (o *ExperimentOptions) generate() (err error) {
	// Create a new experiment generator
	g := &experiment.Generator{FileSystem: filesys.MakeFsOnDisk()}

	// Read the application
	g.Application, err = o.application()
	if err != nil {
		return err
	}

	// Validate the requested scenario against the application
	g.Scenario, err = o.scenario(g.Application)
	if err != nil {
		return err
	}

	// Validate the requested objectives against the application
	g.Objectives, err = o.objectives(g.Application)
	if err != nil {
		return err
	}

	// Configure how we filter the application resources when looking for requests/limits
	// TODO This is kind of a hack: we are just adding labels (if present) to the default selectors
	g.ContainerResourcesSelector = experiment.DefaultContainerResourcesSelectors()
	if g.Application.Parameters != nil && g.Application.Parameters.ContainerResources != nil {
		ls := labels.Set(g.Application.Parameters.ContainerResources.Labels).String()
		for i := range g.ContainerResourcesSelector {
			g.ContainerResourcesSelector[i].LabelSelector = ls
		}
	}

	// Generate the experiment
	list, err := g.GenerateExperiment()
	if err != nil {
		return err
	}

	// TODO Do some sanity checks to make sure everything is valid

	return o.Printer.PrintObj(list, o.Out)
}

func (o *ExperimentOptions) application() (*v1alpha1.Application, error) {
	app := &v1alpha1.Application{}

	// Read the configuration from disk if specified
	if o.Filename != "" {
		r, err := o.IOStreams.OpenFile(o.Filename)
		if err != nil {
			return nil, err
		}

		rr := commander.NewResourceReader()
		_ = v1alpha1.AddToScheme(rr.Scheme)
		if err := rr.ReadInto(r, app); err != nil {
			return nil, err
		}
	}

	// Add additional resources (this allows addition manifests to be added when invoking the CLI)
	app.Resources = append(app.Resources, o.Resources...)

	return app, nil
}

func (o *ExperimentOptions) scenario(app *v1alpha1.Application) (string, error) {
	switch len(app.Scenarios) {

	case 0:
		if o.Scenario == "" {
			return "", nil
		}
		return "", fmt.Errorf("unknown scenario '%s' (application has no scenarios defined)", o.Scenario)

	case 1:
		if o.Scenario == app.Scenarios[0].Name || o.Scenario == "" {
			return app.Scenarios[0].Name, nil
		}
		return "", fmt.Errorf("unknown scenario '%s' (must be %s)", o.Scenario, app.Scenarios[0].Name)

	default:
		names := make([]string, 0, len(app.Scenarios))
		for i := range app.Scenarios {
			if app.Scenarios[i].Name == o.Scenario {
				return o.Scenario, nil
			}
			names = append(names, app.Scenarios[i].Name)
		}
		return "", fmt.Errorf("unknown scenario '%s' (should be one of %s)", o.Scenario, strings.Join(names, ", "))

	}
}

func (o *ExperimentOptions) objectives(app *v1alpha1.Application) ([]string, error) {
	r := make(map[string]struct{}, len(o.Objectives))
	for _, o := range o.Objectives {
		r[o] = struct{}{}
	}

	names := make([]string, 0, len(o.Objectives))
	for i := range app.Objectives {
		if _, ok := r[app.Objectives[i].Name]; ok || len(o.Objectives) == 0 {
			names = append(names, app.Objectives[i].Name)
			delete(r, app.Objectives[i].Name)
		}
	}

	if len(r) > 0 {
		names = make([]string, 0, len(r))
		for k := range r {
			names = append(names, k)
		}
		return nil, fmt.Errorf("unknown objectives %s", strings.Join(names, ", "))
	}

	return names, nil
}
