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

package generate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
)

// TODO Instead of it's own generator, have this be an option on the experiment generator

type RBACOptions struct {
	// Config is the Red Sky Configuration used to generate the role binding
	Config *config.RedSkyConfig
	// Printer is the resource printer used to render generated objects
	Printer commander.ResourcePrinter
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Filename           string
	Name               string
	IncludeNames       bool
	ClusterRole        bool
	ClusterRoleBinding bool

	mapper meta.RESTMapper
}

func NewRBACCommand(o *RBACOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rbac",
		Short: "Generate experiment roles",
		Long:  "Generate RBAC manifests from an experiment manifest",

		Annotations: map[string]string{
			commander.PrinterAllowedFormats: "json,yaml",
			commander.PrinterOutputFormat:   "yaml",
		},

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.Complete()
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "File that contains the experiment to extract roles from.")
	cmd.Flags().StringVar(&o.Name, "role-name", o.Name, "Name of the cluster role to generate (default is to use a generated name).")
	cmd.Flags().BoolVar(&o.IncludeNames, "include-names", o.IncludeNames, "Include resource names in the generated role.")
	cmd.Flags().BoolVar(&o.ClusterRole, "cluster-role", o.ClusterRole, "Generate a cluster role.")
	cmd.Flags().BoolVar(&o.ClusterRoleBinding, "cluster-role-binding", o.ClusterRoleBinding, "When generating a cluster role, also generate a cluster role binding.")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	commander.SetKubePrinter(&o.Printer, cmd)
	commander.ExitOnError(cmd)
	return cmd
}

func (o *RBACOptions) Complete() {
	// Create a REST mapper to convert from GroupVersionKind (used on patch targets) to GroupVersionResource (used in policy rules)
	rm := meta.NewDefaultRESTMapper(scheme.Scheme.PreferredVersionAllGroups())
	for gvk := range scheme.Scheme.AllKnownTypes() {
		rm.Add(gvk, meta.RESTScopeRoot)
	}
	o.mapper = rm
}

func (o *RBACOptions) generate() error {
	// Read the experiments
	// TODO For now just pretend like `readExperiment` could return multiple results
	experimentList := &redskyv1alpha1.ExperimentList{}
	experimentList.Items = make([]redskyv1alpha1.Experiment, 1)
	if err := readExperiment(o.Filename, o.In, &experimentList.Items[0]); err != nil {
		return err
	}

	// Determine the binding targets
	roleRef, subject, namespaces, err := o.bindingTargets(experimentList)
	if err != nil {
		return err
	}

	// Discover the policy rules from the experiments and collapse them
	var experimentRules []*rbacv1.PolicyRule
	for i := range experimentList.Items {
		experimentRules = o.appendRules(experimentRules, &experimentList.Items[i])
	}
	rules := make([]rbacv1.PolicyRule, 0, len(experimentRules))
	for _, r := range experimentRules {
		rules = mergeRule(rules, r)
	}
	if len(rules) == 0 {
		return nil
	}

	// Add up all the objects and print them out
	rbac := buildRBAC(roleRef, subject, rules, namespaces)
	return o.Printer.PrintObj(rbac, o.Out)
}

func (o *RBACOptions) bindingTargets(experimentList *redskyv1alpha1.ExperimentList) (*rbacv1.RoleRef, *rbacv1.Subject, []string, error) {
	// Create the role reference
	roleRef := &rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: o.Name}
	if o.ClusterRole {
		roleRef.Kind = "ClusterRole"
	}
	if roleRef.Name == "" {
		roleRef.Name = "redsky-patching-"
		if len(experimentList.Items) == 1 && experimentList.Items[0].Name != "" {
			roleRef.Name += experimentList.Items[0].Name
		} else if o.Filename == "-" {
			roleRef.Name += fmt.Sprintf("stdin-%s", rand.String(5))
		} else if o.Filename != "" {
			dir, suffix := filepath.Split(o.Filename)
			suffix = strings.TrimSuffix(suffix, filepath.Ext(suffix))
			if suffix == "experiment" {
				suffix = filepath.Base(dir)
			}
			re := regexp.MustCompile("[^a-z0-9]+")
			suffix = re.ReplaceAllString(strings.ToLower(suffix), "-")
			roleRef.Name += suffix
		} else {
			roleRef.Name += rand.String(5)
		}
	}

	// Create the subject using the namespace of the controller from the configuration
	ctrl, err := config.CurrentController(o.Config.Reader())
	if err != nil {
		return nil, nil, nil, err
	}
	subject := &rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "default",
		Namespace: ctrl.Namespace,
	}

	// Namespaces
	var namespaces []string
	if !o.ClusterRole || !o.ClusterRoleBinding {
		// Get the distinct list of namespaces from the experiments
		ns := make(map[string]bool, len(experimentList.Items))
		for i := range experimentList.Items {
			ns[experimentList.Items[i].Namespace] = true
		}

		// Try to provide an explicit default value (cstr.Namespace can still be "")
		if ns[""] {
			if cstr, err := config.CurrentCluster(o.Config.Reader()); err == nil {
				delete(ns, "")
				ns[cstr.Namespace] = true
			}
		}

		namespaces = make([]string, 0, len(ns))
		for k := range ns {
			namespaces = append(namespaces, k)
		}
	}

	// The manager runs as the default service account
	return roleRef, subject, namespaces, nil
}

func buildRBAC(roleRef *rbacv1.RoleRef, subject *rbacv1.Subject, rules []rbacv1.PolicyRule, namespaces []string) *corev1.List {
	result := &corev1.List{}
	switch roleRef.Kind {

	case "ClusterRole":
		// Include a single cluster role
		result.Items = append(result.Items, runtime.RawExtension{
			Object: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: roleRef.Name,
				},
				Rules: rules,
			},
		})

		// For each namespace, include a role binding
		for _, ns := range namespaces {
			result.Items = append(result.Items, runtime.RawExtension{
				Object: &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleRef.Name + "binding",
						Namespace: ns,
					},
					Subjects: []rbacv1.Subject{*subject},
					RoleRef:  *roleRef,
				},
			})
		}

		// If there are no namespaces, include a single cluster role binding
		if len(namespaces) == 0 {
			result.Items = append(result.Items, runtime.RawExtension{
				Object: &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: roleRef.Name + "binding",
					},
					Subjects: []rbacv1.Subject{*subject},
					RoleRef:  *roleRef,
				},
			})
		}

	case "Role":
		for _, ns := range namespaces {
			// Include a role and role binding for each namespace
			result.Items = append(result.Items,
				runtime.RawExtension{
					Object: &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleRef.Name,
							Namespace: ns,
						},
						Rules: rules,
					},
				},
				runtime.RawExtension{
					Object: &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleRef.Name + "binding",
							Namespace: ns,
						},
						Subjects: []rbacv1.Subject{*subject},
						RoleRef:  *roleRef,
					},
				})
		}
	}
	return result
}

// appendRules finds the patch and readiness targets from an experiment
func (o *RBACOptions) appendRules(rules []*rbacv1.PolicyRule, exp *redskyv1alpha1.Experiment) []*rbacv1.PolicyRule {
	// Patches require "get" and "patch" permissions
	for i := range exp.Spec.Patches {
		// TODO This needs to use patch_controller.go `renderTemplate` to get the correct reference (e.g. SMP may have the ref in the payload)
		// NOTE: Technically we can not get the target reference without an actual trial; in most cases a dummy trial should work
		ref := exp.Spec.Patches[i].TargetRef
		if ref != nil {
			rules = append(rules, o.newPolicyRule(ref, "get", "patch"))
		}
	}

	// Readiness checks with no name require "list" permissions
	for i := range exp.Spec.Template.Spec.ReadinessChecks {
		ref := &exp.Spec.Template.Spec.ReadinessChecks[i].TargetRef
		if ref.Name == "" {
			rules = append(rules, o.newPolicyRule(ref, "list"))
		}
	}

	// Readiness gates will be converted to readiness checks; therefore we need the same check on non-empty names
	for i := range exp.Spec.Template.Spec.ReadinessGates {
		r := &exp.Spec.Template.Spec.ReadinessGates[i]
		if r.Name == "" {
			ref := &corev1.ObjectReference{Kind: r.Kind, APIVersion: r.APIVersion}
			rules = append(rules, o.newPolicyRule(ref, "list"))
		}
	}

	return rules
}

// newPolicyRule creates a new policy rule for the specified object reference and list of verbs
func (o *RBACOptions) newPolicyRule(ref *corev1.ObjectReference, verbs ...string) *rbacv1.PolicyRule {
	// Start with the requested verbs
	r := &rbacv1.PolicyRule{
		Verbs: verbs,
	}

	// Get the mapping from GVK to GVR
	gvk := ref.GroupVersionKind()
	m, err := o.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		// TODO If this is guessing wrong too often we may need to allow additional mappings in the configuration
		m = &meta.RESTMapping{GroupVersionKind: gvk, Scope: meta.RESTScopeRoot}
		m.Resource, _ = meta.UnsafeGuessKindToResource(gvk)
	}
	r.APIGroups = []string{m.Resource.Group}
	r.Resources = []string{m.Resource.Resource}

	// Include the resource name if requested and available
	if o.IncludeNames && ref.Name != "" {
		r.ResourceNames = []string{ref.Name}
	}

	return r
}

// mergeRule attempts to combine the supplied rule with an existing compatible rule, failing that the rules are return with a new rule appended
func mergeRule(rules []rbacv1.PolicyRule, rule *rbacv1.PolicyRule) []rbacv1.PolicyRule {
	for i := range rules {
		r := &rules[i]
		if doesNotMatch(r.Verbs, rule.Verbs) {
			continue
		}
		if doesNotMatch(r.APIGroups, rule.APIGroups) {
			continue
		}
		if len(r.ResourceNames) > 0 && doesNotMatch(r.Resources, rule.Resources) {
			continue
		}

		for _, rr := range rule.Resources {
			r.Resources = appendMissing(r.Resources, rr)
		}
		sort.Strings(r.Resources)

		for _, rr := range rule.ResourceNames {
			r.ResourceNames = appendMissing(r.ResourceNames, rr)
		}
		sort.Strings(r.ResourceNames)

		return rules
	}
	return append(rules, *rule)
}

// doesNotMatch returns true if the two slices do not have the same ordered contents
func doesNotMatch(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return true
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return true
		}
	}
	return false
}

// appendMissing appends a string only if it does not already exist
func appendMissing(s []string, e string) []string {
	for _, i := range s {
		if i == e {
			return s
		}
	}
	return append(s, e)
}
