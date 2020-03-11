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

package authorize_cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// Options are the configuration options for authorizing a cluster
type Options struct {
	GeneratorOptions
}

// NewCommand creates a new command for authorizing a cluster
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "authorize-cluster",
		Short: "Authorize a cluster",
		Long:  "Authorize Red Sky Ops in a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.authorizeCluster),
	}

	// TODO This is an argument for having "add flags" function that takes the command and the options struct...

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) authorizeCluster(ctx context.Context) error {
	// Capture a hash of the secret manifest as a way to force re-deployment of the manager
	h := sha256.New()

	// Pipe the generator output into `kubectl apply`
	generator := func() (*cobra.Command, error) { return NewGeneratorCommand(&o.GeneratorOptions), nil }
	apply := func() (*exec.Cmd, error) { return o.Config.Kubectl(ctx, "apply", "-f", "-") }
	sha256sum := func(w io.Writer) io.Writer { return io.MultiWriter(h, w) }
	if err := commander.RunPipe(o.IOStreams, generator, apply, sha256sum); err != nil {
		return err
	}

	// Patch the controller deployment using the hash of the secret
	if err := o.patchDeployment(ctx, hex.EncodeToString(h.Sum(nil))); err != nil {
		return err
	}

	return nil
}

// patchDeployment patches the Red Sky Controller deployment to reflect the state of the secret; any changes to the
// will cause the controller to be re-deployed.
func (o *Options) patchDeployment(ctx context.Context, secretHash string) error {
	// TODO In theory we wouldn't need this if we switched to file based configuration in the cluster and had a watch on the file to pick up changes...

	// TODO What about the controller deployment name? It could be different, e.g. for a Helm deploy
	name := "redsky-controller-manager"
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"redskyops.dev/secretHash":"%s"}}}}}`, secretHash)

	ctrl, err := config.CurrentController(o.Config.Reader())
	if err != nil {
		return err
	}

	kubectlPatch, err := o.Config.Kubectl(ctx, "patch", "deployment", name, "--namespace", ctrl.Namespace, "--patch", patch)
	if err != nil {
		return err
	}
	kubectlPatch.Stdout = o.Out
	kubectlPatch.Stderr = o.ErrOut
	return kubectlPatch.Run()
}
