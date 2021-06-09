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

package authorize_cluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
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
		Long:  "Authorize StormForge Optimize in a cluster",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.authorizeCluster),
	}

	o.addFlags(cmd)

	return cmd
}

func (o *Options) authorizeCluster(ctx context.Context) error {
	// Fork `kubectl apply` and get a pipe to write manifests to
	kubectlApply, err := o.Config.Kubectl(ctx, "apply", "-f", "-")
	if err != nil {
		return err
	}
	kubectlApply.Stdout = o.Out
	kubectlApply.Stderr = o.ErrOut
	w, err := kubectlApply.StdinPipe()
	if err != nil {
		return err
	}

	// Generate the secret manifest (populating the name/hash of the secret as a side effect)
	var secretName, secretHash string
	go func() {
		// NOTE: Ignore errors and rely on logging to stderr
		defer func() { _ = w.Close() }()
		if err := o.generateSecret(w, &secretName, &secretHash); err != nil {
			return
		}
	}()

	// Apply the secret manifest (after this returns, the name/hash should be safely populated)
	if err := kubectlApply.Run(); err != nil {
		return err
	}

	// Patch the controller deployment using the hash of the secret
	if err := o.patchDeployment(ctx, secretName, secretHash); err != nil {
		return err
	}

	return nil
}

// generateSecret produces an authorization configuration secret, as a side effect the name and hash of
// the generated secret are used to populate the supplied string pointers
func (o *Options) generateSecret(out io.Writer, secretName, secretHash *string) error {
	h := sha256.New()

	opts := o.GeneratorOptions
	cmd := NewGeneratorCommand(&opts)
	cmd.SetArgs([]string{})
	cmd.SetOut(io.MultiWriter(h, out))
	cmd.SetErr(o.ErrOut)
	if err := cmd.Execute(); err != nil {
		return err
	}

	// Record the name and SHA-256 hash of the secret that was generated
	*secretName = opts.Name
	*secretHash = hex.EncodeToString(h.Sum(nil))
	return nil
}

// patchDeployment patches the Optimize Controller deployment to reflect the state of the secret; any changes to the
// will cause the controller to be re-deployed.
func (o *Options) patchDeployment(ctx context.Context, secretName, secretHash string) error {
	ctrl, err := config.CurrentController(o.Config.Reader())
	if err != nil {
		return err
	}

	// Generate the deployment patch using the secret name and hash
	var buf bytes.Buffer
	tmpl := template.Must(template.New("patch").Parse(patchTemplate))
	err = tmpl.Execute(&buf, map[string]string{
		"SecretName": secretName,
		"SecretHash": secretHash,
	})
	if err != nil {
		return err
	}

	// Execute the patch
	name := ctrl.DeploymentName
	namespace := ctrl.Namespace
	patch := buf.String()
	kubectlPatch, err := o.Config.Kubectl(ctx, "patch", "deployment", name, "--namespace", namespace, "--patch", patch)
	if err != nil {
		return err
	}
	kubectlPatch.Stdout = o.Out
	kubectlPatch.Stderr = o.ErrOut
	return kubectlPatch.Run()
}

// patchTemplate is used to patch the deployment with the secret information
const patchTemplate = `
spec:
  template:
    metadata:
      annotations:
        "stormforge.io/secretHash": "{{ .SecretHash }}"
    spec:
      containers:
      - name: manager
        envFrom:
        - secretRef:
            name: "{{ .SecretName }}"
`
