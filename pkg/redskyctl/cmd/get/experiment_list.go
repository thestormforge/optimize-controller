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
	"context"
	"path"
	"strings"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/experiment"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	getExperimentListLong    = `Prints a list of experiments using a tabular format by default`
	getExperimentListExample = ``
)

func NewGetExperimentListCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGetOptions(ioStreams)

	printFlags := cmdutil.NewPrintFlags(&experimentTableMeta{})

	cmd := &cobra.Command{
		Use:     "experiments",
		Short:   "Display a list of experiments",
		Long:    getExperimentListLong,
		Example: getExperimentListExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args, printFlags))
			cmdutil.CheckErr(RunGetExperimentList(o))
		},
	}

	o.AddFlags(cmd)
	printFlags.AddFlags(cmd)

	return cmd
}

func RunGetExperimentList(o *GetOptions) error {
	var list *redsky.ExperimentList
	var err error
	if o.RedSkyAPI != nil {
		list, err = getRedSkyAPIExperimentList(*o.RedSkyAPI, o.ChunkSize)
	} else if o.RedSkyClientSet != nil {
		list, err = getKubernetesExperimentList(o.RedSkyClientSet, o.Namespace, o.ChunkSize)
	} else {
		return nil
	}
	if err != nil {
		return err
	}

	return o.Printer.PrintObj(list, o.Out)
}

func getRedSkyAPIExperimentList(api redsky.API, chunkSize int) (*redsky.ExperimentList, error) {
	l, err := api.GetAllExperiments(context.TODO(), &redsky.ExperimentListQuery{Limit: chunkSize})
	if err != nil {
		return nil, err
	}

	n := l
	for n.Next != "" {
		if n, err = api.GetAllExperimentsByPage(context.TODO(), n.Next); err != nil {
			return nil, err
		}
		l.Experiments = append(l.Experiments, n.Experiments...)
	}

	return &l, nil
}

func getKubernetesExperimentList(clientset *redskykube.Clientset, namespace string, chunkSize int) (*redsky.ExperimentList, error) {
	experiments := clientset.RedskyopsV1alpha1().Experiments(namespace)
	opts := metav1.ListOptions{Limit: int64(chunkSize)}
	l := &redsky.ExperimentList{}
	for opts.Continue == "" {
		el, err := experiments.List(opts)
		if err != nil {
			return nil, err
		}

		err = experiment.ConvertExperimentList(el, l)
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

type experimentTableMeta struct{}

func (*experimentTableMeta) IsListType(obj interface{}) bool {
	if _, ok := obj.(*redsky.ExperimentList); ok {
		return true
	}
	return false
}

func (*experimentTableMeta) ExtractList(obj interface{}) ([]interface{}, error) {
	switch o := obj.(type) {
	case *redsky.ExperimentList:
		list := make([]interface{}, len(o.Experiments))
		for i := range o.Experiments {
			list[i] = &o.Experiments[i]
		}
		return list, nil
	default:
		return []interface{}{obj}, nil
	}
}

func (*experimentTableMeta) ExtractValue(obj interface{}, column string) (string, error) {
	switch o := obj.(type) {
	case *redsky.ExperimentItem:
		switch column {
		case "name":
			return path.Base(o.Self), nil
		}
	}
	// TODO Is this an error?
	return "", nil
}

func (*experimentTableMeta) Allow(outputFormat string) bool {
	return strings.ToLower(outputFormat) != "csv"
}

func (*experimentTableMeta) Columns(outputFormat string) []string {
	return []string{"name"}
}

func (*experimentTableMeta) Header(outputFormat string, column string) string {
	return strings.ToUpper(column)
}
