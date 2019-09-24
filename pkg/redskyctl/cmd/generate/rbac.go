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
	"fmt"
	"io/ioutil"
	"sort"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

// TODO Determine if this should be exposed as a Kustomize plugin also

const (
	generateRBACLong    = `Generate an experiment manifest from a configuration file`
	generateRBACExample = ``
)

type GenerateRBACOptions struct {
	Filename     string
	Name         string
	IncludeNames bool

	mapper meta.RESTMapper
	cmdutil.IOStreams
}

func NewGenerateRBACOptions(ioStreams cmdutil.IOStreams) *GenerateRBACOptions {
	return &GenerateRBACOptions{
		IOStreams: ioStreams,
	}
}

func NewGenerateRBACCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGenerateRBACOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "rbac",
		Short:   "Generate experiment roles",
		Long:    generateRBACLong,
		Example: generateRBACExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "File that contains the experiment to extract roles from.")
	_ = cmd.MarkFlagRequired("filename")
	cmd.Flags().StringVar(&o.Name, "role-name", "", "Name of the cluster role to generate (default is to use a generated name).")
	cmd.Flags().BoolVar(&o.IncludeNames, "include-names", false, "Include resource names in the generated role.")

	return cmd
}

func (o *GenerateRBACOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	// Create a REST mapper to convert from GroupVersionKind (used on patch targets) to GroupVersionResource (used in policy rules)
	rm := meta.NewDefaultRESTMapper(scheme.Scheme.PreferredVersionAllGroups())
	for gvk := range scheme.Scheme.AllKnownTypes() {
		rm.Add(gvk, meta.RESTScopeRoot)
	}
	o.mapper = rm

	return nil
}

func (o *GenerateRBACOptions) Run() error {
	// Read the experiment
	var data []byte
	var err error
	if o.Filename == "-" {
		data, err = ioutil.ReadAll(o.In)
	} else {
		data, err = ioutil.ReadFile(o.Filename)
	}
	if err != nil {
		return err
	}
	experiment := &redskyv1alpha1.Experiment{}
	if err = yaml.Unmarshal(data, experiment); err != nil {
		return err
	}
	if experiment.GroupVersionKind().GroupVersion() != redskyv1alpha1.GroupVersion || experiment.Kind != "Experiment" {
		return fmt.Errorf("expected experiment, got: %s", experiment.GroupVersionKind())
	}

	// Generate a cluster role
	clusterRole := &rbacv1.ClusterRole{}
	if o.Name != "" {
		clusterRole.Name = o.Name
	} else {
		clusterRole.GenerateName = "redsky-patching-"
	}
	clusterRole.Labels = map[string]string{"redskyops.dev/aggregate-to-patching": "true"}

	for i := range experiment.Spec.Patches {
		gvk, name, ok := extractTarget(&experiment.Spec.Patches[i])
		if !ok {
			continue
		}

		r := &rbacv1.PolicyRule{Verbs: []string{"get", "patch"}}

		if m, err := o.mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
			return err
		} else {
			r.APIGroups = appendMissing(r.APIGroups, m.Resource.Group)
			r.Resources = appendMissing(r.Resources, m.Resource.Resource)
		}

		if o.IncludeNames && name != "" {
			r.ResourceNames = appendMissing(r.ResourceNames, name)
		}

		clusterRole.Rules = mergeRule(clusterRole.Rules, r)
	}

	return serialize(clusterRole, o.Out)
}

// extractTarget attempts to get the patch target from the template
func extractTarget(p *redskyv1alpha1.PatchTemplate) (schema.GroupVersionKind, string, bool) {
	// TODO This should evaluate patch templates to ensure consistency; e.g. extract ref from the patch
	if p.TargetRef == nil {
		return schema.GroupVersionKind{}, "", false
	}
	return p.TargetRef.GroupVersionKind(), p.TargetRef.Name, true
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
