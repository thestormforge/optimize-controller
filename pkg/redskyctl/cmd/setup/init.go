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
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	initLong    = `Install Red Sky Ops to a cluster`
	initExample = ``
)

// InitOptions is the configuration for initialization
type InitOptions struct {
	Kubectl              *cmdutil.Kubectl
	Namespace            string
	IncludeBootstrapRole bool

	// TODO Add --envFile option that gets merged with the configuration environment variables
	// TODO Should we get information from other secrets in other namespaces?
	// TODO What about overriding the secret name to something we do not overwrite?

	cmdutil.IOStreams
}

// NewInitOptions returns a new initialization options struct
func NewInitOptions(ioStreams cmdutil.IOStreams) *InitOptions {
	return &InitOptions{
		Kubectl:              cmdutil.NewKubectl(),
		IncludeBootstrapRole: true,
		IOStreams:            ioStreams,
	}
}

func NewInitCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewInitOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install to a cluster",
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Override default namespace.")
	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "Create the bootstrap role (if it does not exist).")

	return cmd
}

func (o *InitOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) Run() error {
	if err := o.install(); err != nil {
		return err
	}

	if err := o.bootstrapRole(); err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) install() error {
	env, err := o.generateManagerEnv()
	if err != nil {
		return err
	}

	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s"
	applyCmd := o.Kubectl.NewCmd("apply", "-f", "-")
	applyCmd.Stdout = o.Out
	applyCmd.Stderr = o.ErrOut
	return install(o.Kubectl, o.Namespace, env, applyCmd)
}

func (o *InitOptions) bootstrapRole() error {
	if !o.IncludeBootstrapRole {
		return nil
	}

	createCmd := o.Kubectl.NewCmd("create", "-f", "-")
	if err := bootstrapRole(o.Kubectl, createCmd); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// TODO We expect this to fail when the resource exists, but what about everything else?
			return nil
		}
		return err
	}
	return nil
}

func (o *InitOptions) generateManagerEnv() (io.Reader, error) {
	env := make(map[string]string)

	// Add environment variables from the default configuration
	cfg, err := api.DefaultConfig()
	if err != nil {
		return nil, err
	}
	env["REDSKY_ADDRESS"] = cfg.Address
	if cfg.OAuth2 != nil {
		env["REDSKY_OAUTH2_CLIENT_ID"] = cfg.OAuth2.ClientID
		env["REDSKY_OAUTH2_CLIENT_SECRET"] = cfg.OAuth2.ClientSecret
		env["REDSKY_OAUTH2_TOKEN_URL"] = cfg.OAuth2.TokenURL
	}
	if cfg.Manager != nil {
		for _, v := range cfg.Manager.Environment {
			env[v.Name] = v.Value
		}
	}

	// Serialize the environment map to a ".env" format
	b := &bytes.Buffer{}
	for k, v := range env {
		if v != "" {
			_, _ = fmt.Fprintf(b, "%s=%s\n", k, v)
		}
	}
	return b, nil
}
