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
	"os"
	"path/filepath"

	redskyappsv1alpha1 "github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	"github.com/redskyops/redskyops-controller/pkg/application"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
)

type ExperimentOptions struct {
	experiments.Options

	Filename   string
	Resources  []string
	Scenario   string
	Objectives []string
}

// Other possible options:
// Have the option to create (or not create?) untracked metrics (e.g. CPU and Memory requests along side Cost)

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

func (o *ExperimentOptions) generate() error {
	g := experiment.NewGenerator(filesys.MakeFsOnDisk())

	if o.Filename != "" {
		r, err := o.IOStreams.OpenFile(o.Filename)
		if err != nil {
			return err
		}

		rr := commander.NewResourceReader()
		if err := rr.ReadInto(r, &g.Application); err != nil {
			return err
		}
	}

	if err := o.filterResources(&g.Application); err != nil {
		return err
	}

	if err := application.FilterScenarios(&g.Application, o.Scenario); err != nil {
		return err
	}

	if err := application.FilterObjectives(&g.Application, o.Objectives); err != nil {
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
	list, err := g.Generate()
	if err != nil {
		return err
	}

	return o.Printer.PrintObj(list, o.Out)
}

func (o *ExperimentOptions) filterResources(app *redskyappsv1alpha1.Application) error {
	// Add additional resources (this allows addition manifests to be added when invoking the CLI)
	app.Resources = append(app.Resources, o.Resources...)

	// Check to see if there is a Kustomization root at the same location as the file
	if len(app.Resources) == 0 && o.Filename != "" {
		dir := filepath.Dir(o.Filename)
		for _, n := range konfig.RecognizedKustomizationFileNames() {
			if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
				app.Resources = append(app.Resources, dir)
				break
			}
		}
	}

	return nil
}
