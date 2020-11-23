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

package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestParameterNames(t *testing.T) {
	cases := []struct {
		desc     string
		ars      []*applicationResource
		expected []string
	}{
		{
			desc: "empty",
		},

		{
			desc: "one deployment one container",
			ars: []*applicationResource{
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test]", "resources"},
					},
				},
			},
			expected: []string{
				"cpu",
				"memory",
			},
		},

		{
			desc: "one deployment two containers",
			ars: []*applicationResource{
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test1]", "resources"},
						{"spec", "template", "spec", "containers", "[name=test2]", "resources"},
					},
				},
			},
			expected: []string{
				"test1_cpu",
				"test1_memory",
				"test2_cpu",
				"test2_memory",
			},
		},

		{
			desc: "two deployments one container",
			ars: []*applicationResource{
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test1",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test]", "resources"},
					},
				},
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test2",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test]", "resources"},
					},
				},
			},
			expected: []string{
				"test1_cpu",
				"test1_memory",
				"test2_cpu",
				"test2_memory",
			},
		},

		{
			desc: "two deployments two containers",
			ars: []*applicationResource{
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test1",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test1]", "resources"},
						{"spec", "template", "spec", "containers", "[name=test2]", "resources"},
					},
				},
				{
					targetRef: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "test2",
					},
					containerResourcesPaths: [][]string{
						{"spec", "template", "spec", "containers", "[name=test]", "resources"},
					},
				},
			},
			expected: []string{
				"test1_test1_cpu",
				"test1_test1_memory",
				"test1_test2_cpu",
				"test1_test2_memory",
				"test2_cpu",
				"test2_memory",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			p := parameterNamePrefix(c.ars)
			var actual []string
			for _, cr := range c.ars {
				pn := parameterName(p, cr)
				for i := range cr.containerResourcesPaths {
					actual = append(actual, pn(&cr.targetRef, cr.containerResourcesPaths[i], "cpu"))
					actual = append(actual, pn(&cr.targetRef, cr.containerResourcesPaths[i], "memory"))
				}
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}
