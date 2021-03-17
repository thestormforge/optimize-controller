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

package generation

import (
	"crypto/sha256"
	"fmt"

	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// SetExperimentLabel is a filter that sets a label on an experiment object.
func SetExperimentLabel(key, value string) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		if value == "" {
			return node, nil
		}

		setLabel := yaml.SetLabel(key, value)
		return node.Pipe(yaml.Tee(
			isExperiment(),
			yaml.Tee(setLabel),
			yaml.Lookup("spec", "trialTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "jobTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "template"), yaml.Tee(setLabel),
		))
	})
}

// SetNamespace sets the namespace on a resource (if necessary).
func SetNamespace(namespace string) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		if namespace == "" {
			return node, nil
		}

		return node.Pipe(yaml.Tee(
			yaml.Tee(
				isNamespaceScoped(),
				yaml.SetK8sNamespace(namespace),
			),
			yaml.Tee(
				isClusterRoleOrBinding(),
				yaml.Get("subjects"),
				yaml.GetElementByKey("name"),
				&yaml.FieldMatcher{Name: "namespace", Create: yaml.NewScalarRNode(namespace)},
			),
		))
	})
}

// SetExperimentName sets the name on the experiment. In addition, the experiment name is set as a
// suffix on any generated cluster roles or cluster role bindings.
func SetExperimentName(name string) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		suffix := &yaml.SuffixSetter{Value: fmt.Sprintf("-%x", sha256.Sum256([]byte(name)))[0:7]}
		return node.Pipe(
			yaml.Tee(
				isExperiment(),
				yaml.SetK8sName(name),
			),
			yaml.Tee(
				isClusterRoleOrBinding(),
				yaml.Tee(yaml.Lookup(yaml.MetadataField, yaml.NameField), suffix),
				yaml.Tee(yaml.Lookup("roleRef", yaml.NameField), suffix),
			),
		)
	})
}

func isExperiment() yaml.Filter {
	return &kindFilter{apiVersion: []string{redskyv1beta1.GroupVersion.String()}, kind: []string{"Experiment"}}
}

func isClusterRoleOrBinding() yaml.Filter {
	return &kindFilter{apiVersion: []string{rbacv1.SchemeGroupVersion.String()}, kind: []string{"ClusterRole", "ClusterRoleBinding"}}
}

func isNamespaceScoped() yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		meta, err := node.GetMeta()
		if err != nil {
			return nil, err
		}
		if ns, ok := openapi.IsNamespaceScoped(meta.TypeMeta); !ns && ok {
			return nil, nil
		}
		return node, nil
	})
}

// TODO How does something like this not exist in KYAML?
type kindFilter struct {
	kind       []string
	apiVersion []string
}

func (f *kindFilter) Filter(node *yaml.RNode) (*yaml.RNode, error) {
	kindNode, err := node.Pipe(yaml.Get(yaml.KindField))
	if err != nil || kindNode == nil || !f.matchKind(kindNode.YNode().Value) {
		return nil, err
	}

	apiVersion, err := node.Pipe(yaml.Get(yaml.APIVersionField))
	if err != nil || apiVersion == nil || !f.matchAPIVersion(apiVersion.YNode().Value) {
		return nil, err
	}

	return node, nil
}

func (f *kindFilter) matchKind(kind string) bool {
	for _, k := range f.kind {
		if k == kind {
			return true
		}
	}
	return false
}

func (f *kindFilter) matchAPIVersion(apiVersion string) bool {
	for _, v := range f.apiVersion {
		if v == apiVersion {
			return true
		}
	}
	return false
}
