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

package grant_permissions

import (
	"context"
	"os/exec"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// Options are the configuration options for granting controller permissions
type Options struct {
	GeneratorOptions
}

// NewCommand creates a new command for granting controller permissions
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grant-permissions",
		Short: "Grant permissions",
		Long:  "Grant the Red Sky Controller permissions",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.grantPermissions),
	}

	o.addFlags(cmd)

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) grantPermissions(ctx context.Context) error {
	// Pipe the generator output into `kubectl apply`
	generator := func() (*cobra.Command, error) { return NewGeneratorCommand(&o.GeneratorOptions), nil }
	apply := func() (*exec.Cmd, error) { return o.Config.Kubectl(ctx, "apply", "-f", "-") }
	return commander.RunPipe(o.IOStreams, generator, apply, nil)
}
