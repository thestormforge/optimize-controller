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

package generate

import (
	"io"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// TODO Add documentation about Kustomize v3, `--enable_alpha_plugins`, and what config files should look like
// TODO Should we have a "create scaffolding" command that has a stub Kustomize project?

const (
	generateLong    = `Generate Red Sky Ops object manifests`
	generateExample = ``
)

func NewGenerateCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate Red Sky Ops obejcts",
		Long:    generateLong,
		Example: generateExample,
	}
	cmd.Run = cmd.HelpFunc()

	cmd.AddCommand(NewGenerateExperimentCommand(ioStreams))
	cmd.AddCommand(NewGenerateInstallCmd(ioStreams))
	cmd.AddCommand(NewGenerateRBACCommand(ioStreams))

	return cmd
}

func serialize(e interface{}, w io.Writer) error {
	// This scheme is a subset with only the types that we are generating
	scheme := runtime.NewScheme()
	_ = rbacv1.AddToScheme(scheme)
	_ = redskyv1alpha1.AddToScheme(scheme)
	u := &unstructured.Unstructured{}
	if err := scheme.Convert(e, u, runtime.InternalGroupVersioner); err != nil {
		return err
	}
	if b, err := yaml.Marshal(u); err != nil {
		return err
	} else if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}
