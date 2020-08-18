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

package initialize

import (
	"context"
	"fmt"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/spf13/cobra"
)

// GeneratorOptions are the configuration options for generating the controller installation
type GeneratorOptions struct {
	// Config is the Red Sky Configuration used to generate the controller installation
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Image string
}

// NewGeneratorCommand creates a command for generating the controller installation
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate Red Sky Ops manifests",
		Long:  "Generate installation manifests for Red Sky Ops",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.generate),
	}

	cmd.Flags().StringVar(&o.Image, "image", kustomize.BuildImage, "Specify the controller image to use.")
	_ = cmd.Flags().MarkHidden("image")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *GeneratorOptions) generate(ctx context.Context) error {
	r := o.Config.Reader()
	ctrl, err := config.CurrentController(r)
	if err != nil {
		return err
	}

	auth, err := config.CurrentAuthorization(r)
	if err != nil {
		return err
	}

	apiEnabled := false
	if auth.Credential.TokenCredential != nil {
		apiEnabled = true
	}

	yamls, err := kustomize.Yamls(
		kustomize.WithInstall(),
		kustomize.WithImage(o.Image),
		kustomize.WithNamespace(ctrl.Namespace),
		kustomize.WithAPI(apiEnabled),
	)

	if err != nil {
		return err
	}

	fmt.Fprintln(o.Out, string(yamls))

	return nil
}
