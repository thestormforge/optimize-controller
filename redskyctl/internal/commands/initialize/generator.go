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

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/setup"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// GeneratorOptions are the configuration options for generating the controller installation
type GeneratorOptions struct {
	// Config is the Red Sky Configuration used to generate the controller installation
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
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

	commander.ExitOnError(cmd)
	return cmd
}

func (o *GeneratorOptions) generate(ctx context.Context) error {
	// Read the initial information from the configuration
	r := o.Config.Reader()
	ctrl, err := config.CurrentController(r)
	if err != nil {
		return err
	}

	// Create an argument list to generate the installation manifests
	args := []string{"run", "redsky-bootstrap"}

	// Create a single attached pod
	args = append(args, "--restart", "Never", "--attach")

	// Quietly remove the pod when we are done
	args = append(args, "--rm", "--quiet")

	// Use the image embedded in the code
	args = append(args, "--image", setup.Image)
	// TODO We may need to overwrite this for offline clusters
	args = append(args, "--image-pull-policy", setup.ImagePullPolicy)

	// Do not allow the pod to access the API
	args = append(args, "--overrides", `{"spec":{"automountServiceAccountToken":false}}`)

	// Overwrite the "redsky-system" namespace
	args = append(args, "--env", "NAMESPACE="+ctrl.Namespace)

	// Arguments passed to the container
	args = append(args, "--", "install")

	// Run the command straight through to the configured output stream
	// TODO How do we filter out the warning about not being able to attach?
	kubectlRun, err := o.Config.Kubectl(ctx, args...)
	if err != nil {
		return err
	}
	kubectlRun.Stdout = o.Out
	kubectlRun.Stderr = o.ErrOut
	return kubectlRun.Run()
}
