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
package generate

import (
	"fmt"
	"io/ioutil"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// TODO `redskyctl kustomize edit add experiment`...?
// TODO Have the option to read a partial experiment from a file
// TODO Use patch conventions like Kubebuilder
// TODO Name pattern/name randomizer?
// TODO No-op "wait" patches, patches from files
// TODO Metric generators that can build PromQL queries for you
// TODO Parameter macros (e.g. template values that are replaced in Kustomize to make writing experiments more concise)
// TODO Resource limit/request tuning automatically, e.g. give it a deployment name, get a full experiment

const (
	generateExperimentLong    = `Generate an experiment manifest from a configuration file`
	generateExperimentExample = ``
)

type ExperimentGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type GenerateExperimentOptions struct {
	Config *ExperimentGenerator
	cmdutil.IOStreams
}

func NewGenerateExperimentOptions(ioStreams cmdutil.IOStreams) *GenerateExperimentOptions {
	return &GenerateExperimentOptions{
		IOStreams: ioStreams,
	}
}

func NewGenerateExperimentCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGenerateExperimentOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate experiments",
		Long:    generateExperimentLong,
		Example: generateExperimentExample,
		Args:    cobra.ExactArgs(1),
		Hidden:  true,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *GenerateExperimentOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	// TODO There are probably APIs for doing this if we register the experiment generator properly
	if b, err := ioutil.ReadFile(args[0]); err != nil {
		return err
	} else if err := yaml.Unmarshal(b, &o.Config); err != nil {
		return err
	}
	if o.Config.Kind != "ExperimentGenerator" {
		return fmt.Errorf("expected experiment generator, got: %s", o.Config.Kind)
	}
	return nil
}

func (o *GenerateExperimentOptions) Run() error {
	e := redskyv1alpha1.Experiment{}

	// TODO Populate this thing

	return serialize(&e, o.Out)
}
