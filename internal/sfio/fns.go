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

// ClearFieldComment returns a filter which will clear matching line comments
// from a named field.
func ClearFieldComment(name string, comments ...string) CommentClearer {
	cc := CommentClearer{Name: name}
	if len(comments) > 0 {
		cc.Comments.LineComment = comments[0]
	}
	if len(comments) > 1 {
		cc.Comments.HeadComment = comments[1]
	}
	if len(comments) > 2 {
		cc.Comments.FootComment = comments[2]
	}
	return cc
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
