/*
Copyright 2021 GramLabs, Inc.

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

package scan

import (
	"bytes"
	"fmt"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

// Minikubectl is just a miniature in-process kubectl for us to use to avoid
// a binary dependency on the tool itself.
type Minikubectl struct {
	*genericclioptions.ConfigFlags
	*genericclioptions.ResourceBuilderFlags
	*genericclioptions.PrintFlags
	IgnoreNotFound bool
}

// NewMinikubectl creates a new minikubectl, the empty state is not usable.
func NewMinikubectl() *Minikubectl {
	outputFormat := ""
	return &Minikubectl{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		ResourceBuilderFlags: genericclioptions.NewResourceBuilderFlags().
			WithLabelSelector("").
			WithAll(true).
			WithLatest(),
		PrintFlags: &genericclioptions.PrintFlags{
			JSONYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
			OutputFormat:       &outputFormat,
		},
	}
}

// AddFlags configures the supplied flag set with the recognized flags.
func (k *Minikubectl) AddFlags(flags *pflag.FlagSet) {
	k.ConfigFlags.AddFlags(flags)
	k.ResourceBuilderFlags.AddFlags(flags)

	// PrintFlags somehow is tied to Cobra...
	flags.StringVarP(k.PrintFlags.OutputFormat, "output", "o", *k.PrintFlags.OutputFormat, "")

	// Don't bother with usage strings here, we aren't showing help to anyone
	flags.BoolVar(&k.IgnoreNotFound, "ignore-not-found", k.IgnoreNotFound, "")
}

// Complete validates we can execute against the supplied arguments.
func (k *Minikubectl) Complete(args []string) error {
	if len(args) == 0 || args[0] != "get" {
		return fmt.Errorf("minikubectl only supports get")
	}

	return nil
}

// Run executes the supplied arguments and returns the output as bytes.
func (k *Minikubectl) Run(args []string) ([]byte, error) {
	v := k.ResourceBuilderFlags.ToBuilder(k.ConfigFlags, args[1:]).Do()

	// Create a printer to dump the objects
	printer, err := k.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}

	// Use the printer to render everything into a byte buffer
	var b bytes.Buffer
	err = v.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			if k.IgnoreNotFound && apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		return printer.PrintObj(info.Object, &b)
	})
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
