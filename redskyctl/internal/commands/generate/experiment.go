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
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/filesys"
)

const experimentConfigKind = "Application"

type ExperimentOptions struct {
	experiments.Options

	Application experiment.Application
	Filename    string

	Resources []string
}

func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Generate an experiment",
		Long:  "Generate an experiment using magik",

		Annotations: map[string]string{
			"KustomizePluginKind":           experimentConfigKind,
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
			commander.PrinterHideStatus:     "true",
			commander.PrinterStreamList:     "true",
		},

		PreRun: func(cmd *cobra.Command, args []string) {
			// Handle the case when we are invoked as a Kustomize exec plugin
			if cmd.CalledAs() == experimentConfigKind && len(args) == 1 {
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
	commander.ExitOnError(cmd)
	return cmd
}

func (o *ExperimentOptions) generate() error {
	list := &corev1.List{}

	// Read the experiment configuration
	if err := o.readConfig(); err != nil {
		return err
	}

	// Start the experiment template
	exp := &redskyv1beta1.Experiment{}
	// TODO Do we want to filter out any of this information? Re-format it (e.g. "{appName}-{version}"?
	o.Application.ObjectMeta.DeepCopyInto(&exp.ObjectMeta)

	// Scan the resources and add the results into the experiment object
	if err := o.newScanner().ScanInto(exp); err != nil {
		return err
	}

	// Add the cost metric
	experiment.AddCostMetric(&o.Application, exp)

	// TODO Do some sanity checks to make sure the experiment is valid before we add it
	list.Items = append(list.Items, runtime.RawExtension{Object: exp})

	// TODO What other objects should we add to the list? Service account? RBAC?

	return o.Printer.PrintObj(list, o.Out)
}

func (o *ExperimentOptions) readConfig() error {
	// Read the configuration from disk if specified
	if o.Filename != "" {
		r, err := o.IOStreams.OpenFile(o.Filename)
		if err != nil {
			return err
		}

		rr := commander.NewResourceReader()
		_ = experiment.AddToScheme(rr.Scheme)
		if err := rr.ReadInto(r, &o.Application); err != nil {
			return err
		}
	}

	// Add additional resources
	o.Application.Resources = append(o.Application.Resources, o.Resources...)

	// TODO Should we expose additional overrides/merges on the CLI options? Like name?

	return nil
}

func (o *ExperimentOptions) newScanner() *experiment.Scanner {
	s := &experiment.Scanner{
		FileSystem:                 filesys.MakeFsOnDisk(),
		Resources:                  o.Application.Resources,
		ContainerResourcesSelector: experiment.DefaultContainerResourcesSelectors(),
	}

	if o.Application.Parameters != nil && o.Application.Parameters.ContainerResources != nil {
		ls := labels.Set(o.Application.Parameters.ContainerResources.Labels).String()
		for i := range s.ContainerResourcesSelector {
			s.ContainerResourcesSelector[i].LabelSelector = ls
		}
	}

	return s
}
