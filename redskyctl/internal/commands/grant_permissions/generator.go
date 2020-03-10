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
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratorOptions are the configuration options for generating the controller role definitions
type GeneratorOptions struct {
	// Config is the Red Sky Configuration used to generate the authorization secret
	Config *config.RedSkyConfig
	// Printer is the resource printer used to render generated objects
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// SkipDefault bypasses the default permissions (get/patch on config maps, stateful sets, and deployments)
	SkipDefault bool
	// CreateTrialNamespaces includes additional permissions to allow the controller to create trial namespaces
	CreateTrialNamespaces bool
}

// NewGeneratorCommand creates a command for generating the controller role definitions
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap-cluster-role",
		Short: "Generate Red Sky Ops permissions",
		Long:  "Generate RBAC for Red Sky Ops",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.generate),
	}

	o.addFlags(cmd)

	commander.SetKubePrinter(&o.Printer, cmd)
	commander.ExitOnError(cmd)
	return cmd
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.SkipDefault, "skip-default", o.SkipDefault, "Skip default permissions.")
	cmd.Flags().BoolVar(&o.CreateTrialNamespaces, "create-trial-namespace", o.CreateTrialNamespaces, "Include trial namespace creation permissions.")
}

func (o *GeneratorOptions) generate() error {
	// The cluster role includes an aggregation label for a role that is already bound to the controller's service account
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redsky-aggregate-to-patching",
			Labels: map[string]string{
				"redskyops.dev/aggregate-to-patching": "true",
			},
		},
	}

	// Include the default rules
	if !o.SkipDefault {
		clusterRole.Rules = append(clusterRole.Rules,
			rbacv1.PolicyRule{
				Verbs:     []string{"get", "patch"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
			rbacv1.PolicyRule{
				Verbs:     []string{"get", "patch"},
				APIGroups: []string{"apps", "extensions"},
				Resources: []string{"deployments", "statefulsets"},
			})
	}

	// Trial namespace creation requires extra permissions
	if o.CreateTrialNamespaces {
		clusterRole.Rules = append(clusterRole.Rules,
			rbacv1.PolicyRule{
				Verbs:     []string{"create"},
				APIGroups: []string{""},
				Resources: []string{"namespaces,serviceaccounts"},
			},
		)
	}

	// Do not generate an empty cluster role
	if len(clusterRole.Rules) == 0 {
		// TODO Println("---")?
		return nil
	}

	return o.Printer.PrintObj(clusterRole, o.Out)
}
