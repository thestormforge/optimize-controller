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

	"github.com/redskyops/redskyops-controller/internal/version"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/authorize_cluster"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/grant_permissions"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/redskyops/redskyops-go/pkg/config"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/filters/labels"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// GeneratorOptions are the configuration options for generating the controller installation
type GeneratorOptions struct {
	// Config is the Red Sky Configuration used to generate the controller installation
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool
	NamespaceSelector       string

	Image              string
	SkipControllerRBAC bool
	SkipSecret         bool

	// labels are currently private use for `redskyctl init` only
	labels map[string]string
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

	o.addFlags(cmd)

	commander.ExitOnError(cmd)
	return cmd
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
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
}

func (o *GeneratorOptions) generate(_ context.Context) error {
	// Generate the primary application manifests
	app, err := o.generateApplication()
	if err != nil {
		return err
	}

	// Build a pipeline that will produce the final manifests
	p := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: app},
		},
		Filters: []kio.Filter{
			o.clusterRoleBindingFilter(),
			o.labelFilter(),
		},
		Outputs: []kio.Writer{
			kio.ByteWriter{Writer: o.Out},
		},
	}

	if !o.SkipControllerRBAC {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateControllerRBAC()})
	}

	if !o.SkipSecret {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateSecret()})
	}

	return p.Execute()
}

// clusterRoleBindingFilter returns a filter that removes cluster role bindings for namespaced deployments
func (o *GeneratorOptions) clusterRoleBindingFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		if o.NamespaceSelector == "" {
			return nodes, nil
		}

		output := make([]*yaml.RNode, 0, len(nodes))
		for i := range nodes {
			m, err := nodes[i].GetMeta()
			if err != nil {
				return nil, err
			}

			if m.Kind != "ClusterRoleBinding" || m.APIVersion == "rbac.authorization.k8s.io/v1" {
				output = append(output, nodes[i])
			}
		}
		return output, nil
	})
}

// labelFilter returns a filter that applies the configured labels
func (o *GeneratorOptions) labelFilter() kio.Filter {
	f := labels.Filter{
		Labels: map[string]string{
			"app.kubernetes.io/version": version.GetInfo().Version,
		},
		FsSlice: []types.FieldSpec{
			{
				Gvk: resid.Gvk{
					Kind: "Deployment",
				},
				Path:               "spec/template/metadata/labels",
				CreateIfNotPresent: true,
			},
			{
				Path:               "metadata/labels",
				CreateIfNotPresent: true,
			},
		},
	}

	for k, v := range o.labels {
		f.Labels[k] = v
	}

	return f
}

func (o *GeneratorOptions) generateApplication() (io.Reader, error) {
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
		kustomize.WithAPI(apiEnabled),
	)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(yamls), nil
}

func (o *GeneratorOptions) generateControllerRBAC() io.Reader {
	opts := grant_permissions.GeneratorOptions{
		Config:                o.Config,
		SkipDefault:           !o.IncludeBootstrapRole,
		CreateTrialNamespaces: o.IncludeExtraPermissions,
		NamespaceSelector:     o.NamespaceSelector,
		IncludeManagerRole:    true,
	}
	return o.newStdoutReader(grant_permissions.NewGeneratorCommand(&opts))
}

func (o *GeneratorOptions) generateSecret() io.Reader {
	opts := authorize_cluster.GeneratorOptions{
		Config:            o.Config,
		AllowUnauthorized: true,
	}
	return o.newStdoutReader(authorize_cluster.NewGeneratorCommand(&opts))
}

// newStdoutReader returns an io.Reader which will execute the supplied command on the first read
func (o *GeneratorOptions) newStdoutReader(cmd *cobra.Command) io.Reader {
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
