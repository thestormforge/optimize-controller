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
	"io/ioutil"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/yaml"
)

const experimentConfigKind = "MagikExperiment"

type ExperimentOptions struct {
	experiments.Options

	ExperimentConfig experiment.MagikExperiment
	Filename         string

	Resources []string
}

func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "experiment",
		Short:   "Generate an experiment",
		Long:    "Generate an experiment using magik",
		Aliases: []string{experimentConfigKind},

		Annotations: map[string]string{
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
	exp := o.newExperiment()

	// Scan the resources and add the results into the experiment object
	s := &experiment.Scanner{
		FileSystem: filesys.MakeFsOnDisk(),
	}
	if err := s.ScanInto(o.ExperimentConfig.Resources, exp); err != nil {
		return err
	}

	// TODO Do some sanity checks to make sure the experiment is valid before we add it
	list.Items = append(list.Items, runtime.RawExtension{Object: exp})

	// TODO What other objects should we add to the list? Service account? RBAC?

	return o.Printer.PrintObj(list, o.Out)
}

func (o *ExperimentOptions) readConfig() error {
	// Read the configuration file
	b, err := ioutil.ReadFile(o.Filename)
	if err != nil {
		return err
	}

	// TODO We should be using the Kubernetes object decoder for this
	if err := yaml.Unmarshal(b, &o.ExperimentConfig); err != nil {
		return err
	}
	gvk := experiment.GroupVersion.WithKind(experimentConfigKind)
	if o.ExperimentConfig.GroupVersionKind() != gvk {
		return fmt.Errorf("incorrect input type: %s", o.ExperimentConfig.GroupVersionKind().String())
	}

	// Add additional resources
	o.ExperimentConfig.Resources = append(o.ExperimentConfig.Resources, o.Resources...)

	// TODO Should we expose additional overrides/merges on the CLI options? Like name?

	return nil
}

func (o *ExperimentOptions) newExperiment() *redskyv1beta1.Experiment {
	exp := &redskyv1beta1.Experiment{
		Spec: redskyv1beta1.ExperimentSpec{
			Metrics: []redskyv1beta1.Metric{},
			TrialTemplate: redskyv1beta1.TrialTemplateSpec{
				Spec: redskyv1beta1.TrialSpec{
					JobTemplate: &batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{},
								},
							},
						},
					},
					InitialDelaySeconds: 15,
					SetupTasks: []redskyv1beta1.SetupTask{
						{
							Name: "monitoring",
							Args: []string{"prometheus", "$(MODE}"},
						},
					},
					SetupServiceAccountName: "promsetup",
				},
			},
		},
	}

	// TODO Do we want to filter out any of this information?
	o.ExperimentConfig.ObjectMeta.DeepCopyInto(&exp.ObjectMeta)

	return exp
}
