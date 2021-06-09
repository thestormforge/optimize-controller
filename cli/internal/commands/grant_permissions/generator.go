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
	"bufio"
	"bytes"
	"context"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GeneratorOptions are the configuration options for generating the controller role definitions
type GeneratorOptions struct {
	// Config is the Optimize Configuration used to generate the authorization secret
	Config *config.OptimizeConfig
	// Printer is the resource printer used to render generated objects
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// SkipDefault bypasses the default permissions (get/patch on config maps, stateful sets, and deployments)
	SkipDefault bool
	// CreateTrialNamespaces includes additional permissions to allow the controller to create trial namespaces
	CreateTrialNamespaces bool
	// NamespaceSelector generates namespaced bindings instead of cluster bindings
	NamespaceSelector string
	// IncludeManagerRole generates an additional binding to the manager role for each matched namespace
	IncludeManagerRole bool
}

// NewGeneratorCommand creates a command for generating the controller role definitions
func NewGeneratorCommand(o *GeneratorOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller-rbac",
		Short: "Generate Optimize permissions",
		Long:  "Generate RBAC for StormForge Optimize",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.generate),
	}

	o.addFlags(cmd)

	commander.SetKubePrinter(&o.Printer, cmd, nil)

	return cmd
}

func (o *GeneratorOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.SkipDefault, "skip-default", o.SkipDefault, "skip default permissions")
	cmd.Flags().BoolVar(&o.CreateTrialNamespaces, "create-trial-namespace", o.CreateTrialNamespaces, "include trial namespace creation permissions")
	cmd.Flags().StringVar(&o.NamespaceSelector, "ns-selector", o.NamespaceSelector, "bind to matching namespaces")
	cmd.Flags().BoolVar(&o.IncludeManagerRole, "include-manager", o.IncludeManagerRole, "bind manager to matching namespaces")
}

func (o *GeneratorOptions) generate(ctx context.Context) error {
	result := &corev1.List{}

	// Determine the binding targets
	roleRef, subject, err := o.bindingTargets()
	if err != nil {
		return err
	}

	// Generate the cluster role
	if clusterRole := o.generateClusterRole(roleRef); clusterRole != nil {
		result.Items = append(result.Items, runtime.RawExtension{Object: clusterRole})
	} else {
		// Do not generate bindings if we didn't end up creating a role
		roleRef = nil
	}

	// Generate the cluster role binding
	if clusterRoleBinding := o.generateClusterRoleBinding(roleRef, subject); clusterRoleBinding != nil {
		result.Items = append(result.Items, runtime.RawExtension{Object: clusterRoleBinding})
	}

	// Generate the role bindings
	roleBindings, err := o.generateRoleBindings(ctx, roleRef, subject)
	if err != nil {
		return err
	}
	for _, rb := range roleBindings {
		result.Items = append(result.Items, runtime.RawExtension{Object: rb})
	}

	// Print the result
	if len(result.Items) == 0 {
		return nil
	}
	return o.Printer.PrintObj(result, o.Out)
}

func (o *GeneratorOptions) bindingTargets() (*rbacv1.RoleRef, *rbacv1.Subject, error) {
	// Get the namespace of the controller from the configuration
	ctrl, err := config.CurrentController(o.Config.Reader())
	if err != nil {
		return nil, nil, err
	}

	// The manager runs as the default service account
	return &rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "optimize-patching-role",
		},
		&rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: ctrl.Namespace,
		}, nil
}

func (o *GeneratorOptions) generateClusterRole(roleRef *rbacv1.RoleRef) *rbacv1.ClusterRole {
	if roleRef == nil || (o.SkipDefault && !o.CreateTrialNamespaces) {
		return nil
	}

	clusterRole := &rbacv1.ClusterRole{}
	clusterRole.Name = roleRef.Name

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

	return clusterRole
}

func (o *GeneratorOptions) generateClusterRoleBinding(roleRef *rbacv1.RoleRef, subject *rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	if o.NamespaceSelector != "" || roleRef == nil || subject == nil {
		return nil
	}

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleRef.Name + "binding",
		},
		Subjects: []rbacv1.Subject{*subject},
		RoleRef:  *roleRef,
	}
}

func (o *GeneratorOptions) generateRoleBindings(ctx context.Context, roleRef *rbacv1.RoleRef, subject *rbacv1.Subject) ([]*rbacv1.RoleBinding, error) {
	if o.NamespaceSelector == "" || subject == nil {
		return nil, nil
	}

	// Get the namespaces matching the selector
	getCmd, err := o.Config.Kubectl(ctx, "get", "namespaces", "--selector", o.NamespaceSelector, "-o", "custom-columns=:metadata.name", "--no-headers")
	if err != nil {
		return nil, err
	}
	getCmd.Stderr = o.ErrOut
	out, err := getCmd.Output()
	if err != nil {
		return nil, err
	}

	// Scan the output, namespaces, one-per line
	var roleBindings []*rbacv1.RoleBinding
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		// Create namespaced binding for the generated cluster role
		if roleRef != nil {
			roleBindings = append(roleBindings, &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleRef.Name + "binding",
					Namespace: scanner.Text(),
				},
				Subjects: []rbacv1.Subject{*subject},
				RoleRef:  *roleRef,
			})
		}

		// Create namespaced binding for the manager cluster role
		if o.IncludeManagerRole {
			roleBindings = append(roleBindings, &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "optimize-manager-rolebinding",
					Namespace: scanner.Text(),
				},
				Subjects: []rbacv1.Subject{*subject},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "optimize-manager-role",
				},
			})
		}
	}
	return roleBindings, nil
}
