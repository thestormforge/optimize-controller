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

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type ApplicationFilter struct {
	Application *redskyappsv1alpha1.Application
}

var _ kio.Filter = &ApplicationFilter{}

func (f *ApplicationFilter) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	var fs []yaml.Filter

	// TODO The first filter should make sure we only apply things to resources we generated

	for i := range f.Application.Objectives {
		if !f.Application.Objectives[i].Implemented {
			return nil, fmt.Errorf("generated experiment cannot optimize objective: %s", f.Application.Objectives[i].Name)
		}
	}

	if f.Application.Name != "" {
		setLabel := yaml.SetLabel(redskyappsv1alpha1.LabelApplication, f.Application.Name)
		fs = append(fs, yaml.Tee(
			yaml.Tee(setLabel),
			isExperiment(),
			yaml.Lookup("spec", "trialTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "jobTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "template"), yaml.Tee(setLabel),
		))
	}

	if f.Application.Namespace != "" {
		fs = append(fs, yaml.Tee(
			isNamespaceScoped(),
			yaml.SetK8sNamespace(f.Application.Namespace),
		))

		fs = append(fs, yaml.Tee(
			isClusterRoleOrBinding(),
			yaml.Get("subjects"),
			yaml.GetElementByKey("name"),
			&yaml.FieldMatcher{Name: "namespace", Create: yaml.NewScalarRNode(f.Application.Namespace)},
		))
	}

	if len(f.Application.Scenarios) == 1 {
		setLabel := yaml.SetLabel(redskyappsv1alpha1.LabelScenario, f.Application.Scenarios[0].Name)
		fs = append(fs, yaml.Tee(
			isExperiment(),
			yaml.Tee(setLabel),
			yaml.Lookup("spec", "trialTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "jobTemplate"), yaml.Tee(setLabel),
			yaml.Lookup("spec", "template"), yaml.Tee(setLabel),
		))
	}

	// Get the experiment name and add it as a suffix
	experimentName := getExperimentName(nodes)
	if experimentName != "" {
		suffix := &yaml.SuffixSetter{Value: fmt.Sprintf("-%x", sha256.Sum256([]byte(experimentName)))[0:7]}
		fs = append(fs, yaml.Tee(
			isClusterRoleOrBinding(),
			yaml.Tee(yaml.Lookup(yaml.MetadataField, yaml.NameField), suffix),
			yaml.Tee(yaml.Lookup("roleRef", yaml.NameField), suffix),
		))
	}

	for i := range nodes {
		if err := nodes[i].PipeE(fs...); err != nil {
			return nil, err
		}
	}

	return nodes, nil
}

func getExperimentName(nodes []*yaml.RNode) string {
	var experimentName string
	_, _ = kio.FilterAll(yaml.Tee(
		isExperiment(), yaml.Lookup("metadata", "name"), yaml.FilterFunc(func(object *yaml.RNode) (*yaml.RNode, error) {
			experimentName = object.YNode().Value
			return nil, fmt.Errorf("ignore this error, just stop iterating")
		}))).Filter(nodes)
	return experimentName
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
