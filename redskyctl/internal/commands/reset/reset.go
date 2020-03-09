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

package reset

import (
	"context"
	"os/exec"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/initialize"
	"github.com/spf13/cobra"
)

// ResetOptions is the configuration for suggesting assignments
type Options struct {
	// Config is the Red Sky Configuration used to generate the controller manifests for reset
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall from a cluster",
		Long:  "Uninstall Red Sky Ops from a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.reset),
	}

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) reset(ctx context.Context) error {
	kubectlDelete := func() (*exec.Cmd, error) { return o.Config.Kubectl(ctx, "delete", "--ignore-not-found", "-f", "-") }

	// Delete the CRDs first to avoid issues with the controller being deleted before it can remove the finalizers
	kubectlDeleteCRD := func() (cmd *exec.Cmd, err error) {
		return o.Config.Kubectl(ctx, "delete", "--ignore-not-found", "crd",
			"trials.redskyops.dev",
			"experiments.redskyops.dev",
		)
	}
	if err := commander.Run(o.IOStreams, kubectlDeleteCRD); err != nil {
		return err
	}

	// Delete the permission manifests
	grantPermissionsOptions := grant_permissions.GeneratorOptions{
		Config: o.Config,
	}
	grantPermissionGenerator := func() (*cobra.Command, error) {
		return grant_permissions.NewGeneratorCommand(&grantPermissionsOptions), nil
	}
	if err := commander.RunPipe(o.IOStreams, grantPermissionGenerator, kubectlDelete, nil); err != nil {
		return err
	}

	// Delete the main installation manifests
	initializeOptions := initialize.GeneratorOptions{
		Config: o.Config,
	}
	initializeGenerator := func() (*cobra.Command, error) {
		return initialize.NewGeneratorCommand(&initializeOptions), nil
	}
	if err := commander.RunPipe(o.IOStreams, initializeGenerator, kubectlDelete, nil); err != nil {
		return err
	}

	return nil
}
