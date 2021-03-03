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

package scan

import (
	"regexp"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// GenericSelector can be used to implement the "Select" part of the Selector
// interface. The "*Selector" fields are treated as Kubernetes selectors, all
// other fields are regular expressions for matching metadata.
type GenericSelector struct {
	Group              string `json:"group,omitempty"`
	Version            string `json:"version,omitempty"`
	Kind               string `json:"kind,omitempty"`
	Namespace          string `json:"namespace,omitempty"`
	Name               string `json:"name,omitempty"`
	LabelSelector      string `json:"labelSelector,omitempty"`
	AnnotationSelector string `json:"annotationSelector,omitempty"`
}

// Select reduces the supplied resource node slice by only returning those
// nodes which match this selector.
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

// newMetaMatcher creates a new matcher by compiling all of the regexps.
func newMetaMatcher(g *GenericSelector) (m *metaMatcher, err error) {
	m = &metaMatcher{}

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

// compileAnchored is a helper to make regular expressions match a full string.
func compileAnchored(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile("^(?:" + pattern + ")$")
}
