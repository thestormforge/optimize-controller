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
	"io"
	"os"

	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/check"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/generate"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/setup"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/status"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/cmd/suggest"
	"github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

func NewDefaultRedskyctlCommand() *cobra.Command {
	return NewRedskyctlCommand(os.Stdin, os.Stdout, os.Stderr)
}

func NewRedskyctlCommand(in io.Reader, out, err io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "redskyctl",
	}
	rootCmd.Run = rootCmd.HelpFunc()

	flags := rootCmd.PersistentFlags()

	kubeConfigFlags := util.NewConfigFlags()
	kubeConfigFlags.AddFlags(flags)

	redskyConfigFlags := util.NewServerFlags()
	redskyConfigFlags.AddFlags(flags)

	f := util.NewFactory(kubeConfigFlags, redskyConfigFlags)

	ioStreams := util.IOStreams{In: in, Out: out, ErrOut: err}

	rootCmd.AddCommand(NewDocsCommand(ioStreams))
	rootCmd.AddCommand(NewVersionCommand(f, ioStreams))

	rootCmd.AddCommand(setup.NewInitCommand(f, ioStreams))
	rootCmd.AddCommand(setup.NewResetCommand(f, ioStreams))
	rootCmd.AddCommand(check.NewCheckCommand(f, ioStreams))
	rootCmd.AddCommand(suggest.NewSuggestCommand(f, ioStreams))
	rootCmd.AddCommand(generate.NewGenerateCommand(f, ioStreams))
	rootCmd.AddCommand(status.NewStatusCommand(f, ioStreams))

	// TODO Add 'backup' and 'restore' maintenance commands ('maint' subcommands?)
	// TODO Add API client commands for interacting with a remote server
	// TODO We need helpers for doing a "dry run" on patches to make configuration easier

	return rootCmd
}

// NewDefaultCommand is used for creating commands from the standard NewXCommand functions
func NewDefaultCommand(cmd func(f util.Factory, ioStreams util.IOStreams) *cobra.Command) *cobra.Command {
	ioStreams := util.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	// TODO Make a dummy implementation that returns reasonable errors
	var f util.Factory
	return cmd(f, ioStreams)
}
