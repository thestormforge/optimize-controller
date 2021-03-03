/*
Copyright 2021 GramLabs, Inc.

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

package scan

import (
	"regexp"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Scheme is used globally to ensure ObjectSlice produces RNodes with type
// metadata (kind and apiVersion). In general, there isn't a convenient way to
// parameterize the Read call (i.e. no Context), thus the global variable.
// By default, the scheme is loaded with the known Kubernetes API types and
// the latest version of the types from this project.
var Scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = redskyv1beta1.AddToScheme(Scheme)
	_ = redskyappsv1alpha1.AddToScheme(Scheme)
}

// Selector is used to filter (reduce) a list of resource nodes and then
// map them into something that can be consumed by a transformer.
type Selector interface {
	Select([]*yaml.RNode) ([]*yaml.RNode, error)
	Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error)
}

// Transformer consumes the aggregated outputs from the selectors and
// turns them back into resource nodes. The original resource nodes used
// to generate the inputs are also made available.
type Transformer interface {
	Filter(nodes []*yaml.RNode, selected []interface{}) ([]*yaml.RNode, error)
}

type Scanner struct {
	Selectors   []Selector
	Transformer Transformer
}

var _ kio.Filter = &Scanner{}

func (s *Scanner) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	var selected []interface{}
	for _, sel := range s.Selectors {
		snodes, err := sel.Select(nodes)
		if err != nil {
			return nil, err
		}

		for _, node := range snodes {
			meta, err := node.GetMeta()
			if err != nil {
				return nil, err
			}

			mapped, err := sel.Map(node, meta)
			if err != nil {
				return nil, err
			}

			selected = append(selected, mapped...)
		}
	}

	return s.Transformer.Filter(nodes, selected)
}

type GenericSelector struct {
	Group              string `json:"group,omitempty"`
	Version            string `json:"version,omitempty"`
	Kind               string `json:"kind,omitempty"`
	Namespace          string `json:"namespace,omitempty"`
	Name               string `json:"name,omitempty"`
	LabelSelector      string `json:"labelSelector,omitempty"`
	AnnotationSelector string `json:"annotationSelector,omitempty"`
}

func (g *GenericSelector) Select(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	m, err := newMetaMatcher(g)
	if err != nil {
		return nil, err
	}

	result := make([]*yaml.RNode, 0, len(nodes))
	for _, n := range nodes {
		if meta, err := n.GetMeta(); err != nil {
			return nil, err
		} else if !m.matchesMeta(meta) {
			continue
		}

		if matched, err := n.MatchesLabelSelector(g.LabelSelector); err != nil {
			return nil, err
		} else if !matched {
			continue
		}

		if matched, err := n.MatchesAnnotationSelector(g.AnnotationSelector); err != nil {
			return nil, err
		} else if !matched {
			continue
		}

		result = append(result, n)
	}
	return result, nil
}

// metaMatcher matches metadata. This is an alternative to `types.SelectorRegex` from Kustomize.
type metaMatcher struct {
	namespaceRegex *regexp.Regexp
	nameRegex      *regexp.Regexp
	groupRegex     *regexp.Regexp
	versionRegex   *regexp.Regexp
	kindRegex      *regexp.Regexp
}

func newMetaMatcher(g *GenericSelector) (m *metaMatcher, err error) {
	m = &metaMatcher{}

	// Helper to make regular expressions match the full string
	compileAnchored := func(pattern string) (*regexp.Regexp, error) {
		if pattern == "" {
			return nil, nil
		}
		return regexp.Compile("^(?:" + pattern + ")$")
	}

	m.namespaceRegex, err = compileAnchored(g.Namespace)
	if err != nil {
		return nil, err
	}

	m.nameRegex, err = compileAnchored(g.Name)
	if err != nil {
		return nil, err
	}

	m.groupRegex, err = compileAnchored(g.Group)
	if err != nil {
		return nil, err
	}

	m.versionRegex, err = compileAnchored(g.Version)
	if err != nil {
		return nil, err
	}

	m.kindRegex, err = compileAnchored(g.Kind)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// matchesMeta returns true if the supplied metadata is accepted by this matcher.
func (m *metaMatcher) matchesMeta(meta yaml.ResourceMeta) bool {
	if m.namespaceRegex != nil && !m.namespaceRegex.MatchString(meta.Namespace) {
		return false
	}
	if m.nameRegex != nil && !m.nameRegex.MatchString(meta.Name) {
		return false
	}

	if m.groupRegex != nil || m.versionRegex != nil {
		group, version := "", meta.APIVersion
		if pos := strings.Index(version, "/"); pos >= 0 {
			group, version = version[0:pos], version[pos+1:]
		}

		if m.groupRegex != nil && !m.groupRegex.MatchString(group) {
			return false
		}

		if m.versionRegex != nil && !m.versionRegex.MatchString(version) {
			return false
		}
	}

	if m.kindRegex != nil && !m.kindRegex.MatchString(meta.Kind) {
		return false
	}

	return true
}

// ObjectSlices allows a slice of object instances to be read as resource nodes.
type ObjectSlice []runtime.Object

// Read converts the objects to JSON and then to YAML RNodes.
func (os ObjectSlice) Read() ([]*yaml.RNode, error) {
	var result []*yaml.RNode
	for _, o := range os {
		u := &unstructured.Unstructured{}
		if err := Scheme.Convert(o, u, runtime.InternalGroupVersioner); err != nil {
			return nil, err
		}

		data, err := json.Marshal(u)
		if err != nil {
			return nil, err
		}

		node, err := yaml.ConvertJSONToYamlNode(string(data))
		if err != nil {
			return nil, err
		}

		if err := node.PipeE(yaml.FilterFunc(clearCreationTimestamp)); err != nil {
			return nil, err
		}

		result = append(result, node)
	}

	return result, nil
}

// ObjectList allows a generic list type to be used as a KYAML writer to provide
// interoperability between the YAML streaming and runtime objects.
type ObjectList corev1.List

// Write converts the resource nodes into runtime objects.
func (o *ObjectList) Write(nodes []*yaml.RNode) error {
	for _, node := range nodes {
		data, err := node.MarshalJSON()
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(data); err != nil {
			return err
		}

		obj := newObjectForKind(u)
		if err := Scheme.Convert(u, obj, runtime.InternalGroupVersioner); err != nil {
			return err
		}

		o.Items = append(o.Items, runtime.RawExtension{Object: obj})
	}

	return nil
}

// clearCreationTimestamp is a dumb problem to have. Creation times are always
// serialized as JSON "null" (the Go JSON encoder does not have "omitempty" for
// generic structs); they should have been specified as a pointer to avoid this
// problem. But they weren't.
func clearCreationTimestamp(node *yaml.RNode) (*yaml.RNode, error) {
	var err error
	switch node.YNode().Kind {
	case yaml.MappingNode:
		if removed, err := node.Pipe(yaml.Clear("creationTimestamp")); err != nil || removed != nil {
			return node, err
		}

		err = node.VisitFields(func(node *yaml.MapNode) error {
			return node.Value.PipeE(yaml.FilterFunc(clearCreationTimestamp))
		})
	case yaml.SequenceNode:
		err = node.VisitElements(func(node *yaml.RNode) error {
			return node.PipeE(yaml.FilterFunc(clearCreationTimestamp))
		})
	}

	return node, err
}

// newObjectForKind uses the scheme to create a new typed object of the kind
// of the supplied object. If a new typed object cannot be created, the supplied
// object is returned.
func newObjectForKind(obj runtime.Object) runtime.Object {
	if gvks, _, _ := Scheme.ObjectKinds(obj); len(gvks) > 0 {
		if typed, err := Scheme.New(gvks[0]); err == nil {
			return typed
		}
	}

	return obj
}
