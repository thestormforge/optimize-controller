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

package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSelectorApplyToList(t *testing.T) {
	cases := []struct {
		desc              string
		clientListOptions *client.ListOptions
		selector          *Selector
	}{
		{
			desc:              "nil",
			clientListOptions: &client.ListOptions{},
		},
		{
			desc:              "not nil",
			clientListOptions: &client.ListOptions{},
			selector: &Selector{
				Selector: labels.Everything(),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			c.selector.ApplyToList(c.clientListOptions)
			assert.Equal(t, c.selector, c.clientListOptions.LabelSelector)
		})
	}
}

func TestSelectorApplyToListOptions(t *testing.T) {
	cases := []struct {
		desc            string
		metaListOptions *metav1.ListOptions
		selector        *Selector
	}{
		{
			desc:            "not nil",
			metaListOptions: &metav1.ListOptions{},
			selector: &Selector{
				Selector: labels.Everything(),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			c.selector.ApplyToListOptions(c.metaListOptions)
			assert.Equal(t, c.selector.String(), c.metaListOptions.LabelSelector)
		})
	}
}

func TestMatchingSelector(t *testing.T) {
	cases := []struct {
		desc              string
		metaLabelSelector *metav1.LabelSelector
		selectorResult    *Selector
		errContains       string
	}{
		{
			desc: "nil label selector",
			selectorResult: &Selector{
				Selector: labels.Nothing(),
			},
		},
		{
			desc:              "everything label selector",
			metaLabelSelector: &metav1.LabelSelector{},
			selectorResult: &Selector{
				Selector: labels.Everything(),
			},
		},
		{
			desc: "invalid label selector",
			metaLabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Operator: "111"},
				},
			},
			errContains: "is not a valid pod selector operator",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			selector, err := MatchingSelector(c.metaLabelSelector)
			if c.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errContains)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.selectorResult, selector)
		})
	}
}
