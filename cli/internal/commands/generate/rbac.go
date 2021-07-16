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
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
)

// TODO Instead of it's own generator, have this be an option on the experiment generator

type RBACOptions struct {
	// Config is the Optimize Configuration used to generate the role binding
	Config *config.OptimizeConfig
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
			o.Complete(cmd.Context())
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "`file` that contains the experiment to extract roles from")
	cmd.Flags().StringVar(&o.Name, "role-name", o.Name, "name of the cluster role to generate (default is to use a generated name)")
	cmd.Flags().BoolVar(&o.IncludeNames, "include-names", o.IncludeNames, "include resource names in the generated role")
	cmd.Flags().BoolVar(&o.ClusterRole, "cluster-role", o.ClusterRole, "generate a cluster role")
	cmd.Flags().BoolVar(&o.ClusterRoleBinding, "cluster-role-binding", o.ClusterRoleBinding, "when generating a cluster role, also generate a cluster role binding")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	commander.SetKubePrinter(&o.Printer, cmd, nil)

	return cmd
}

func (o *RBACOptions) Complete(ctx context.Context) {
	// Create a REST mapper to convert from GroupVersionKind (used on patch targets) to GroupVersionResource (used in policy rules)
	rm := meta.NewDefaultRESTMapper(scheme.Scheme.PreferredVersionAllGroups())
	for gvk := range scheme.Scheme.AllKnownTypes() {
		rm.Add(gvk, meta.RESTScopeRoot)
	}
	o.addInstalledCRDs(ctx, rm)
	o.mapper = rm
}

func (o *RBACOptions) addInstalledCRDs(ctx context.Context, rm *meta.DefaultRESTMapper) {
	cmd, err := o.Config.Kubectl(ctx, "get", "crds", "--output", "jsonpath", "--template",
		`{range .items[*].spec}{.group}/{.version} {.names.kind} {.names.plural} {.names.singular}{"\n"}{end}`)
	if err != nil {
		return
	}

	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())

		gv, err := schema.ParseGroupVersion(f[0])
		if err != nil {
			continue
		}

		rm.AddSpecific(gv.WithKind(f[1]), gv.WithResource(f[2]), gv.WithResource(f[3]), meta.RESTScopeRoot)
	}
}

func (o *RBACOptions) generate() error {
	r, err := o.IOStreams.OpenFile(o.Filename)
	if err != nil {
		return err
	}

	// Read the experiments
	experimentList := &optimizev1beta2.ExperimentList{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, experimentList); err != nil {
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

func (o *RBACOptions) bindingTargets(experimentList *optimizev1beta2.ExperimentList) (*rbacv1.RoleRef, *rbacv1.Subject, []string, error) {
	// Read the configuration objects we need
	r := o.Config.Reader()
	ctrl, err := config.CurrentController(r)
	if err != nil {
		return nil, nil, nil, err
	}
	cstr, err := config.CurrentCluster(r)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create the role reference
	roleRef := &rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: o.Name}
	if o.ClusterRole {
		roleRef.Kind = "ClusterRole"
	}
	if roleRef.Name == "" {
		roleRef.Name = roleName(o.Filename, experimentList)
	}

	// Create the subject using the namespace of the controller from the configuration
	subject := &rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "default",
		Namespace: ctrl.Namespace,
	}

	// Namespaces
	var namespaces []string
	if !o.ClusterRole || !o.ClusterRoleBinding {
		// Get the distinct list of namespaces from the experiments
		distinct := make(map[string]struct{}, len(experimentList.Items))
		for i := range experimentList.Items {
			ns := experimentList.Items[i].Namespace
			if ns == "" {
				ns = cstr.Namespace
			}
			distinct[ns] = struct{}{}
		}

		namespaces = make([]string, 0, len(distinct))
		for k := range distinct {
			namespaces = append(namespaces, k)
		}
	}

	// The manager runs as the default service account
	return roleRef, subject, namespaces, nil
}

func buildRBAC(roleRef *rbacv1.RoleRef, subject *rbacv1.Subject, rules []rbacv1.PolicyRule, namespaces []string) *corev1.List {
	result := &corev1.List{}

	// Include either a cluster role or a role for each namespace
	switch roleRef.Kind {
	case "ClusterRole":
		result.Items = append(result.Items, runtime.RawExtension{
			Object: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: roleRef.Name,
				},
				Rules: rules,
			},
		})
	case "Role":
		for _, ns := range namespaces {
			result.Items = append(result.Items, runtime.RawExtension{
				Object: &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleRef.Name,
						Namespace: ns,
					},
					Rules: rules,
				},
			})
		}
	}

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

	// If there are no namespaces for a cluster role, include a cluster role binding
	if roleRef.Kind == "ClusterRole" && len(namespaces) == 0 {
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

	return result
}

// appendRules finds the patch and readiness targets from an experiment
func (o *RBACOptions) appendRules(rules []*rbacv1.PolicyRule, exp *optimizev1beta2.Experiment) []*rbacv1.PolicyRule {
	// Patches require "get" and "patch" permissions
	for i := range exp.Spec.Patches {
		// TODO This needs to use patch_controller.go `renderTemplate` to get the correct reference (e.g. SMP may have the ref in the payload)
		// NOTE: Technically we can not get the target reference without an actual trial; in most cases a dummy trial should work
		ref := exp.Spec.Patches[i].TargetRef
		if ref != nil {
			rules = append(rules, o.newPolicyRule(ref, "get", "patch"))
		}
	}

	// Readiness gates with no name require "list" permissions
	for i := range exp.Spec.TrialTemplate.Spec.ReadinessGates {
		rg := &exp.Spec.TrialTemplate.Spec.ReadinessGates[i]
		if rg.Name == "" {
			ref := &corev1.ObjectReference{
				Kind:       rg.Kind,
				APIVersion: rg.APIVersion,
			}
			rules = append(rules, o.newPolicyRule(ref, "list"))
		}
	}

	// Readiness gates will be converted to readiness checks; therefore we need the same check on non-empty names
	for i := range exp.Spec.TrialTemplate.Spec.ReadinessGates {
		r := &exp.Spec.TrialTemplate.Spec.ReadinessGates[i]
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

// roleName attempts to generate a semi-unique role name
func roleName(filename string, experimentList *optimizev1beta2.ExperimentList) string {
	// If there is a single experiment, incorporate it's name into the role name
	if len(experimentList.Items) == 1 && experimentList.Items[0].Name != "" {
		return fmt.Sprintf("optimize-patching-%s-role", experimentList.Items[0].Name)
	}

	// If there are no inputs do not do anything special
	if len(experimentList.Items) == 0 || filename == "" {
		return "optimize-patching-role"
	}

	// Multiple experiments, tie the role name to the input file
	dir, unique := filepath.Split(filename)
	unique = strings.ToLower(strings.TrimSuffix(unique, filepath.Ext(unique)))
	if unique == "-" {
		// The experiments were read from stdin
		return fmt.Sprintf("optimize-patching-stdin-role-%s", rand.String(5))
	} else if dir == "/dev/fd/" {
		// The experiments were probably read via process substitution
		return fmt.Sprintf("optimize-patching-fd-role-%s", rand.String(5))
	}

	// Attempt to clean up the filename into something usable
	re := regexp.MustCompile("[^a-z0-9]+")
	unique = re.ReplaceAllString(unique, "-")
	return fmt.Sprintf("optimize-patching-%s-role", unique)
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
