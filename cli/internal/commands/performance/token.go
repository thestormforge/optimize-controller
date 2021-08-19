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

package performance

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thestormforge/konjure/pkg/konjure"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type TokenOptions struct {
	// Config is the Optimize Configuration to use.
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams.
	commander.IOStreams

	ShellOutput bool
}

// NewTokenCommand creates a new command for obtaining a Performance token.
func NewTokenCommand(o *TokenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "performance-token",
		Short:   "Generate StormForge Performance tokens",
		Long:    "Generate a token for accessing StormForge Performance Testing",
		Aliases: []string{"performance-env"},

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.ShellOutput = cmd.CalledAs() == "performance-env"
		},
		RunE: commander.WithContextE(o.generate),
	}

	return cmd
}

func (o *TokenOptions) generate(ctx context.Context) error {
	src, err := o.Config.PerformanceAuthorization(ctx)
	if err != nil {
		return err
	}

	t, err := src.Token()
	if err != nil {
		return err
	}

	if o.ShellOutput {
		return kio.Pipeline{
			Outputs: []kio.Writer{&konjure.EnvWriter{Writer: o.Out}},
			Inputs: []kio.Reader{sfio.ObjectSlice{&corev1.Secret{
				Data: map[string][]byte{"STORMFORGER_JWT": []byte(t.AccessToken)},
			}}},
		}.Execute()
	}

	_, _ = fmt.Fprintln(o.Out, t.AccessToken)
	return nil
}
