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

package sfio

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// RenameField returns a filter that renames a field, merging the content if the
// "to" field already exists.
func RenameField(from, to string) FieldRenamer {
	return FieldRenamer{From: from, To: to}
}

// FieldRenamer is a filter for renaming fields.
type FieldRenamer struct {
	// The field to rename.
	From string
	// The target field name.
	To string
}

// Filter returns the node representing the (possibly merged) value of the
// renamed node or nil if the "from" field was not present.
func (f FieldRenamer) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	if err := yaml.ErrorIfInvalid(rn, yaml.MappingNode); err != nil {
		return nil, err
	}

	from, _ := yaml.Get(f.From).Filter(rn)
	if from == nil {
		return nil, nil
	}

	to, _ := yaml.Get(f.To).Filter(rn)
	if to != nil {
		to.YNode().Content = append(to.YNode().Content, from.YNode().Content...)
		_, _ = yaml.Clear(f.From).Filter(rn)
		return to, nil
	}

	for i := 0; i < len(rn.Content()); i = yaml.IncrementFieldIndex(i) {
		if rn.Content()[i].Value == f.From {
			rn.Content()[i].Value = f.To
			return yaml.Get(f.To).Filter(rn)
		}
	}

	return nil, nil
}

// PrefixClearer removes a prefix from a node value.
type PrefixClearer struct {
	Value string
}

// Filter removes the prefix from the node value or returns a nil node.
func (f *PrefixClearer) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	if strings.HasPrefix(rn.YNode().Value, f.Value) {
		rn.YNode().Value = strings.TrimPrefix(rn.YNode().Value, f.Value)
		return rn, nil
	}
	return nil, nil
}

// ClearFieldComment returns a filter which will clear matching line comments
// from a named field.
func ClearFieldComment(name string, lineComment string) CommentClearer {
	return CommentClearer{Name: name, Comments: yaml.Comments{LineComment: lineComment}}
}

// CommentClearer is filter for clearing specific comments.
type CommentClearer struct {
	// The name of the field to remove comments from.
	Name string
	// The collection of exact matching comments to clear.
	yaml.Comments
}

// Filter returns the supplied node with the appropriate field comments removed.
func (f CommentClearer) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	if err := yaml.ErrorIfInvalid(rn, yaml.MappingNode); err != nil {
		return nil, err
	}

	for i := 0; i < len(rn.Content()); i = yaml.IncrementFieldIndex(i) {
		c := rn.Content()[i]
		if c.Value == f.Name {
			if c.LineComment == f.Comments.LineComment {
				c.LineComment = ""
			}
			if c.HeadComment == f.Comments.HeadComment {
				c.HeadComment = ""
			}
			if c.FootComment == f.Comments.FootComment {
				c.FootComment = ""
			}
			return yaml.Get(f.Name).Filter(rn)
		}
	}

	return rn, nil
}

// Has works like `yaml.Tee` except that is also preserves a nil filter result.
func Has(filters ...yaml.Filter) HasFilter {
	return HasFilter{Filters: filters}
}

// HasFilter is an alternative to a "tee" filter in that it applies a list of
// filters. However, unlike "tee" filter, if the result of the filters is nil,
// the final result is also nil. This allows for constructing filter pipelines
// that with simplified conditional logic.
type HasFilter struct {
	Filters []yaml.Filter
}

// Filter returns the supplied node or nil if the the result of applying the
// configured filters is nil.
func (f HasFilter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	n, err := rn.Pipe(f.Filters...)
	if err != nil {
		return nil, err
	}
	if yaml.IsMissingOrNull(n) {
		return nil, nil
	}
	return rn, nil
}

// TeeMatched acts as a "tee" filter for nodes matched by the supplied path matcher:
// each matched node is processed by the supplied filters and the result of the
// entire operation is the initial node (or an error).
func TeeMatched(pathMatcher yaml.PathMatcher, filters ...yaml.Filter) TeeMatchedFilter {
	return TeeMatchedFilter{
		PathMatcher: pathMatcher,
		Filters:     filters,
	}
}

// TeeMatchedFilter is a filter that applies a set of filters to the nodes
// matched by a path matcher.
type TeeMatchedFilter struct {
	PathMatcher yaml.PathMatcher
	Filters     []yaml.Filter
}

// Filter always returns the supplied node, however all matching nodes will have
// been processed by the configured filters.
func (f TeeMatchedFilter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	matches, err := f.PathMatcher.Filter(rn)
	if err != nil {
		return nil, err
	}
	if err := matches.VisitElements(f.visitMatched); err != nil {
		return nil, err
	}
	return rn, nil
}

// visitMatched is used internally to preserve the field path and apply the
// configured filters.
func (f TeeMatchedFilter) visitMatched(node *yaml.RNode) error {
	matches := f.PathMatcher.Matches[node.YNode()]
	matchIndex := len(matches)
	for _, p := range f.PathMatcher.Path {
		if yaml.IsListIndex(p) && matchIndex > 0 {
			matchIndex--
			name, _, _ := yaml.SplitIndexNameValue(p)
			p = fmt.Sprintf("[%s=%s]", name, matches[matchIndex])
		}
		node.AppendToFieldPath(p)
	}

	return node.PipeE(f.Filters...)
}
