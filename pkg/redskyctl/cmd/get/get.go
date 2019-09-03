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
package get

import (
	"fmt"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	getLong    = `Display one or many Red Sky resources`
	getExample = ``
)

type GetOptions struct {
	ForceRedSkyAPI  bool
	ForceKubernetes bool

	Namespace string
	Name      string
	ChunkSize int

	Printer         cmdutil.ResourcePrinter
	RedSkyAPI       *redsky.API
	RedSkyClientSet *redskykube.Clientset

	cmdutil.IOStreams
}

func NewGetOptions(ioStreams cmdutil.IOStreams) *GetOptions {
	return &GetOptions{
		IOStreams: ioStreams,
	}
}

func NewGetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Display a Red Sky resource",
		Long:    getLong,
		Example: getExample,
	}
	cmd.Run = cmd.HelpFunc()

	cmd.AddCommand(NewGetExperimentListCommand(f, ioStreams))
	cmd.AddCommand(NewGetTrialListCommand(f, ioStreams))

	return cmd
}

func (o *GetOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&o.ChunkSize, "chunk-size", 500, "Fetch large lists in chunks rather then all at once.")
}

func (o *GetOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string, printFlags *cmdutil.PrintFlags) error {
	if !o.ForceKubernetes {
		if api, err := f.RedSkyAPI(); err == nil {
			// Get from the remote Red Sky API
			o.RedSkyAPI = &api
		} else if o.ForceRedSkyAPI {
			// Failure to explicitly use the Red Sky API
			return err
		}
	}

	if o.RedSkyAPI == nil {
		if cs, err := f.RedSkyClientSet(); err == nil {
			// Get from the Kube cluster
			o.RedSkyClientSet = cs

			// Get the namespace to use
			o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
			}
		} else if o.ForceKubernetes {
			// Failure to explicitly use the Kubernetes cluster
			return err
		}
	}

	if o.RedSkyAPI == nil && o.RedSkyClientSet == nil {
		return fmt.Errorf("unable to connect")
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	if p, err := printFlags.ToPrinter(); err != nil {
		return err
	} else {
		o.Printer = p
	}

	if o.ChunkSize < 0 {
		o.ChunkSize = 0
	}

	return nil
}

// Helper to invoke PrintObj or propagate the result of a multi-return call
func (o *GetOptions) printIf(obj interface{}, err error) error {
	if err != nil {
		return err
	}

	return o.Printer.PrintObj(obj, o.Out)
}
