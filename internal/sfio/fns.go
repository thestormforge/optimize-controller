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

func RenameField(from, to string) FieldRenamer {
	return FieldRenamer{From: from, To: to}
}

type FieldRenamer struct {
	From string
	To   string
}

func (f FieldRenamer) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	if err := yaml.ErrorIfInvalid(rn, yaml.MappingNode); err != nil {
		return nil, err
	}

	// TODO This doesn't account for when the "To" field already exists...

	for i := 0; i < len(rn.Content()); i = yaml.IncrementFieldIndex(i) {
		if rn.Content()[i].Value == f.From {
			rn.Content()[i].Value = f.To
			return yaml.Get(f.To).Filter(rn)
		}
	}

	return nil, nil
}

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

type CommentClearer struct {
	Name string
	yaml.Comments
}

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

type HasFilter struct {
	Filters []yaml.Filter
}

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
