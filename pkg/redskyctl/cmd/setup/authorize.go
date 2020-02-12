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

package setup

import (
	"fmt"

	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	authorizeLong    = `Authorize Red Sky Ops in a cluster`
	authorizeExample = ``
)

// AuthorizeOptions is the configuration for initialization
type AuthorizeOptions struct {
	Kubectl   *cmdutil.Kubectl
	Namespace string

	// TODO Add --envFile option that gets merged with the configuration environment variables
	// TODO Should we get information from other secrets in other namespaces?
	// TODO What about overriding the secret name to something we do not overwrite?

	cmdutil.IOStreams
}

// NewAuthorizeOptions returns a new authorization options struct
func NewAuthorizeOptions(ioStreams cmdutil.IOStreams) *AuthorizeOptions {
	return &AuthorizeOptions{
		Kubectl:   cmdutil.NewKubectl(),
		Namespace: "redsky-system",
		IOStreams: ioStreams,
	}
}

func NewAuthorizeCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewAuthorizeOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "authorize-cluster",
		Short:   "Authorize a cluster",
		Long:    authorizeLong,
		Example: authorizeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cmd, o.Complete(f, cmd, args))
			cmdutil.CheckErr(cmd, o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Override default namespace.")

	return cmd
}

func (o *AuthorizeOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *AuthorizeOptions) Run() error {
	// Generate and apply the secret
	applyCmd := o.Kubectl.NewCmd("apply", "-f", "-")
	applyCmd.Stdout = o.Out
	applyCmd.Stderr = o.ErrOut
	name, err := secret(o.Namespace, applyCmd)
	if err != nil {
		return err
	}

	// Patch the pod template to trigger an update when the configuration changes
	// TODO In theory we wouldn't need this if we had a watch on the file and could pick up changes...
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"redskyops.dev/secretHash":"%s"}}}}}`, name)
	patchArgs := []string{"patch", "deployment", "redsky-controller-manager", "--patch", patch}
	if o.Namespace != "" {
		patchArgs = append(patchArgs, "-n", o.Namespace)
	}
	patchCmd := o.Kubectl.NewCmd(patchArgs...)
	patchCmd.Stdout = o.Out
	patchCmd.Stderr = o.ErrOut
	return patchCmd.Run()
}
