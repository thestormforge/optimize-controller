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
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/internal/version"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Options is the configuration for initialization
type Options struct {
	GeneratorOptions

	Wait                    bool
	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool
	NamespaceSelector       string
	Image                   string
	SkipControllerRBAC      bool
	SkipSecret              bool
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

	cmd.Flags().BoolVar(&o.Wait, "wait", o.Wait, "Wait for resources to be established before returning.")
	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "Create the bootstrap role (if it does not exist).")
	cmd.Flags().BoolVar(&o.IncludeExtraPermissions, "extra-permissions", o.IncludeExtraPermissions, "Generate permissions required for features like namespace creation")
	cmd.Flags().StringVar(&o.NamespaceSelector, "ns-selector", o.NamespaceSelector, "Create namespaced role bindings to matching namespaces.")

	// Add hidden options
	cmd.Flags().StringVar(&o.Image, "image", kustomize.BuildImage, "Specify the controller image to use.")
	cmd.Flags().BoolVar(&o.SkipControllerRBAC, "skip-controller-rbac", o.SkipControllerRBAC, "Skip generation of additional controller roles.")
	cmd.Flags().BoolVar(&o.SkipSecret, "skip-secret", o.SkipSecret, "Skip generation of secret.")
	_ = cmd.Flags().MarkHidden("image")
	_ = cmd.Flags().MarkHidden("skip-controller-rbac")
	_ = cmd.Flags().MarkHidden("skip-secret")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) initialize(ctx context.Context) error {
	var manifests bytes.Buffer

	install, err := o.generateInstall(ctx)
	if err != nil {
		return err
	}

	// Generate all of the manifests using a kyaml pipeline
	p := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: install}},
		Filters: []kio.Filter{kio.FilterFunc(o.filter)},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &manifests}},
	}
	if !o.SkipControllerRBAC {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateControllerRBAC()})
	}
	if !o.SkipSecret {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateSecret()})
	}

	// Execute the pipeline to populate the manifests buffer
	if err := p.Execute(); err != nil {
		return err
	}

	// Run `kubectl apply` to install the product
	// TODO Handle upgrades with "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s"
	kubectlApply, err := o.Config.Kubectl(ctx, "apply", "-f", "-")
	if err != nil {
		return err
	}
	kubectlApply.Stdout = o.Out
	kubectlApply.Stderr = o.ErrOut
	kubectlApply.Stdin = &manifests
	if err := kubectlApply.Run(); err != nil {
		return err
	}

	// Run `kubectl wait` to ensure the CRD is installed
	if o.Wait {
		kubectlWait, err := o.Config.Kubectl(ctx, "wait", "crd/experiments.redskyops.dev", "crd/trials.redskyops.dev", "--for", "condition=Established")
		if err != nil {
			return err
		}
		if err := kubectlWait.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (o *Options) generateInstall(ctx context.Context) (io.Reader, error) {
	r := o.Config.Reader()
	ctrl, err := config.CurrentController(r)
	if err != nil {
		return nil, err
	}

	auth, err := config.CurrentAuthorization(r)
	if err != nil {
		return nil, err
	}

	apiEnabled := false
	if auth.Credential.TokenCredential != nil {
		apiEnabled = true
	}

	yamls, err := kustomize.Yamls(
		kustomize.WithInstall(),
		kustomize.WithNamespace(ctrl.Namespace),
		kustomize.WithImage(o.Image),
		kustomize.WithLabels(map[string]string{
			"app.kubernetes.io/version":    version.GetInfo().Version,
			"app.kubernetes.io/managed-by": "redskyctl",
		}),
		kustomize.WithAPI(apiEnabled),
	)

	if err != nil {
		return nil, err
	}

	return bytes.NewReader(yamls), nil
}

func (o *Options) generateControllerRBAC() io.Reader {
	opts := grant_permissions.GeneratorOptions{
		Config:                o.Config,
		SkipDefault:           !o.IncludeBootstrapRole,
		CreateTrialNamespaces: o.IncludeExtraPermissions,
		NamespaceSelector:     o.NamespaceSelector,
		IncludeManagerRole:    true,
	}
	return o.newStdoutReader(grant_permissions.NewGeneratorCommand(&opts))
}

func (o *Options) generateSecret() io.Reader {
	opts := authorize_cluster.GeneratorOptions{
		Config:            o.Config,
		AllowUnauthorized: true,
	}
	return o.newStdoutReader(authorize_cluster.NewGeneratorCommand(&opts))
}

// filter adjusts the generated initialization resources as necessary
func (o *Options) filter(input []*yaml.RNode) ([]*yaml.RNode, error) {
	// TODO We should eliminate the "/config/install" Kustomization and just do everything here

	if o.NamespaceSelector == "" {
		return input, nil
	}

	// If there is a namespace filter, we must remove cluster role bindings
	var output kio.ResourceNodeSlice
	for i := range input {
		m, err := input[i].GetMeta()
		if err != nil {
			return nil, err
		}
		if m.Kind == "ClusterRoleBinding" && m.APIVersion == "rbac.authorization.k8s.io/v1" {
			continue
		}
		output = append(output, input[i])
	}
	return output, nil
}

// newStdoutReader returns an io.Reader which will execute the supplied command on the first read
func (o *Options) newStdoutReader(cmd *cobra.Command) io.Reader {
	r := &stdoutReader{}
	r.exec = cmd.Execute    // This is the function invoked once to populate the buffer
	cmd.SetOut(&r.stdout)   // Have the command write to our buffer
	cmd.SetErr(o.ErrOut)    // Have the command print error messages straight to our error stream
	cmd.SetArgs([]string{}) // Supply an explicit empty argument array so it doesn't get the OS arguments by default
	return r
}

type stdoutReader struct {
	stdout bytes.Buffer
	once   sync.Once
	exec   func() error
}

func (c *stdoutReader) Read(b []byte) (n int, err error) {
	c.once.Do(func() { err = c.exec() })
	if err != nil {
		return n, err
	}
	return c.stdout.Read(b)
}
