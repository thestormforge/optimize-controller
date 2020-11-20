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
	"context"
	"os"
	"path/filepath"
	"strings"

	redskyappsv1alpha1 "github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	"github.com/redskyops/redskyops-controller/pkg/application"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-go/pkg/config"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
)

type ExperimentOptions struct {
	// Config is the Red Sky Configuration used to generate the role binding
	Config *config.RedSkyConfig
	// Printer is the resource printer used to render generated objects
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

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

	_ = cmd.MarkFlagRequired("filename")
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

	// Make sure there is an explicit namespace and name
	if g.Application.Namespace == "" {
		g.Application.Namespace = o.defaultNamespace()
	}
	if g.Application.Name == "" {
		g.Application.Name = o.defaultName()
	}

	// Configure how we filter the application resources when looking for requests/limits
	g.SetDefaultSelectors()

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

func (o *ExperimentOptions) defaultNamespace() string {
	// First check to see if we have an explicit namespace override set
	if cstr, err := config.CurrentCluster(o.Config.Reader()); err == nil && cstr.Namespace != "" {
		return cstr.Namespace
	}

	// Check the kubectl config output
	cmd, err := o.Config.Kubectl(context.Background(), "config", "view", "--minify", "-o", "jsonpath='{.contexts[0].context.namespace}'")
	if err == nil {
		if out, err := cmd.CombinedOutput(); err == nil {
			if ns := strings.TrimSpace(strings.Trim(string(out), "'")); ns != "" {
				return ns
			}
		}
	}

	// We cannot return empty, use default
	return "default"
}

func (o *ExperimentOptions) defaultName() string {
	// Use the directory name
	if af, err := filepath.Abs(o.Filename); err == nil {
		if d := filepath.Base(filepath.Dir(af)); d != "." {
			return d
		}
	}

	// We cannot return empty, use default
	return "default"
}
