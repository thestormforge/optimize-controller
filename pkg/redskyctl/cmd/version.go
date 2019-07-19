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
package cmd

import (
	"fmt"

	"github.com/redskyops/k8s-experiment/pkg/controller/trial"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/pkg/version"
	"github.com/spf13/cobra"
)

// TODO Add support for getting Red Sky server version
// TODO Add support for getting manager version in cluster
// TODO Add a "--notes" option to print the release notes?

const (
	versionLong    = `Show the version information for Red Sky Control.`
	versionExample = ``
)

type VersionOptions struct {
	SetupToolsImage bool

	root *cobra.Command

	cmdutil.IOStreams
}

func NewVersionOptions(ioStreams cmdutil.IOStreams) *VersionOptions {
	return &VersionOptions{
		IOStreams: ioStreams,
	}
}

func NewVersionCommand(ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewVersionOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Long:    versionLong,
		Example: versionExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.SetupToolsImage, "setuptools", false, "print only the name of the setuptools image")

	return cmd
}

func (o *VersionOptions) Complete(cmd *cobra.Command) error {
	o.root = cmd.Root()

	return nil
}

func (o *VersionOptions) Run() error {
	if o.SetupToolsImage {
		// TODO We should have an option to print this as JSON with the pull policy, e.g. `{"image":"...", "imagePullPolicy":"..."}`...
		_, err := fmt.Fprintf(o.Out, "%s\n", trial.DefaultImage)
		return err
	}

	_, err := fmt.Fprintf(o.Out, "%s version: %s\n", o.root.Name(), version.GetVersion())
	return err
}
