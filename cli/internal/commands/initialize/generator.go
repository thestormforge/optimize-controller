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
	"html/template"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/authorize_cluster"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/grant_permissions"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/kustomize"
	"github.com/thestormforge/optimize-controller/v2/internal/setup"
	"github.com/thestormforge/optimize-go/pkg/config"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

// GeneratorOptions are the configuration options for generating the controller installation
type GeneratorOptions struct {
	// Config is the Optimize Configuration used to generate the controller installation
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	IncludeBootstrapRole    bool
	IncludeExtraPermissions bool
	NamespaceSelector       string
	OutputDirectory         string

	Image              string
	SkipControllerRBAC bool
	SkipSecret         bool

	// labels are currently private use for `stormforge init` only
	labels map[string]string
}

// NewGeneratorCommand creates a command for generating the controller installation
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate Optimize manifests",
		Long:  "Generate installation manifests for StormForge Optimize",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVar(&o.OutputDirectory, "output-dir", o.OutputDirectory, "write files to a `directory` instead of stdout")
	o.addFlags(cmd)

	return cmd
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.IncludeBootstrapRole, "bootstrap-role", o.IncludeBootstrapRole, "create the bootstrap role")
	cmd.Flags().BoolVar(&o.IncludeExtraPermissions, "extra-permissions", o.IncludeExtraPermissions, "generate permissions required for features like namespace creation")
	cmd.Flags().StringVar(&o.NamespaceSelector, "ns-selector", o.NamespaceSelector, "create namespaced role bindings to matching namespaces")

	// Add hidden options
	cmd.Flags().StringVar(&o.Image, "image", kustomize.BuildImage, "specify the controller image to use")
	cmd.Flags().BoolVar(&o.SkipControllerRBAC, "skip-controller-rbac", o.SkipControllerRBAC, "skip generation of additional controller roles")
	cmd.Flags().BoolVar(&o.SkipSecret, "skip-secret", o.SkipSecret, "skip generation of secret")
	_ = cmd.Flags().MarkHidden("image")
	_ = cmd.Flags().MarkHidden("skip-controller-rbac")
	_ = cmd.Flags().MarkHidden("skip-secret")
}

func (o *GeneratorOptions) generate() error {
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
			o.labelFilter(),
		},
		Outputs: []kio.Writer{
			kio.ByteWriter{Writer: o.Out},
		},
	}

	if !o.SkipControllerRBAC {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateControllerRBAC()})
	}

	if o.SkipSecret {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generatePlaceholderSecret()})
	} else {
		p.Inputs = append(p.Inputs, &kio.ByteReader{Reader: o.generateSecret()})
	}

	if o.NamespaceSelector != "" {
		p.Filters = append(p.Filters, o.clusterRoleBindingFilter())
	}

	if o.OutputDirectory != "" {
		if err := os.MkdirAll(o.OutputDirectory, 0700); err != nil {
			return err
		}
		p.Outputs = []kio.Writer{kio.LocalPackageWriter{
			PackagePath: o.OutputDirectory,
		}}
	}

	return p.Execute()
}

// generateApplication produces the primary application manifests via an in-memory kustomization of
// go-generated assets computed during the build.
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
		kustomize.WithImagePullPolicy(setup.ImagePullPolicy),
		kustomize.WithAPI(apiEnabled),
	)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(yamls), nil
}

// generateControllerRBAC produces the RBAC manifests by invoking the
// `grant_permissions` generator in memory.
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

// generateSecret produces the Experiments API credentials secret manifest by invoking the
// `authorize_cluster` generator in memory.
func (o *GeneratorOptions) generateSecret() io.Reader {
	opts := authorize_cluster.GeneratorOptions{
		Config:            o.Config,
		AllowUnauthorized: true,
	}
	return o.newStdoutReader(authorize_cluster.NewGeneratorCommand(&opts))
}

// generatePlaceholderSecret produces an empty secret object with the expected name for the
// deployment's `envFrom` reference. Note that when making changes to the placeholder in
// the cluster you must remember to delete pods to have the changes picked up.
func (o *GeneratorOptions) generatePlaceholderSecret() io.Reader {
	r := &stdoutReader{}
	r.exec = func() error {
		ctrl, err := config.CurrentController(o.Config.Reader())
		if err != nil {
			return err
		}

		tmpl := template.Must(template.New("secret").Parse(`apiVersion: v1
kind: Secret
metadata:
  name: optimize-manager
  namespace: {{ .Namespace }}
type: Opaque
data: {}
`))

		return tmpl.Execute(&r.stdout, &ctrl)
	}
	return r
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
