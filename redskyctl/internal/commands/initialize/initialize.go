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

package initialize

import (
	"context"
	"os/exec"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/spf13/cobra"
)

// Options is the configuration for initialization
type Options struct {
	GeneratorOptions

	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool
}

// NewCommand creates a command for performing an initialization
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install to a cluster",
		Long:  "Install Red Sky Ops to a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.initialize),
	}

	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "Create the bootstrap role (if it does not exist).")
	cmd.Flags().BoolVar(&o.IncludeExtraPermissions, "extra-permissions", o.IncludeExtraPermissions, "Generate permissions required for features like namespace creation")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) initialize(ctx context.Context) error {
	// TODO We should be passing in labels/annotations to apply from here (e.g. managed-by redskyctl comes from here)

	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s"
	kubectlApply := func() (*exec.Cmd, error) { return o.Config.Kubectl(ctx, "apply", "-f", "-") }

	// Generate the main installation manifests
	initializeGenerator := func() (*cobra.Command, error) {
		return NewGeneratorCommand(&o.GeneratorOptions), nil
	}
	if err := commander.RunPipe(o.IOStreams, initializeGenerator, kubectlApply, nil); err != nil {
		return err
	}

	// Generate the permission manifests
	grantPermissionsOptions := grant_permissions.GeneratorOptions{
		Config:                o.Config,
		SkipDefault:           !o.IncludeBootstrapRole,
		CreateTrialNamespaces: o.IncludeExtraPermissions,
	}
	grantPermissionGenerator := func() (*cobra.Command, error) {
		return grant_permissions.NewGeneratorCommand(&grantPermissionsOptions), nil
	}
	if err := commander.RunPipe(o.IOStreams, grantPermissionGenerator, kubectlApply, nil); err != nil {
		return err
	}

	// Generate the authorization manifests
	authorizeClusterOptions := authorize_cluster.GeneratorOptions{
		Config: o.Config,
	}
	authorizeClusterGenerator := func() (*cobra.Command, error) {
		return authorize_cluster.NewGeneratorCommand(&authorizeClusterOptions), nil
	}
	if err := commander.RunPipe(o.IOStreams, authorizeClusterGenerator, kubectlApply, nil); err != nil {
		// TODO Ignore errors stemming from not having logged in yet
		return err
	}

	return nil
}
