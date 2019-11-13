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
	"reflect"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"
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
	Selector  string
	SortBy    string

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
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", "", "Selector to filter on.")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", "", "Sort list types using this JSONPath expression.")
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
		return fmt.Errorf("unable to connect, make sure either your Red Sky API or Kube configuration is valid")
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

// Helper to sort using a JSONPath expression
func sortByField(sortBy string, item func(int) interface{}) func(int, int) bool {
	field := sortBy // TODO Make "{}" and leading "." optional

	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	if err := parser.Parse(field); err != nil {
		return nil
	}

	return func(i, j int) bool {
		var iField, jField reflect.Value
		if r, err := parser.FindResults(item(i)); err != nil || len(r) == 0 || len(r[0]) == 0 {
			return true
		} else {
			iField = r[0][0]
		}
		if r, err := parser.FindResults(item(j)); err != nil || len(r) == 0 || len(r[0]) == 0 {
			return false
		} else {
			jField = r[0][0]
		}
		less, _ := isLess(iField, jField)
		return less
	}
}

// Compares to values, only int64, float64, and string are allowed
func isLess(i, j reflect.Value) (bool, error) {
	switch i.Kind() {
	case reflect.Int64:
		return i.Int() < j.Int(), nil
	case reflect.Float64:
		return i.Float() < j.Float(), nil
	case reflect.String:
		return i.String() < j.String(), nil // TODO Improve the sort order
	default:
		return false, fmt.Errorf("unsortable type: %v", i.Kind())
	}
}
